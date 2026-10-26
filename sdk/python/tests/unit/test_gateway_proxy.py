"""
Unit tests for Gateway Reverse Proxy Forwarding, Header Sanitization, Upstream Secret Isolation, and Structured Logging (T5.3).
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
from agentpay.client.signer import LocalSigner, generate_nonce
from agentpay.core.codecs import HEADER_PAYMENT_SIGNATURE, encode_authorization
from agentpay.core.models import PaymentChallenge
from agentpay.core.state import FulfillmentState
from agentpay.policy.models import ReservationToken
from agentpay_gateway.config.models import FacilitatorConfig, RouteConfig, SettlementDefaults
from agentpay_gateway.config.router import RouteMatcher
from agentpay_gateway.facilitator.service import MockFacilitatorService
from agentpay_gateway.payment.challenge import ChallengeManager
from agentpay_gateway.payment.verifier import PaymentVerifier
from agentpay_gateway.proxy.forwarder import ProxyForwarder
from agentpay_gateway.proxy.headers import (
    sanitize_request_headers,
    sanitize_response_headers,
)
from agentpay_gateway.proxy.logging import StructuredAuditLogger, redact_sensitive_headers
from agentpay_gateway.proxy.service import GatewayProxyService
from agentpay_gateway.state.coordinator import GatewayStateCoordinator
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database

MERCHANT_PAY_TO = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
BASE_USDC_ASSET = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
NETWORK_ID = "eip155:84532"
BUYER_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"


# ==============================================================================
# Header Sanitization & Upstream Secret Isolation Tests
# ==============================================================================


class TestHeaderSanitization:
    """Tests for hop-by-hop stripping and upstream credential isolation."""

    def test_strip_hop_by_hop_and_payment_headers(self) -> None:
        client_headers = {
            "Host": "api.agentpay.dev",
            "Connection": "keep-alive",
            "Keep-Alive": "timeout=5",
            "Upgrade": "websocket",
            "Payment-Signature": "sig_bytes_here",
            "PAYMENT-REQUIRED": "chall_header",
            "Accept": "application/json",
            "User-Agent": "AgentPay-Client/1.0",
        }
        sanitized = sanitize_request_headers(client_headers)

        assert "Accept" in sanitized
        assert "User-Agent" in sanitized
        assert "Host" not in sanitized
        assert "Connection" not in sanitized
        assert "Keep-Alive" not in sanitized
        assert "Upgrade" not in sanitized
        assert "Payment-Signature" not in sanitized
        assert "PAYMENT-REQUIRED" not in sanitized

    def test_context_injection(self) -> None:
        sanitized = sanitize_request_headers(
            client_headers={"Accept": "*/*"},
            client_ip="198.51.100.42",
            client_host="gateway.internal",
            scheme="https",
            payer_address="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
            settlement_id="stl_12345",
            fulfillment_id="ful_67890",
            request_id="req_abcde",
        )
        assert sanitized["X-Forwarded-For"] == "198.51.100.42"
        assert sanitized["X-Forwarded-Proto"] == "https"
        assert sanitized["X-Forwarded-Host"] == "gateway.internal"
        assert sanitized["X-AgentPay-Payer"] == "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
        assert sanitized["X-AgentPay-Settlement-Id"] == "stl_12345"
        assert sanitized["X-AgentPay-Fulfillment-Id"] == "ful_67890"
        assert sanitized["X-Request-Id"] == "req_abcde"

    def test_upstream_secret_isolation_overwrites_client(self) -> None:
        # Threat model test: Client attempts to forge or override internal API key
        client_headers = {
            "X-Internal-Secret": "client_attacker_forged_key",
            "Authorization": "Bearer client_attacker_token",
            "Accept": "application/json",
        }
        upstream_secrets = {
            "X-Internal-Secret": "super_secret_internal_key",
            "Authorization": "Bearer valid_upstream_jwt_token",
        }

        sanitized = sanitize_request_headers(client_headers, upstream_headers=upstream_secrets)

        assert sanitized["X-Internal-Secret"] == "super_secret_internal_key"
        assert sanitized["Authorization"] == "Bearer valid_upstream_jwt_token"
        assert sanitized["Accept"] == "application/json"

    def test_sanitize_response_headers_strips_internal_secrets(self) -> None:
        upstream_resp_headers = {
            "Content-Type": "application/json",
            "Connection": "close",
            "X-Internal-Secret": "super_secret_internal_key",
            "X-Upstream-Trace": "trace_xyz",
        }
        secret_keys = {"X-Internal-Secret"}

        cleaned = sanitize_response_headers(upstream_resp_headers, upstream_secret_keys=secret_keys)
        assert "Content-Type" in cleaned
        assert "X-Upstream-Trace" in cleaned
        assert "Connection" not in cleaned
        assert "X-Internal-Secret" not in cleaned


# ==============================================================================
# Structured Logging & Audit Trail Tests
# ==============================================================================


class TestStructuredAuditLogger:
    """Tests for structured JSON logging and credential redaction."""

    def test_redact_sensitive_headers(self) -> None:
        raw_headers = {
            "Authorization": "Bearer secret_api_jwt_token",
            "X-Internal-Secret": "super_secret_pass",
            "X-Api-Key": "apikey_9999",
            "Content-Type": "application/json",
        }
        redacted = redact_sensitive_headers(raw_headers)
        assert redacted["Authorization"] == "Bearer [REDACTED]"
        assert redacted["X-Internal-Secret"] == "[REDACTED]"
        assert redacted["X-Api-Key"] == "[REDACTED]"
        assert redacted["Content-Type"] == "application/json"

    def test_audit_log_file_output(self, tmp_path: Path) -> None:
        log_file = tmp_path / "audit" / "gateway.log"
        logger = StructuredAuditLogger(audit_log_path=log_file)

        logger.log_request_received(
            request_id="req_001",
            method="GET",
            path="/v1/market-data",
            client_ip="127.0.0.1",
            headers={"Authorization": "Bearer secret", "Accept": "*/*"},
        )
        logger.log_upstream_completed(
            request_id="req_001",
            upstream_url="http://upstream:8080/data",
            status_code=200,
            latency_ms=12.45,
            response_bytes=1024,
        )

        assert log_file.is_file()
        lines = log_file.read_text(encoding="utf-8").strip().split("\n")
        assert len(lines) == 2

        event1 = json.loads(lines[0])
        assert event1["event"] == "request_received"
        assert event1["request_id"] == "req_001"
        assert event1["headers"]["Authorization"] == "Bearer [REDACTED]"

        event2 = json.loads(lines[1])
        assert event2["event"] == "upstream_completed"
        assert event2["status_code"] == 200
        assert event2["latency_ms"] == 12.45


# ==============================================================================
# Proxy Forwarder Tests
# ==============================================================================


class TestProxyForwarder:
    """Tests for synchronous and asynchronous upstream HTTP forwarding."""

    def test_successful_forward(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            assert request.headers.get("X-Forwarded-Proto") == "https"
            return httpx.Response(200, json={"data": "ticker_btc"}, headers={"X-Upstream-Server": "node-1"})

        transport = httpx.MockTransport(handler)
        forwarder = ProxyForwarder(transport=transport)

        resp = forwarder.forward(
            method="GET",
            url="http://mock-upstream/v1/ticker",
            headers={"X-Forwarded-Proto": "https"},
        )
        assert resp.status_code == 200
        assert resp.is_error is False
        assert json.loads(resp.content) == {"data": "ticker_btc"}
        assert resp.headers.get("x-upstream-server") == "node-1"
        assert resp.latency_ms >= 0.0

    def test_upstream_connection_error_returns_502(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            raise httpx.ConnectError("Connection refused by upstream target")

        transport = httpx.MockTransport(handler)
        forwarder = ProxyForwarder(transport=transport)

        resp = forwarder.forward(
            method="GET",
            url="http://unreachable-host:9999/data",
            headers={},
        )
        assert resp.status_code == 502
        assert resp.is_error is True
        assert "Connection refused" in (resp.error_message or "")

    def test_upstream_timeout_returns_504(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:
            raise httpx.TimeoutException("Read timed out")

        transport = httpx.MockTransport(handler)
        forwarder = ProxyForwarder(transport=transport)

        resp = forwarder.forward(
            method="POST",
            url="http://slow-upstream/data",
            headers={},
        )
        assert resp.status_code == 504
        assert resp.is_error is True
        assert "timed out" in (resp.error_message or "").lower()


# ==============================================================================
# End-to-End GatewayProxyService Tests
# ==============================================================================


class TestGatewayProxyService:
    """Tests for end-to-end request handling in GatewayProxyService."""

    @pytest.fixture
    def setup_service(self, tmp_path: Path):
        # 1. Database & Coordinator
        conn = create_gateway_connection(":memory:")
        init_gateway_database(conn)
        coordinator = GatewayStateCoordinator(conn)

        # 2. Challenge & Verifier
        fac_cfg = FacilitatorConfig(url="http://127.0.0.1:8001")
        settlement_defaults = SettlementDefaults(
            default_network=NETWORK_ID,
            default_asset=BASE_USDC_ASSET,
            default_pay_to=MERCHANT_PAY_TO,
        )
        cm = ChallengeManager(fac_cfg, settlement_defaults)
        fac_service = MockFacilitatorService()
        verifier = PaymentVerifier(
            challenge_manager=cm,
            facilitator_client=fac_service,
            state_coordinator=coordinator,
        )

        # 3. Routes
        routes = [
            RouteConfig(
                path="/v1/public/ping",
                upstream="http://mock-upstream/ping",
                price=0,
                pay_to=MERCHANT_PAY_TO,
            ),
            RouteConfig(
                path="/v1/market-data",
                upstream="http://mock-upstream/market-data",
                price="0.001 USDC",
                pay_to=MERCHANT_PAY_TO,
                network=NETWORK_ID,
                asset=BASE_USDC_ASSET,
                upstream_headers={"X-Internal-Token": "secret_backend_token"},
            ),
        ]
        router = RouteMatcher(routes)

        # 4. Mock Upstream Transport
        def upstream_handler(request: httpx.Request) -> httpx.Response:
            path = request.url.path
            if path == "/ping":
                return httpx.Response(200, json={"pong": True})
            if path == "/market-data":
                assert request.headers.get("X-Internal-Token") == "secret_backend_token"
                assert "X-AgentPay-Payer" in request.headers
                return httpx.Response(200, json={"price": "65000", "symbol": "BTC"})
            return httpx.Response(404)

        transport = httpx.MockTransport(upstream_handler)
        forwarder = ProxyForwarder(transport=transport)
        audit_log = tmp_path / "gateway.log"
        logger = StructuredAuditLogger(audit_log_path=audit_log)

        service = GatewayProxyService(
            route_matcher=router,
            payment_verifier=verifier,
            forwarder=forwarder,
            audit_logger=logger,
            state_coordinator=coordinator,
        )
        return service, coordinator, audit_log

    def test_free_route_passes_unauthenticated(self, setup_service: tuple) -> None:
        service, _, _ = setup_service
        resp = service.handle_request(method="GET", path="/v1/public/ping", headers={})
        assert resp.status_code == 200
        assert json.loads(resp.content) == {"pong": True}
        assert "X-Request-Id" in resp.headers

    def test_paid_route_requires_402_challenge(self, setup_service: tuple) -> None:
        service, _, _ = setup_service
        resp = service.handle_request(method="GET", path="/v1/market-data", headers={})
        assert resp.status_code == 402
        assert "PAYMENT-REQUIRED" in resp.headers
        body = json.loads(resp.content)
        assert body["error"] == "payment_required"
        assert body["challenge"]["amount"] == "1000"

    def test_paid_route_verified_and_delivered(self, setup_service: tuple) -> None:
        service, coordinator, audit_log = setup_service

        # 1. Request challenge
        res_402 = service.handle_request(method="GET", path="/v1/market-data", headers={})
        challenge_data = json.loads(res_402.content)["challenge"]
        challenge = PaymentChallenge.model_validate(challenge_data)

        # 2. Buyer signs authorization
        signer = LocalSigner(BUYER_PRIVATE_KEY)
        res_token = ReservationToken(
            reservation_id="res_test",
            intent_id=challenge.challenge_id,
            policy_id="pol_default",
            amount=int(challenge.amount),
            expires_at=challenge.valid_before + 60,
            created_at=challenge.valid_after,
        )
        auth = signer.sign_payment_authorization(
            challenge,
            res_token,
            nonce=generate_nonce(),
            token_name="USD Coin",
            token_version="2",
        )
        auth_header = encode_authorization(auth)

        # 3. Dispatch with PAYMENT-SIGNATURE
        res_paid = service.handle_request(
            method="GET",
            path="/v1/market-data",
            headers={HEADER_PAYMENT_SIGNATURE: auth_header, "Idempotency-Key": "client_order_001"},
        )
        assert res_paid.status_code == 200
        assert json.loads(res_paid.content) == {"price": "65000", "symbol": "BTC"}

        # 4. Verify state machine transitioned to DELIVERED
        cursor = coordinator._conn.cursor()
        cursor.execute("SELECT status, upstream_status_code FROM ap_gateway_fulfillment;")
        row = cursor.fetchone()
        assert row is not None
        assert row[0] == FulfillmentState.DELIVERED.value
        assert row[1] == 200

        # 5. Verify audit log contains full trail
        assert audit_log.is_file()
        lines = [json.loads(line) for line in audit_log.read_text(encoding="utf-8").strip().split("\n")]
        events = [line["event"] for line in lines]
        assert "request_received" in events
        assert "payment_verified" in events
        assert "upstream_completed" in events

    def test_unknown_route_returns_404(self, setup_service: tuple) -> None:
        service, _, _ = setup_service
        resp = service.handle_request(method="GET", path="/v2/non-existent", headers={})
        assert resp.status_code == 404
        assert json.loads(resp.content)["error"] == "not_found"

    def test_method_not_allowed_returns_405(self, setup_service: tuple) -> None:
        service, _, _ = setup_service
        # Public ping route default methods include GET, let's test with a disallowed method
        # RouteConfig default allows GET, POST, PUT, DELETE, PATCH, HEAD.
        # Create route with only GET to test 405:
        get_only_route = RouteConfig(
            path="/v1/readonly",
            upstream="http://mock-upstream/read",
            price=0,
            pay_to=MERCHANT_PAY_TO,
            methods=["GET"],
        )
        service.route_matcher = RouteMatcher([get_only_route])
        resp = service.handle_request(method="POST", path="/v1/readonly", headers={})
        assert resp.status_code == 405
        assert resp.headers.get("Allow") == "GET"
