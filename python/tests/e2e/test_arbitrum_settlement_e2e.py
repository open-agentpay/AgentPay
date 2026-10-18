"""
End-to-End Test for Arbitrum Sepolia Testnet Settlement (T6.2).

Validates the full autonomous payment and clearing lifecycle:
1. Buyer Agent (AgentPayClient + PolicyEngine + LocalSigner)
2. Gateway Reverse Proxy (GatewayProxyService + RouteMatcher + PaymentVerifier)
3. On-chain Facilitator Node (EvmFacilitatorService + EvmRpcClient)
4. Asynchronous EVM transaction broadcast and receipt confirmation
"""

from __future__ import annotations

import json
from typing import Any

import httpx
from agentpay.client.client import AgentPayClient
from agentpay.client.signer import LocalSigner
from agentpay.core.state import FulfillmentState, SettlementState
from agentpay.evm.constants import (
    ARBITRUM_SEPOLIA_CAIP2,
    ARBITRUM_SEPOLIA_CHAIN_ID,
    ARBITRUM_SEPOLIA_RPC_URL,
    ARBITRUM_SEPOLIA_USDC_ADDRESS,
)
from agentpay.evm.rpc import EvmRpcClient
from agentpay.policy.config import PolicyConfig
from agentpay.policy.db import create_buyer_connection, init_buyer_database
from agentpay.policy.sqlite_engine import SQLitePolicyEngine
from agentpay_gateway.config.models import (
    FacilitatorConfig,
    RouteConfig,
    SettlementDefaults,
)
from agentpay_gateway.config.router import RouteMatcher
from agentpay_gateway.facilitator.client import FacilitatorClient
from agentpay_gateway.facilitator.evm_service import EvmFacilitatorService
from agentpay_gateway.payment.challenge import ChallengeManager
from agentpay_gateway.payment.settlement import AsyncSettlementWorker
from agentpay_gateway.payment.verifier import PaymentVerifier
from agentpay_gateway.proxy.forwarder import ProxyForwarder
from agentpay_gateway.proxy.service import GatewayProxyService, GatewayTransport
from agentpay_gateway.state.coordinator import GatewayStateCoordinator
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database

BUYER_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

FACILITATOR_KEY = "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
MERCHANT_PAYEE = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"


class MockArbitrumRpcHandler:
    """Simulates an Arbitrum Sepolia JSON-RPC node."""

    def __init__(self) -> None:
        self.broadcasted_raw_txs: list[str] = []
        self.receipts: dict[str, dict[str, Any]] = {}

    def handle(self, request: httpx.Request) -> httpx.Response:
        data = json.loads(request.content.decode("utf-8"))
        method = data.get("method")
        params = data.get("params", [])
        req_id = data.get("id", 1)

        result: Any = None
        if method == "eth_chainId":
            result = hex(ARBITRUM_SEPOLIA_CHAIN_ID)
        elif method == "eth_blockNumber":
            result = "0xbc614e"  # block 12345678
        elif method == "eth_gasPrice":
            result = "0x5f5e100"  # 0.1 gwei
        elif method == "eth_getTransactionCount":
            result = hex(len(self.broadcasted_raw_txs))
        elif method == "eth_estimateGas":
            result = "0x11170"  # 70,000 gas
        elif method == "eth_call":
            # authorizationState returns 0 (unused)
            result = "0x" + "00" * 32
        elif method == "eth_sendRawTransaction":
            raw_tx = params[0]
            self.broadcasted_raw_txs.append(raw_tx)
            tx_hash = f"0x{len(self.broadcasted_raw_txs):064x}"
            self.receipts[tx_hash] = {
                "transactionHash": tx_hash,
                "blockNumber": "0xbc614f",
                "status": "0x1",
                "gasUsed": "0xfde8",
            }
            result = tx_hash
        elif method == "eth_getTransactionReceipt":
            tx_hash = params[0]
            result = self.receipts.get(tx_hash)
        else:
            result = "0x0"

        return httpx.Response(
            status_code=200,
            json={"jsonrpc": "2.0", "result": result, "id": req_id},
            request=request,
        )


class TestArbitrumSepoliaE2ESettlement:
    def test_complete_payment_and_onchain_clearing_flow(self, tmp_path) -> None:
        # 1. Setup Mock EVM RPC for Arbitrum Sepolia
        rpc_handler = MockArbitrumRpcHandler()
        rpc_client = EvmRpcClient(
            rpc_url=ARBITRUM_SEPOLIA_RPC_URL,
            expected_chain_id=ARBITRUM_SEPOLIA_CHAIN_ID,
            client=httpx.Client(transport=httpx.MockTransport(rpc_handler.handle)),
        )

        # 2. Setup EvmFacilitatorService
        facilitator_service = EvmFacilitatorService(
            rpc_url=ARBITRUM_SEPOLIA_RPC_URL,
            chain_id=ARBITRUM_SEPOLIA_CHAIN_ID,
            token_address=ARBITRUM_SEPOLIA_USDC_ADDRESS,
            private_key=FACILITATOR_KEY,
            rpc_client=rpc_client,
            auto_poll_receipts=False,
        )
        facilitator_client = FacilitatorClient(
            base_url="http://facilitator:8001",
            transport=facilitator_service.create_sync_transport(),
        )

        # 3. Setup Gateway Infrastructure
        gateway_db = str(tmp_path / "gateway_test.db")
        db_conn = create_gateway_connection(gateway_db)
        init_gateway_database(db_conn)
        state_coordinator = GatewayStateCoordinator(conn=db_conn)

        challenge_mgr = ChallengeManager(
            facilitator_config=FacilitatorConfig(url="http://facilitator:8001"),
            settlement_defaults=SettlementDefaults(
                default_network=ARBITRUM_SEPOLIA_CAIP2,
                default_asset=ARBITRUM_SEPOLIA_USDC_ADDRESS,
                default_pay_to=MERCHANT_PAYEE,
                default_asset_symbol="USDC",
                default_asset_decimals=6,
            ),
        )

        settlement_worker = AsyncSettlementWorker(
            facilitator_client=facilitator_client,
            state_coordinator=state_coordinator,
        )

        payment_verifier = PaymentVerifier(
            challenge_manager=challenge_mgr,
            state_coordinator=state_coordinator,
            facilitator_client=facilitator_client,
            settlement_worker=settlement_worker,
        )

        routes = [
            RouteConfig(
                path="/v1/market-data",
                upstream="http://mock-upstream:9000/v1/market-data",
                methods=["GET"],
                price="0.005 USDC",  # 5,000 base units
                network=ARBITRUM_SEPOLIA_CAIP2,
                asset=ARBITRUM_SEPOLIA_USDC_ADDRESS,
                pay_to=MERCHANT_PAYEE,
            )
        ]

        route_matcher = RouteMatcher(routes)

        # Mock upstream business API returning market data
        def upstream_handler(req: httpx.Request) -> httpx.Response:
            return httpx.Response(200, json={"symbol": "ETH/USDC", "price": 3200.50}, request=req)

        mock_upstream_client = httpx.Client(transport=httpx.MockTransport(upstream_handler))
        forwarder = ProxyForwarder(client=mock_upstream_client)

        gateway_service = GatewayProxyService(
            route_matcher=route_matcher,
            payment_verifier=payment_verifier,
            forwarder=forwarder,
            state_coordinator=state_coordinator,
        )

        gateway_transport = GatewayTransport(gateway_service)

        # 4. Setup Buyer Agent (AgentPayClient + Financial Firewall)
        buyer_db = str(tmp_path / "buyer_policy.db")
        buyer_conn = create_buyer_connection(buyer_db)
        init_buyer_database(buyer_conn)
        buyer_signer = LocalSigner(BUYER_KEY)
        policy_config = PolicyConfig(
            id="pol_test_01",
            name="Test Arbitrum Policy",
            wallet_address=buyer_signer.address,
            max_per_tx_base_units=100_000,  # 0.10 USDC cap
            rolling_budget_24h_base_units=1_000_000,  # 1.00 USDC daily
            whitelist_payees=[MERCHANT_PAYEE],
            whitelist_networks=[ARBITRUM_SEPOLIA_CAIP2],
            whitelist_assets=[ARBITRUM_SEPOLIA_USDC_ADDRESS],
            whitelist_domains=["gateway"],
        )

        policy_engine = SQLitePolicyEngine(
            conn=buyer_conn,
            config=policy_config,
        )

        buyer_client = AgentPayClient(
            signer=buyer_signer,
            policy=policy_engine,
            transport=gateway_transport,
            base_url="http://gateway:8000",
        )

        # 5. Execute Autonomous Paid HTTP Request
        response = buyer_client.get("http://gateway:8000/v1/market-data")

        # 6. Validate HTTP 200 OK & Resource Delivery
        assert response.status_code == 200

        data = response.json()
        assert data["symbol"] == "ETH/USDC"
        assert data["price"] == 3200.50
        assert "x-request-id" in response.headers

        # 7. Drain Gateway Settlement Worker Queue
        settlement_worker.drain(timeout=3.0)
        settlement_worker.stop()

        # 8. Verify Blockchain Settlement
        assert len(rpc_handler.broadcasted_raw_txs) == 1
        raw_tx_hex = rpc_handler.broadcasted_raw_txs[0]
        assert raw_tx_hex.startswith("0x")

        # Confirm on-chain status
        assert len(facilitator_service._settlements) == 1
        stl_id = next(iter(facilitator_service._settlements.keys()))
        confirmed_stl = facilitator_service.sync_settlement_receipt(stl_id)
        assert confirmed_stl.status == SettlementState.CONFIRMED
        assert confirmed_stl.error is None
        assert confirmed_stl.tx_hash == "0x" + "0" * 63 + "1"

        # 9. Verify Gateway State Machine Persistence
        records = state_coordinator.list_settlements()
        assert len(records) == 1
        rec = records[0]
        assert rec.payer_address.lower() == buyer_signer.address.lower()
        assert rec.payee_address.lower() == MERCHANT_PAYEE.lower()
        assert rec.amount_base_units == 5000

        fulfillment = state_coordinator.get_fulfillment_by_settlement(rec.settlement_id)
        assert fulfillment is not None
        assert fulfillment.status == FulfillmentState.DELIVERED
