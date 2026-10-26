"""
Unit tests for Gateway Upstream Recovery (RECOVERY_REQUIRED), Free Replay, and Async Settlement Worker (T5.4).
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
from agentpay.client.signer import LocalSigner, generate_nonce
from agentpay.core.codecs import HEADER_PAYMENT_SIGNATURE, encode_authorization
from agentpay.core.models import PaymentChallenge
from agentpay.core.state import FulfillmentState, SettlementState
from agentpay.policy.models import ReservationToken
from agentpay_gateway.config.models import FacilitatorConfig, RouteConfig, SettlementDefaults
from agentpay_gateway.config.router import RouteMatcher
from agentpay_gateway.facilitator.models import SettleRequest, SettleResponse
from agentpay_gateway.facilitator.service import MockFacilitatorService
from agentpay_gateway.payment.challenge import ChallengeManager
from agentpay_gateway.payment.models import SettlementTask
from agentpay_gateway.payment.settlement import AsyncSettlementWorker
from agentpay_gateway.payment.verifier import PaymentVerifier
from agentpay_gateway.proxy.forwarder import ProxyForwarder
from agentpay_gateway.proxy.logging import StructuredAuditLogger
from agentpay_gateway.proxy.service import GatewayProxyService
from agentpay_gateway.state.coordinator import GatewayStateCoordinator
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database

MERCHANT_PAY_TO = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
BASE_USDC_ASSET = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
NETWORK_ID = "eip155:84532"
BUYER_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"


@pytest.fixture
def coordinator() -> GatewayStateCoordinator:
    conn = create_gateway_connection(":memory:")
    init_gateway_database(conn)
    return GatewayStateCoordinator(conn)


@pytest.fixture
def paid_route() -> RouteConfig:
    return RouteConfig(
        path="/v1/generate",
        methods=["POST"],
        upstream="http://mock-upstream:8080/generate",
        price="0.1 USDC",
        asset=BASE_USDC_ASSET,
        network=NETWORK_ID,
        pay_to=MERCHANT_PAY_TO,
    )


@pytest.fixture
def challenge_manager() -> ChallengeManager:
    return ChallengeManager(
        facilitator_config=FacilitatorConfig(url="http://facilitator.mock"),
        settlement_defaults=SettlementDefaults(
            default_network=NETWORK_ID,
            default_asset=BASE_USDC_ASSET,
            default_pay_to=MERCHANT_PAY_TO,
            default_asset_symbol="USDC",
            default_asset_decimals=6,
        ),
    )


def sign_test_payment(challenge: PaymentChallenge) -> str:
    signer = LocalSigner(BUYER_PRIVATE_KEY)
    token = ReservationToken(
        reservation_id="res_rec_1",
        intent_id=challenge.challenge_id,
        policy_id="pol_rec_default",
        amount=int(challenge.amount),
        expires_at=challenge.valid_before + 60,
        created_at=challenge.valid_after,
    )
    auth = signer.sign_payment_authorization(
        challenge,
        token,
        nonce=generate_nonce(),
        token_name="USD Coin",
        token_version="2",
    )
    return encode_authorization(auth)


# ==============================================================================
# Upstream Failure & Recovery Tests
# ==============================================================================


class TestGatewayRecoveryAndAntiRollback:
    """Verifies that upstream failures enter RECOVERY_REQUIRED and preserve settlement."""

    def test_upstream_500_failure_enters_recovery_required_and_anti_rollback(
        self,
        paid_route: RouteConfig,
        challenge_manager: ChallengeManager,
        coordinator: GatewayStateCoordinator,
    ) -> None:
        matcher = RouteMatcher([paid_route])
        facilitator_mock = MockFacilitatorService()
        verifier = PaymentVerifier(
            challenge_manager=challenge_manager,
            facilitator_client=facilitator_mock,
            state_coordinator=coordinator,
        )

        # Mock transport returning 500 Internal Server Error
        def mock_500_handler(request: httpx.Request) -> httpx.Response:
            return httpx.Response(500, json={"error": "upstream exploded"})

        mock_client = httpx.Client(
            transport=httpx.MockTransport(mock_500_handler),
            trust_env=False,
        )
        forwarder = ProxyForwarder(client=mock_client)
        proxy = GatewayProxyService(
            route_matcher=matcher,
            payment_verifier=verifier,
            forwarder=forwarder,
            state_coordinator=coordinator,
        )

        # Step 1: Initial request without payment -> 402
        resp402 = proxy.handle_request("POST", "/v1/generate", headers={})
        assert resp402.status_code == 402
        challenge = PaymentChallenge.model_validate(json.loads(resp402.content)["challenge"])

        # Step 2: Sign payment and send request
        auth_header = sign_test_payment(challenge)
        idem_key = "idem_recovery_test_001"
        resp = proxy.handle_request(
            "POST",
            "/v1/generate",
            headers={
                HEADER_PAYMENT_SIGNATURE: auth_header,
                "Idempotency-Key": idem_key,
            },
        )

        # Upstream failed with 500
        assert resp.status_code == 500

        # Step 3: Verify Anti-Rollback Invariant
        # The settlement record MUST remain SUBMITTED, not deleted or rolled back
        signer = LocalSigner(BUYER_PRIVATE_KEY)
        eval_result = coordinator.check_idempotency(signer.address, idem_key)
        assert eval_result.is_free_replay is True
        assert eval_result.is_cache_hit is False
        assert eval_result.is_in_flight is False
        assert eval_result.settlement_id is not None
        assert eval_result.fulfillment_id is not None

        # Verify fulfillment status is RECOVERY_REQUIRED in database
        fulfillment = coordinator.get_fulfillment(eval_result.fulfillment_id)
        assert fulfillment is not None
        assert fulfillment.status == FulfillmentState.RECOVERY_REQUIRED
        assert fulfillment.upstream_status_code == 500

        # Verify settlement record is intact in SUBMITTED state
        settlement = coordinator.get_settlement(eval_result.settlement_id)
        assert settlement is not None
        assert settlement.status == SettlementState.SUBMITTED

    def test_client_free_replay_after_failure_and_subsequent_cache_hit(
        self,
        paid_route: RouteConfig,
        challenge_manager: ChallengeManager,
        coordinator: GatewayStateCoordinator,
    ) -> None:
        matcher = RouteMatcher([paid_route])
        facilitator_mock = MockFacilitatorService()
        verifier = PaymentVerifier(
            challenge_manager=challenge_manager,
            facilitator_client=facilitator_mock,
            state_coordinator=coordinator,
        )

        upstream_calls = 0

        def flaky_upstream_handler(request: httpx.Request) -> httpx.Response:
            nonlocal upstream_calls
            upstream_calls += 1
            if upstream_calls == 1:
                return httpx.Response(503, json={"error": "service unavailable temporarily"})
            return httpx.Response(
                200,
                json={"result": "generation completed", "attempt": upstream_calls},
                headers={"X-Generated-Id": "gen_123"},
            )

        mock_client = httpx.Client(
            transport=httpx.MockTransport(flaky_upstream_handler),
            trust_env=False,
        )
        forwarder = ProxyForwarder(client=mock_client)
        proxy = GatewayProxyService(
            route_matcher=matcher,
            payment_verifier=verifier,
            forwarder=forwarder,
            state_coordinator=coordinator,
        )

        # Get Challenge
        resp402 = proxy.handle_request("POST", "/v1/generate", headers={})
        challenge = PaymentChallenge.model_validate(json.loads(resp402.content)["challenge"])
        auth_header = sign_test_payment(challenge)
        idem_key = "idem_flaky_retry_002"

        # Call 1: Upstream fails with 503
        resp1 = proxy.handle_request(
            "POST",
            "/v1/generate",
            headers={
                HEADER_PAYMENT_SIGNATURE: auth_header,
                "Idempotency-Key": idem_key,
            },
        )
        assert resp1.status_code == 503
        assert upstream_calls == 1

        # Check recovery state
        signer = LocalSigner(BUYER_PRIVATE_KEY)
        eval_before_replay = coordinator.check_idempotency(signer.address, idem_key)
        assert eval_before_replay.is_free_replay is True
        stl_id_1 = eval_before_replay.settlement_id

        # Call 2: Client retries with the exact same Idempotency-Key and Payment Signature
        # Must be treated as FREE_REPLAY: no new verification, no second charge, forwarded for free
        resp2 = proxy.handle_request(
            "POST",
            "/v1/generate",
            headers={
                HEADER_PAYMENT_SIGNATURE: auth_header,
                "Idempotency-Key": idem_key,
            },
        )
        assert resp2.status_code == 200
        assert upstream_calls == 2
        body2 = json.loads(resp2.content)
        assert body2["result"] == "generation completed"
        assert body2["attempt"] == 2

        # Check state: fulfillment transitioned to DELIVERED
        fulfillment = coordinator.get_fulfillment(eval_before_replay.fulfillment_id)
        assert fulfillment is not None
        assert fulfillment.status == FulfillmentState.DELIVERED
        assert fulfillment.upstream_status_code == 200

        # Settlement ID unchanged
        assert fulfillment.settlement_id == stl_id_1

        # Call 3: Client retries again with same Idempotency-Key -> Hits Cache without upstream call
        resp3 = proxy.handle_request(
            "POST",
            "/v1/generate",
            headers={
                HEADER_PAYMENT_SIGNATURE: auth_header,
                "Idempotency-Key": idem_key,
            },
        )
        assert resp3.status_code == 200
        assert resp3.headers.get("X-Cache") == "HIT"
        assert upstream_calls == 2  # Upstream was NOT invoked a third time
        body3 = json.loads(resp3.content)
        assert body3["result"] == "generation completed"
        assert body3["attempt"] == 2


# ==============================================================================
# In-Process Async Settlement Worker Tests
# ==============================================================================


class TestAsyncSettlementWorker:
    """Verifies background queue processing, Facilitator clearing, and error retries."""

    def test_worker_processes_settlement_and_transitions_db_state(
        self,
        coordinator: GatewayStateCoordinator,
        paid_route: RouteConfig,
        challenge_manager: ChallengeManager,
    ) -> None:
        class ConfirmingFacilitator(MockFacilitatorService):
            def settle(self, request: SettleRequest) -> SettleResponse:
                super().settle(request)
                return SettleResponse(
                    settlement_id="stl_confirmed_1",
                    status=SettlementState.CONFIRMED,
                    queue_position=1,
                )

        facilitator_mock = ConfirmingFacilitator()
        audit_logger = StructuredAuditLogger()
        worker = AsyncSettlementWorker(
            facilitator_client=facilitator_mock,
            state_coordinator=coordinator,
            audit_logger=audit_logger,
            max_retries=2,
            retry_delay_seconds=0.01,
        )

        try:
            verifier = PaymentVerifier(
                challenge_manager=challenge_manager,
                facilitator_client=facilitator_mock,
                state_coordinator=coordinator,
                settlement_worker=worker,
            )

            challenge = challenge_manager.generate_challenge(paid_route, "/v1/generate")
            auth_header = sign_test_payment(challenge)
            from agentpay.core.codecs import decode_authorization

            auth = decode_authorization(auth_header)

            res = verifier.verify_authorization(
                route=paid_route,
                auth=auth,
                challenge=challenge,
                idempotency_key="idem_worker_test_1",
            )
            assert res.settlement_id is not None

            # Wait for worker to drain
            worker.drain(timeout=2.0)
            assert worker.settled_count == 1
            assert worker.failed_count == 0

            # Verify settlement record transitioned to CONFIRMED
            record_after = coordinator.get_settlement(res.settlement_id)
            assert record_after is not None
            assert record_after.status == SettlementState.CONFIRMED
        finally:
            worker.stop()

    def test_worker_retries_transient_failure_then_succeeds(
        self,
        coordinator: GatewayStateCoordinator,
        paid_route: RouteConfig,
        challenge_manager: ChallengeManager,
    ) -> None:
        call_count = 0

        class FlakyFacilitator:
            def settle(self, request: SettleRequest) -> SettleResponse:
                nonlocal call_count
                call_count += 1
                if call_count == 1:
                    raise ConnectionResetError("Temporary network glitch to blockchain node")
                return SettleResponse(
                    settlement_id="fac_stl_ok",
                    status=SettlementState.CONFIRMED,
                    queue_position=1,
                )

        worker = AsyncSettlementWorker(
            facilitator_client=FlakyFacilitator(),
            state_coordinator=coordinator,
            max_retries=3,
            retry_delay_seconds=0.01,
        )

        try:
            challenge = challenge_manager.generate_challenge(paid_route, "/v1/generate")
            auth_header = sign_test_payment(challenge)
            from agentpay.core.codecs import decode_authorization

            auth = decode_authorization(auth_header)

            task = SettlementTask(
                settlement_id="stl_flaky_test_1",
                verification_token="tok_123",
                authorization=auth,
                idempotency_key="idem_flaky_worker",
            )
            worker.submit(task)
            worker.drain(timeout=2.0)

            assert call_count == 2
            assert worker.settled_count == 1
            assert worker.failed_count == 0
        finally:
            worker.stop()

    def test_worker_reaches_max_retries_and_marks_failure(
        self,
        paid_route: RouteConfig,
        challenge_manager: ChallengeManager,
    ) -> None:
        class AlwaysFailingFacilitator:
            def settle(self, request: SettleRequest) -> SettleResponse:
                raise RuntimeError("Permanent consensus failure")

        worker = AsyncSettlementWorker(
            facilitator_client=AlwaysFailingFacilitator(),
            max_retries=2,
            retry_delay_seconds=0.01,
        )

        try:
            challenge = challenge_manager.generate_challenge(paid_route, "/v1/generate")
            auth_header = sign_test_payment(challenge)
            from agentpay.core.codecs import decode_authorization

            auth = decode_authorization(auth_header)

            task = SettlementTask(
                settlement_id="stl_permanent_fail",
                verification_token="tok_fail",
                authorization=auth,
            )
            worker.submit(task)
            worker.drain(timeout=2.0)

            assert worker.settled_count == 0
            assert worker.failed_count == 1
        finally:
            worker.stop()

    def test_end_to_end_optimistic_fulfillment_and_async_clearing(
        self,
        paid_route: RouteConfig,
        challenge_manager: ChallengeManager,
        coordinator: GatewayStateCoordinator,
        tmp_path: Path,
    ) -> None:
        """
        Verify end-to-end:
        1. Client receives 200 OK immediately with upstream data (optimistic fulfillment).
        2. AsyncSettlementWorker finishes clearing on-chain settlement in the background.
        3. Structured audit log contains all lifecycle events.
        """
        log_file = tmp_path / "gateway_audit.log"
        audit_logger = StructuredAuditLogger(audit_log_path=log_file)

        class ConfirmingFacilitator(MockFacilitatorService):
            def settle(self, request: SettleRequest) -> SettleResponse:
                super().settle(request)
                return SettleResponse(
                    settlement_id="stl_async_cleared_e2e",
                    status=SettlementState.CONFIRMED,
                    queue_position=1,
                )

        facilitator_mock = ConfirmingFacilitator()
        worker = AsyncSettlementWorker(
            facilitator_client=facilitator_mock,
            state_coordinator=coordinator,
            audit_logger=audit_logger,
            max_retries=2,
            retry_delay_seconds=0.01,
        )

        try:
            matcher = RouteMatcher([paid_route])
            verifier = PaymentVerifier(
                challenge_manager=challenge_manager,
                facilitator_client=facilitator_mock,
                state_coordinator=coordinator,
                settlement_worker=worker,
            )

            def mock_upstream(request: httpx.Request) -> httpx.Response:
                return httpx.Response(200, json={"model": "gpt-fast", "response": "Hello world!"})

            mock_client = httpx.Client(
                transport=httpx.MockTransport(mock_upstream),
                trust_env=False,
            )
            forwarder = ProxyForwarder(client=mock_client)
            proxy = GatewayProxyService(
                route_matcher=matcher,
                payment_verifier=verifier,
                forwarder=forwarder,
                audit_logger=audit_logger,
                state_coordinator=coordinator,
            )

            # 1. Challenge
            resp402 = proxy.handle_request("POST", "/v1/generate", headers={})
            assert resp402.status_code == 402
            challenge = PaymentChallenge.model_validate(json.loads(resp402.content)["challenge"])

            # 2. Pay and Send
            auth_header = sign_test_payment(challenge)
            idem_key = "idem_async_e2e_001"
            resp = proxy.handle_request(
                "POST",
                "/v1/generate",
                headers={
                    HEADER_PAYMENT_SIGNATURE: auth_header,
                    "Idempotency-Key": idem_key,
                },
            )

            # Instant upstream success
            assert resp.status_code == 200
            data = json.loads(resp.content)
            assert data["model"] == "gpt-fast"

            # 3. Wait for background settlement to drain
            worker.drain(timeout=2.0)
            assert worker.settled_count == 1

            # 4. State verification
            signer = LocalSigner(BUYER_PRIVATE_KEY)
            eval_res = coordinator.check_idempotency(signer.address, idem_key)
            assert eval_res.is_cache_hit is True
            assert eval_res.settlement_id is not None
            assert eval_res.fulfillment_id is not None

            fulfillment = coordinator.get_fulfillment(eval_res.fulfillment_id)
            assert fulfillment is not None
            assert fulfillment.status == FulfillmentState.DELIVERED

            settlement = coordinator.get_settlement(eval_res.settlement_id)
            assert settlement is not None
            assert settlement.status == SettlementState.CONFIRMED

            # 5. Audit log verification
            assert log_file.exists()
            log_lines = [json.loads(line) for line in log_file.read_text().strip().split("\n")]
            event_types = [entry["event"] for entry in log_lines]
            assert "request_received" in event_types
            assert "challenge_issued" in event_types
            assert "payment_verified" in event_types
            assert "upstream_completed" in event_types
            assert "settlement_submitted" in event_types
        finally:
            worker.stop()
