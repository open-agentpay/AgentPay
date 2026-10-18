"""
Unit & Integration tests for Arbitrum Sepolia Testnet & EIP-3009 Contract Integration (T6.1).
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
from agentpay.client.signer import LocalSigner, generate_nonce
from agentpay.core.models import PaymentChallenge
from agentpay.evm.constants import (
    ARBITRUM_SEPOLIA_CAIP2,
    ARBITRUM_SEPOLIA_CHAIN_ID,
    ARBITRUM_SEPOLIA_EXPLORER_URL,
    ARBITRUM_SEPOLIA_RPC_URL,
    ARBITRUM_SEPOLIA_USDC_ADDRESS,
    ARBITRUM_SEPOLIA_USDC_DECIMALS,
    ARBITRUM_SEPOLIA_USDC_NAME,
    ARBITRUM_SEPOLIA_USDC_SYMBOL,
    ARBITRUM_SEPOLIA_USDC_VERSION,
    AUTHORIZATION_STATE_SELECTOR,
    BALANCE_OF_SELECTOR,
    CANCEL_AUTHORIZATION_SELECTOR,
    RECEIVE_WITH_AUTHORIZATION_SELECTOR,
    TRANSFER_WITH_AUTHORIZATION_SELECTOR,
    get_network_spec,
)
from agentpay.evm.eip3009 import (
    decode_authorization_state_result,
    decode_balance_of_result,
    decode_transfer_with_authorization,
    encode_authorization_state,
    encode_balance_of,
    encode_transfer_from_authorization,
    encode_transfer_with_authorization,
)
from agentpay.evm.rpc import (
    EvmRpcClient,
    EvmRpcConnectionError,
    EvmRpcError,
    EvmRpcRequestError,
)
from agentpay.policy.models import ReservationToken
from agentpay_gateway.config.loader import load_gateway_config
from agentpay_gateway.config.models import FacilitatorConfig, SettlementDefaults
from agentpay_gateway.payment.challenge import ChallengeManager
from eth_utils import is_checksum_address, to_checksum_address

BUYER_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
MERCHANT_PAYEE = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"


# ==============================================================================
# 1. Constants & Specification Verification
# ==============================================================================


class TestArbitrumSepoliaConstants:
    """Verifies Arbitrum Sepolia parameters, Circle USDC addresses, and selectors."""

    def test_network_parameters(self) -> None:
        assert ARBITRUM_SEPOLIA_CHAIN_ID == 421614
        assert ARBITRUM_SEPOLIA_CAIP2 == "eip155:421614"
        assert "arbitrum.io" in ARBITRUM_SEPOLIA_RPC_URL
        assert "arbiscan.io" in ARBITRUM_SEPOLIA_EXPLORER_URL

    def test_circle_usdc_contract(self) -> None:
        checksum = to_checksum_address(ARBITRUM_SEPOLIA_USDC_ADDRESS)
        assert is_checksum_address(checksum)
        assert checksum == "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d"
        assert ARBITRUM_SEPOLIA_USDC_SYMBOL == "USDC"
        assert ARBITRUM_SEPOLIA_USDC_DECIMALS == 6
        assert ARBITRUM_SEPOLIA_USDC_NAME == "USD Coin"
        assert ARBITRUM_SEPOLIA_USDC_VERSION == "2"

    def test_eip3009_function_selectors(self) -> None:
        # Expected 4-byte selectors from keccak256
        assert TRANSFER_WITH_AUTHORIZATION_SELECTOR == bytes.fromhex("e3ee160e")
        assert RECEIVE_WITH_AUTHORIZATION_SELECTOR == bytes.fromhex("ef55dec6")
        assert CANCEL_AUTHORIZATION_SELECTOR == bytes.fromhex("5b34b966")
        assert AUTHORIZATION_STATE_SELECTOR == bytes.fromhex("e94a0102")
        assert BALANCE_OF_SELECTOR == bytes.fromhex("70a08231")

    def test_network_registry_lookups(self) -> None:
        spec_by_caip2 = get_network_spec("eip155:421614")
        assert spec_by_caip2 is not None
        assert spec_by_caip2.chain_id == 421614
        assert spec_by_caip2.default_asset_symbol == "USDC"

        spec_by_chain_id = get_network_spec(421614)
        assert spec_by_chain_id is not None
        assert spec_by_chain_id.name == "arbitrum-sepolia"

        spec_by_alias = get_network_spec("arb-sepolia")
        assert spec_by_alias is not None
        assert spec_by_alias.chain_id == 421614

        assert get_network_spec("unknown-chain-999") is None


# ==============================================================================
# 2. EIP-3009 ABI Encoding & Decoding Tests
# ==============================================================================


class TestEip3009Calldata:
    """Tests calldata encoding and decoding for EIP-3009 smart contract interactions."""

    def test_encode_and_decode_transfer_with_authorization_roundtrip(self) -> None:
        from_addr = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
        to_addr = MERCHANT_PAYEE
        value = 250_000  # 0.25 USDC
        valid_after = 1729000000
        valid_before = 1729003600
        nonce = "0x" + "aa" * 32
        v = 28
        r = "0x" + "bb" * 32
        s = "0x" + "cc" * 32

        calldata = encode_transfer_with_authorization(
            from_address=from_addr,
            to_address=to_addr,
            value=value,
            valid_after=valid_after,
            valid_before=valid_before,
            nonce=nonce,
            v=v,
            r=r,
            s=s,
        )

        # 4 bytes selector + 9 * 32 bytes arguments = 292 bytes
        assert len(calldata) == 292
        assert calldata[:4] == TRANSFER_WITH_AUTHORIZATION_SELECTOR

        # Decode and verify
        decoded = decode_transfer_with_authorization(calldata)
        assert decoded["from"].lower() == from_addr.lower()
        assert decoded["to"].lower() == to_addr.lower()
        assert decoded["value"] == value
        assert decoded["validAfter"] == valid_after
        assert decoded["validBefore"] == valid_before
        assert decoded["nonce"].lower() == nonce.lower()
        assert decoded["v"] == v
        assert decoded["r"].lower() == r.lower()
        assert decoded["s"].lower() == s.lower()

    def test_encode_transfer_from_authorization_model(self) -> None:
        signer = LocalSigner(BUYER_KEY)
        challenge = PaymentChallenge(
            version="2.0",
            challenge_id="ch_arb_1",
            network=ARBITRUM_SEPOLIA_CAIP2,
            asset=ARBITRUM_SEPOLIA_USDC_ADDRESS,
            asset_symbol="USDC",
            decimals=6,
            amount="100000",
            payee=MERCHANT_PAYEE,
            scheme="EIP-3009",
            valid_after=1729000000,
            valid_before=1729003600,
            facilitator_url="http://mock",
        )
        token = ReservationToken(
            reservation_id="res_arb_1",
            intent_id=challenge.challenge_id,
            policy_id="pol_default",
            amount=int(challenge.amount),
            expires_at=challenge.valid_before,
            created_at=challenge.valid_after,
        )
        auth = signer.sign_payment_authorization(
            challenge,
            token,
            nonce=generate_nonce(),
            token_name=ARBITRUM_SEPOLIA_USDC_NAME,
            token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
        )

        calldata = encode_transfer_from_authorization(
            authorization=auth,
            to_address=challenge.payee,
            value=int(challenge.amount),
        )

        assert len(calldata) == 292
        assert calldata[:4] == TRANSFER_WITH_AUTHORIZATION_SELECTOR

        decoded = decode_transfer_with_authorization(calldata)
        assert decoded["from"].lower() == signer.address.lower()
        assert decoded["to"].lower() == MERCHANT_PAYEE.lower()
        assert decoded["value"] == 100000
        assert decoded["nonce"].lower() == auth.nonce.lower()

    def test_authorization_state_encoding_and_decoding(self) -> None:
        authorizer = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
        nonce = "0x" + "11" * 32

        # 4 bytes selector + 64 bytes arguments = 68 bytes
        calldata = encode_authorization_state(authorizer, nonce)
        assert len(calldata) == 68
        assert calldata[:4] == AUTHORIZATION_STATE_SELECTOR

        # Decode true / false simulation results from eth_call
        true_result = "0x" + "00" * 31 + "01"
        false_result = "0x" + "00" * 32
        assert decode_authorization_state_result(true_result) is True
        assert decode_authorization_state_result(false_result) is False

    def test_balance_of_encoding_and_decoding(self) -> None:
        account = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
        calldata = encode_balance_of(account)
        assert len(calldata) == 36
        assert calldata[:4] == BALANCE_OF_SELECTOR

        mock_balance_hex = "0x" + hex(150_000_000)[2:].zfill(64)
        balance = decode_balance_of_result(mock_balance_hex)
        assert balance == 150_000_000


# ==============================================================================
# 3. EVM JSON-RPC Client Tests
# ==============================================================================


class TestEvmRpcClient:
    """Tests the lightweight JSON-RPC client against mock node transports."""

    @pytest.fixture
    def mock_rpc_transport(self) -> httpx.MockTransport:
        def handler(request: httpx.Request) -> httpx.Response:
            payload = json.loads(request.read())
            method = payload["method"]
            req_id = payload["id"]

            if method == "eth_chainId":
                # 421614 in hex = 0x66eee
                return httpx.Response(200, json={"jsonrpc": "2.0", "result": "0x66eee", "id": req_id})

            if method == "eth_blockNumber":
                return httpx.Response(200, json={"jsonrpc": "2.0", "result": "0x12a05f", "id": req_id})

            if method == "eth_call":
                tx = payload["params"][0]
                data = tx["data"]
                if data.startswith("0xe94a0102"):
                    # authorizationState: simulate nonce is unused (0)
                    return httpx.Response(200, json={"jsonrpc": "2.0", "result": "0x" + "00" * 32, "id": req_id})
                if data.startswith("0x70a08231"):
                    # balanceOf: return 500.00 USDC (500,000,000)
                    bal_hex = hex(500_000_000)[2:].zfill(64)
                    return httpx.Response(200, json={"jsonrpc": "2.0", "result": "0x" + bal_hex, "id": req_id})
                return httpx.Response(200, json={"jsonrpc": "2.0", "result": "0x", "id": req_id})

            if method == "eth_gasPrice":
                return httpx.Response(200, json={"jsonrpc": "2.0", "result": "0x5f5e100", "id": req_id})

            if method == "eth_getBlockByNumber":
                return httpx.Response(
                    200,
                    json={
                        "jsonrpc": "2.0",
                        "result": {
                            "number": "0x12a05f",
                            "baseFeePerGas": "0x5f5e100",
                        },
                        "id": req_id,
                    },
                )

            if method == "eth_maxPriorityFeePerGas":
                return httpx.Response(200, json={"jsonrpc": "2.0", "result": "0xf4240", "id": req_id})

            if method == "eth_estimateGas":
                return httpx.Response(200, json={"jsonrpc": "2.0", "result": "0x15f90", "id": req_id})

            if method == "eth_sendRawTransaction":
                mock_tx_hash = "0x8888888888888888888888888888888888888888888888888888888888888888"
                return httpx.Response(200, json={"jsonrpc": "2.0", "result": mock_tx_hash, "id": req_id})

            if method == "eth_getTransactionReceipt":
                tx_hash = payload["params"][0]
                return httpx.Response(
                    200,
                    json={
                        "jsonrpc": "2.0",
                        "result": {
                            "transactionHash": tx_hash,
                            "blockNumber": "0x12a060",
                            "status": "0x1",
                            "gasUsed": "0x12c00",
                        },
                        "id": req_id,
                    },
                )

            return httpx.Response(
                200,
                json={
                    "jsonrpc": "2.0",
                    "error": {"code": -32601, "message": f"Method {method} not found"},
                    "id": req_id,
                },
            )

        return httpx.MockTransport(handler)

    def test_rpc_chain_id_and_network_verification(self, mock_rpc_transport: httpx.MockTransport) -> None:
        with EvmRpcClient(transport=mock_rpc_transport, expected_chain_id=421614) as client:
            chain_id = client.get_chain_id()
            assert chain_id == 421614
            assert client.verify_network() is True

    def test_rpc_chain_id_mismatch_raises_error(self, mock_rpc_transport: httpx.MockTransport) -> None:
        with EvmRpcClient(transport=mock_rpc_transport, expected_chain_id=1) as client:
            with pytest.raises(EvmRpcError) as exc_info:
                client.verify_network()
            assert "Chain ID mismatch" in str(exc_info.value)

    def test_rpc_block_number_and_queries(self, mock_rpc_transport: httpx.MockTransport) -> None:
        with EvmRpcClient(transport=mock_rpc_transport) as client:
            block = client.get_block_number()
            assert block == int("0x12a05f", 16)

            latest_block = client.get_latest_block()
            assert latest_block.get("number") == "0x12a05f"

            base_fee = client.get_base_fee()
            assert base_fee == int("0x5f5e100", 16)

            priority_fee = client.get_max_priority_fee()
            assert priority_fee == int("0xf4240", 16)

            gas_price = client.get_gas_price()
            assert gas_price == int("0x5f5e100", 16)

            gas = client.estimate_gas(
                to=ARBITRUM_SEPOLIA_USDC_ADDRESS,
                data=TRANSFER_WITH_AUTHORIZATION_SELECTOR,
            )
            assert gas == int("0x15f90", 16)

    def test_rpc_check_authorization_state_and_balance(self, mock_rpc_transport: httpx.MockTransport) -> None:
        with EvmRpcClient(transport=mock_rpc_transport) as client:
            is_used = client.check_authorization_state(
                token_address=ARBITRUM_SEPOLIA_USDC_ADDRESS,
                authorizer="0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
                nonce="0x" + "00" * 32,
            )
            assert is_used is False

            bal = client.get_token_balance(
                token_address=ARBITRUM_SEPOLIA_USDC_ADDRESS,
                account="0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
            )
            assert bal == 500_000_000

    def test_rpc_transaction_submission_and_receipt(self, mock_rpc_transport: httpx.MockTransport) -> None:
        with EvmRpcClient(transport=mock_rpc_transport) as client:
            raw_tx = bytes.fromhex("02f87283066eee")
            tx_hash = client.send_raw_transaction(raw_tx)
            assert tx_hash.startswith("0x8888")

            receipt = client.wait_for_transaction_receipt(tx_hash, timeout_seconds=2.0)
            assert receipt["transactionHash"] == tx_hash
            assert receipt["status"] == "0x1"

    def test_rpc_error_handling(self) -> None:
        def error_handler(request: httpx.Request) -> httpx.Response:
            payload = json.loads(request.read())
            return httpx.Response(
                200,
                json={
                    "jsonrpc": "2.0",
                    "error": {"code": -32000, "message": "execution reverted: nonce already used"},
                    "id": payload["id"],
                },
            )

        with EvmRpcClient(transport=httpx.MockTransport(error_handler)) as client:
            with pytest.raises(EvmRpcRequestError) as exc_info:
                client.execute_rpc("eth_sendRawTransaction", ["0x1234"])
            assert exc_info.value.code == -32000
            assert "execution reverted" in exc_info.value.rpc_message

    def test_rpc_connection_failure(self) -> None:
        def failure_handler(request: httpx.Request) -> httpx.Response:
            raise httpx.ConnectError("RPC connection refused")

        with EvmRpcClient(transport=httpx.MockTransport(failure_handler)) as client:
            with pytest.raises(EvmRpcConnectionError):
                client.get_chain_id()


# ==============================================================================
# 4. Gateway Arbitrum Sepolia Integration Tests
# ==============================================================================


class TestArbitrumSepoliaGatewayIntegration:
    """Verifies gateway YAML loading, challenge generation, and signing on Arbitrum Sepolia."""

    def test_load_gateway_arbitrum_sepolia_yaml(self) -> None:
        yaml_path = Path("examples/paid-api/gateway-arbitrum-sepolia.yaml")
        assert yaml_path.exists()

        config = load_gateway_config(yaml_path)
        assert config.version == "1.0"
        assert config.settlement.default_network == ARBITRUM_SEPOLIA_CAIP2
        assert config.settlement.default_asset == ARBITRUM_SEPOLIA_USDC_ADDRESS.lower()

        # Check loaded routes
        routes = config.routes
        assert len(routes) == 2
        r1 = routes[0]
        assert r1.path == "/v1/market-data"
        assert r1.price_base_units == 1000  # 0.001 USDC
        assert r1.network == ARBITRUM_SEPOLIA_CAIP2
        assert r1.asset == ARBITRUM_SEPOLIA_USDC_ADDRESS.lower()

        r2 = routes[1]
        assert r2.path == "/v1/ai/generate"
        assert r2.price_base_units == 10000  # 0.01 USDC

    def test_challenge_manager_generates_arbitrum_sepolia_challenge(self) -> None:
        cm = ChallengeManager(
            facilitator_config=FacilitatorConfig(url="http://facilitator:8001"),
            settlement_defaults=SettlementDefaults(
                default_network=ARBITRUM_SEPOLIA_CAIP2,
                default_asset=ARBITRUM_SEPOLIA_USDC_ADDRESS,
                default_pay_to=MERCHANT_PAYEE,
                default_asset_symbol="USDC",
                default_asset_decimals=6,
            ),
        )

        yaml_path = Path("examples/paid-api/gateway-arbitrum-sepolia.yaml")
        config = load_gateway_config(yaml_path)
        route = config.routes[0]

        challenge = cm.generate_challenge(route, "/v1/market-data")
        assert challenge.network == ARBITRUM_SEPOLIA_CAIP2
        assert challenge.asset.lower() == ARBITRUM_SEPOLIA_USDC_ADDRESS.lower()
        assert challenge.payee.lower() == MERCHANT_PAYEE.lower()
        assert challenge.amount == "1000"

        # Sign challenge with LocalSigner and verify chain ID extraction
        signer = LocalSigner(BUYER_KEY)
        token = ReservationToken(
            reservation_id="res_arb_e2e",
            intent_id=challenge.challenge_id,
            policy_id="pol_default",
            amount=int(challenge.amount),
            expires_at=challenge.valid_before,
            created_at=challenge.valid_after,
        )
        auth = signer.sign_payment_authorization(
            challenge,
            token,
            nonce=generate_nonce(),
            token_name=ARBITRUM_SEPOLIA_USDC_NAME,
            token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
        )

        assert auth.payer.lower() == signer.address.lower()
        assert auth.signature.v in (27, 28)
        assert auth.signature.compact.startswith("0x")
        assert len(auth.signature.compact) == 132
