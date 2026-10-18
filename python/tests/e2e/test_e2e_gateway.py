"""
End-to-End (E2E) integration tests replacing Mock Seller with AgentPay Gateway (T5.5).

Validates the full autonomous financial negotiation loop:
  Buyer Agent (httpx + PolicyEngine + LocalSigner + AgentPayClient)
    <---> AgentPay Gateway (GatewayProxyService + RouteMatcher + PaymentVerifier + AsyncSettlementWorker)
    <---> Facilitator (MockFacilitatorService for clearing)
    <---> Protected Upstream API (Target Backend Service)
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import httpx
import pytest
from agentpay.client.client import AgentPayClient, AsyncAgentPayClient
from agentpay.client.signer import LocalSigner
from agentpay.core.state import FulfillmentState, SettlementState
from agentpay.policy.db import create_buyer_connection, init_buyer_database
from agentpay.policy.errors import PolicyErrorCode, PolicyViolationError
from agentpay.policy.models import LedgerEntryState, PolicyConfig
from agentpay.policy.sqlite_engine import SQLitePolicyEngine
from agentpay_gateway.config.models import FacilitatorConfig, RouteConfig, SettlementDefaults
from agentpay_gateway.config.router import RouteMatcher
from agentpay_gateway.facilitator.service import MockFacilitatorService
from agentpay_gateway.payment.challenge import ChallengeManager
from agentpay_gateway.payment.settlement import AsyncSettlementWorker
from agentpay_gateway.payment.verifier import PaymentVerifier
from agentpay_gateway.proxy.forwarder import ProxyForwarder
from agentpay_gateway.proxy.logging import StructuredAuditLogger
from agentpay_gateway.proxy.service import GatewayProxyService
from agentpay_gateway.state.coordinator import GatewayStateCoordinator
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database

# Test constants
BUYER_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
SELLER_PAYEE_ADDRESS = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
USDC_ASSET_ADDRESS = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
NETWORK_BASE_SEPOLIA = "eip155:84532"
FIXED_CLOCK_TIME = 1729000100


@pytest.fixture
def test_clock() -> Any:
    current_time = FIXED_CLOCK_TIME

    def _now() -> int:
        return current_time

    return _now


@pytest.fixture
def signer() -> LocalSigner:
    return LocalSigner(BUYER_PRIVATE_KEY)


@pytest.fixture
def buyer_policy_engine(signer: LocalSigner, test_clock: Any) -> SQLitePolicyEngine:
    conn = create_buyer_connection(":memory:")
    init_buyer_database(conn)
    cfg = PolicyConfig(
        id="pol_gateway_e2e",
        name="Gateway E2E Policy",
        wallet_address=signer.address,
        max_per_tx_base_units=100000,  # 0.1 USDC max per tx
        rolling_budget_24h_base_units=1000000,  # 1.0 USDC daily budget
        whitelist_payees=[SELLER_PAYEE_ADDRESS],
        whitelist_assets=[USDC_ASSET_ADDRESS],
        whitelist_networks=[NETWORK_BASE_SEPOLIA],
        whitelist_domains=["gateway.internal", "localhost"],
        circuit_consecutive_failures=3,
        circuit_cooldown_seconds=60,
    )
    return SQLitePolicyEngine(conn, cfg, clock=test_clock)


@pytest.fixture
def facilitator(test_clock: Any) -> MockFacilitatorService:
    return MockFacilitatorService(clock=test_clock)


@pytest.fixture
def gateway_coordinator(test_clock: Any) -> GatewayStateCoordinator:
    conn = create_gateway_connection(":memory:")
    init_gateway_database(conn)
    return GatewayStateCoordinator(conn, clock=test_clock)


@pytest.fixture
def gateway_setup(
    facilitator: MockFacilitatorService,
    gateway_coordinator: GatewayStateCoordinator,
    test_clock: Any,
    tmp_path: Path,
):
    """Builds a real GatewayProxyService backed by an upstream mock transport."""
    audit_file = tmp_path / "gateway_audit.log"
    logger = StructuredAuditLogger(audit_log_path=audit_file)

    settlement_worker = AsyncSettlementWorker(
        facilitator_client=facilitator,
        state_coordinator=gateway_coordinator,
        audit_logger=logger,
        max_retries=2,
        retry_delay_seconds=0.01,
        clock=test_clock,
    )

    routes = [
        RouteConfig(
            path="/v1/market-data",
            upstream="http://backend.upstream:8080/internal/market-data",
            price="0.05 USDC",  # 50,000 base units
            pay_to=SELLER_PAYEE_ADDRESS,
            network=NETWORK_BASE_SEPOLIA,
            asset=USDC_ASSET_ADDRESS,
            upstream_headers={"X-Internal-Key": "secret_backend_token"},
        ),
        RouteConfig(
            path="/v1/expensive-inference",
            upstream="http://backend.upstream:8080/internal/expensive",
            price="0.5 USDC",  # 500,000 base units (exceeds buyer's 0.1 USDC cap)
            pay_to=SELLER_PAYEE_ADDRESS,
            network=NETWORK_BASE_SEPOLIA,
            asset=USDC_ASSET_ADDRESS,
        ),
        RouteConfig(
            path="/v1/flaky-service",
            upstream="http://backend.upstream:8080/internal/flaky",
            price="0.02 USDC",  # 20,000 base units
            pay_to=SELLER_PAYEE_ADDRESS,
            network=NETWORK_BASE_SEPOLIA,
            asset=USDC_ASSET_ADDRESS,
        ),
    ]

    matcher = RouteMatcher(routes)
    challenge_mgr = ChallengeManager(
        facilitator_config=FacilitatorConfig(url="http://facilitator.internal"),
        settlement_defaults=SettlementDefaults(
            default_network=NETWORK_BASE_SEPOLIA,
            default_asset=USDC_ASSET_ADDRESS,
            default_pay_to=SELLER_PAYEE_ADDRESS,
        ),
        clock=test_clock,
    )

    verifier = PaymentVerifier(
        challenge_manager=challenge_mgr,
        facilitator_client=facilitator,
        state_coordinator=gateway_coordinator,
        settlement_worker=settlement_worker,
        clock=test_clock,
    )

    upstream_state = {
        "call_count": 0,
        "flaky_calls": 0,
        "last_headers": {},
    }

    def upstream_handler(request: httpx.Request) -> httpx.Response:
        upstream_state["call_count"] += 1
        upstream_state["last_headers"] = dict(request.headers)

        if "/internal/market-data" in request.url.path:
            return httpx.Response(
                200,
                json={"ticker": "BTC/USD", "price": "67500.50", "status": "fresh"},
                headers={"X-Upstream-Backend": "cluster-node-1"},
            )

        if "/internal/flaky" in request.url.path:
            upstream_state["flaky_calls"] += 1
            if upstream_state["flaky_calls"] == 1:
                return httpx.Response(502, json={"error": "upstream database timeout"})
            return httpx.Response(200, json={"status": "recovered", "attempt": upstream_state["flaky_calls"]})

        return httpx.Response(404, json={"error": "upstream route not found"})

    mock_client = httpx.Client(
        transport=httpx.MockTransport(upstream_handler),
        trust_env=False,
    )
    forwarder = ProxyForwarder(client=mock_client)

    gateway = GatewayProxyService(
        route_matcher=matcher,
        payment_verifier=verifier,
        forwarder=forwarder,
        audit_logger=logger,
        state_coordinator=gateway_coordinator,
    )

    return gateway, settlement_worker, gateway_coordinator, upstream_state, audit_file


# ==============================================================================
# End-to-End Tests: Buyer Agent <-> Gateway <-> Upstream
# ==============================================================================


class TestE2EGatewaySync:
    """Synchronous Buyer Agent <---> Gateway <---> Upstream tests."""

    def test_e2e_sync_happy_path(
        self,
        signer: LocalSigner,
        buyer_policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        gateway_setup: tuple,
        test_clock: Any,
    ) -> None:
        """
        Complete E2E negotiation loop:
          1. Buyer GET /v1/market-data
          2. Gateway returns 402 with x402 V2 Challenge
          3. AgentPayClient automatically reserves budget, signs EIP-3009 authorization, retries
          4. Gateway verifies authorization, records settlement in SQLite, forwards to Upstream
          5. Upstream receives sanitized headers + injected secret + payer identity
          6. Gateway delivers 200 OK to Buyer Agent
          7. Buyer commits budget in its SQLite ledger
          8. Gateway AsyncSettlementWorker finishes on-chain clearing
          9. Subsequent request hits Gateway cache directly
        """
        gateway, worker, coordinator, upstream_state, audit_file = gateway_setup

        try:
            with AgentPayClient(
                signer=signer,
                policy=buyer_policy_engine,
                transport=gateway.as_transport(),
                clock=test_clock,
            ) as client:
                # 1. Execute paid request through gateway
                resp = client.get("http://gateway.internal/v1/market-data")

                # Assert 1: Transparent 200 OK delivered to buyer agent
                assert resp.status_code == 200
                data = resp.json()
                assert data["ticker"] == "BTC/USD"
                assert data["price"] == "67500.50"

                # Assert 2: Upstream received injected secret and payer identity
                assert upstream_state["call_count"] == 1
                headers = upstream_state["last_headers"]
                assert headers.get("x-internal-key") == "secret_backend_token"
                assert headers.get("x-agentpay-payer") == signer.address.lower()
                assert "x-agentpay-settlement-id" in headers

                # Upstream secret was NOT leaked to buyer
                assert "x-internal-key" not in resp.headers
                assert "X-Internal-Key" not in resp.headers

                # Assert 3: Buyer's budget ledger is COMMITTED
                row = buyer_policy_engine._conn.execute(
                    "SELECT state, amount_base_units FROM ap_budget_ledger WHERE wallet_address = ?",
                    (signer.address.lower(),),
                ).fetchone()
                assert row is not None
                assert row[0] == LedgerEntryState.COMMITTED.value
                assert row[1] == 50000

                # Assert 4: Drain gateway async settlement worker
                worker.drain(timeout=2.0)
                assert worker.settled_count == 1

                # Assert 5: Gateway state coordinator records settlement as SUBMITTED / CONFIRMED
                settlements = coordinator.list_settlements()
                assert len(settlements) == 1
                settlement = settlements[0]
                assert settlement.status in (SettlementState.SUBMITTED, SettlementState.CONFIRMED)
                assert settlement.amount_base_units == 50000
                assert settlement.payer_address == signer.address.lower()

                # Assert 6: Second request with client idempotency key hits CACHE
                resp2 = client.get(
                    "http://gateway.internal/v1/market-data", headers={"Idempotency-Key": settlement.idempotency_key}
                )
                assert resp2.status_code == 200
                assert resp2.headers.get("X-Cache") == "HIT"
                assert upstream_state["call_count"] == 1  # Upstream was NOT invoked again!
        finally:
            worker.stop()

    def test_e2e_buyer_policy_rejection_prevents_payment(
        self,
        signer: LocalSigner,
        buyer_policy_engine: SQLitePolicyEngine,
        gateway_setup: tuple,
        test_clock: Any,
    ) -> None:
        """
        Verify financial firewall:
        Buyer policy limits per-tx to 100,000 base units.
        Route requires 500,000 base units.
        Buyer Agent raises PolicyViolationError without signing; Gateway never settles.
        """
        gateway, worker, coordinator, upstream_state, _ = gateway_setup

        try:
            with AgentPayClient(
                signer=signer,
                policy=buyer_policy_engine,
                transport=gateway.as_transport(),
                raise_on_policy_rejection=True,
                clock=test_clock,
            ) as client:
                with pytest.raises(PolicyViolationError) as exc_info:
                    client.get("http://gateway.internal/v1/expensive-inference")

                assert exc_info.value.code == PolicyErrorCode.SINGLE_TX_CAP_EXCEEDED
                assert upstream_state["call_count"] == 0

                # No settlement or fulfillment recorded in gateway
                settlements = coordinator.list_settlements()
                assert len(settlements) == 0
        finally:
            worker.stop()

    def test_e2e_upstream_failure_recovery_and_free_replay(
        self,
        signer: LocalSigner,
        buyer_policy_engine: SQLitePolicyEngine,
        gateway_setup: tuple,
        test_clock: Any,
    ) -> None:
        """
        Test Anti-Rollback Invariant and Free Replay across Buyer <-> Gateway <-> Upstream:
        1. Buyer sends request to flaky route with custom Idempotency-Key.
        2. First upstream attempt fails with 502 -> Gateway enters RECOVERY_REQUIRED.
        3. Settlement record remains SUBMITTED (Anti-Rollback Invariant).
        4. Buyer retries with the SAME Idempotency-Key.
        5. Gateway accepts as FREE_REPLAY (zero secondary charges/settlements).
        6. Upstream recovers and returns 200 OK.
        7. Gateway caches result and marks fulfillment DELIVERED.
        8. Third attempt hits CACHE without invoking upstream.
        """
        gateway, worker, coordinator, upstream_state, _ = gateway_setup

        try:
            custom_headers = {"Idempotency-Key": "client_req_flaky_999"}

            with AgentPayClient(
                signer=signer,
                policy=buyer_policy_engine,
                transport=gateway.as_transport(),
                clock=test_clock,
            ) as client:
                # Step 1: First call fails upstream
                resp1 = client.get("http://gateway.internal/v1/flaky-service", headers=custom_headers)
                assert resp1.status_code == 502
                assert upstream_state["flaky_calls"] == 1

                # Verify Anti-Rollback Invariant:
                eval_rec = coordinator.check_idempotency(signer.address, "client_req_flaky_999")
                assert eval_rec.is_free_replay is True
                assert eval_rec.settlement_id is not None
                assert eval_rec.fulfillment_id is not None

                ful = coordinator.get_fulfillment(eval_rec.fulfillment_id)
                assert ful is not None
                assert ful.status == FulfillmentState.RECOVERY_REQUIRED

                stl = coordinator.get_settlement(eval_rec.settlement_id)
                assert stl is not None
                assert stl.status in (SettlementState.SUBMITTED, SettlementState.CONFIRMED)

                # Step 2: Buyer retries with SAME Idempotency-Key -> Handled as FREE_REPLAY
                resp2 = client.get("http://gateway.internal/v1/flaky-service", headers=custom_headers)
                assert resp2.status_code == 200
                assert upstream_state["flaky_calls"] == 2
                body2 = resp2.json()
                assert body2["status"] == "recovered"

                # State is now DELIVERED
                ful_delivered = coordinator.get_fulfillment(eval_rec.fulfillment_id)
                assert ful_delivered is not None
                assert ful_delivered.status == FulfillmentState.DELIVERED

                # Step 3: Third call hits CACHE without invoking upstream
                resp3 = client.get("http://gateway.internal/v1/flaky-service", headers=custom_headers)
                assert resp3.status_code == 200
                assert resp3.headers.get("X-Cache") == "HIT"
                assert upstream_state["flaky_calls"] == 2  # Not called again
        finally:
            worker.stop()


# ==============================================================================
# End-to-End Tests: Async Buyer Agent <-> Gateway <-> Upstream
# ==============================================================================


class TestE2EGatewayAsync:
    """Asynchronous Buyer Agent <---> Gateway <---> Upstream tests."""

    @pytest.mark.anyio
    async def test_e2e_async_happy_path(
        self,
        signer: LocalSigner,
        buyer_policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        gateway_setup: tuple,
        test_clock: Any,
    ) -> None:
        """
        Verify async execution path via AsyncAgentPayClient and AsyncGatewayTransport.
        """
        gateway, worker, coordinator, upstream_state, _ = gateway_setup

        try:
            async with AsyncAgentPayClient(
                signer=signer,
                policy=buyer_policy_engine,
                transport=gateway.as_async_transport(),
                clock=test_clock,
            ) as client:
                resp = await client.get("http://gateway.internal/v1/market-data")
                assert resp.status_code == 200
                data = resp.json()
                assert data["ticker"] == "BTC/USD"
                assert upstream_state["call_count"] == 1

                # Flush worker
                worker.drain(timeout=2.0)
                assert worker.settled_count == 1
        finally:
            worker.stop()
