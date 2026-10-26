"""
High-Level Reverse Proxy Service for AgentPay Gateway.

Integrates:
- Longest Prefix Match (LPM) routing engine
- x402 V2 payment challenge & verification pipeline
- Request header sanitization & upstream credential isolation
- Anti-SSRF upstream HTTP forwarding
- Tri-state fulfillment synchronization (DELIVERING / DELIVERED / RECOVERY_REQUIRED)
- Structured JSON audit logging
"""

from __future__ import annotations

import json
import uuid
from collections.abc import Mapping

import httpx
from pydantic import BaseModel, ConfigDict

from agentpay_gateway.config.errors import MethodNotAllowedError, RouteNotFoundError
from agentpay_gateway.config.router import RouteMatcher
from agentpay_gateway.payment.models import PaymentStatus
from agentpay_gateway.payment.verifier import PaymentVerifier
from agentpay_gateway.proxy.forwarder import ProxyForwarder
from agentpay_gateway.proxy.headers import sanitize_request_headers
from agentpay_gateway.proxy.logging import StructuredAuditLogger
from agentpay_gateway.state.coordinator import GatewayStateCoordinator


class GatewayResponse(BaseModel):
    """Normalized HTTP response returned by GatewayProxyService."""

    model_config = ConfigDict(frozen=True)

    status_code: int
    headers: dict[str, str]
    content: bytes
    request_id: str
    latency_ms: float = 0.0


class GatewayProxyService:
    """
    Core reverse proxy coordinator processing inbound HTTP traffic.
    """

    def __init__(
        self,
        route_matcher: RouteMatcher,
        payment_verifier: PaymentVerifier,
        forwarder: ProxyForwarder | None = None,
        audit_logger: StructuredAuditLogger | None = None,
        state_coordinator: GatewayStateCoordinator | None = None,
    ) -> None:
        self.route_matcher = route_matcher
        self.payment_verifier = payment_verifier
        self.forwarder = forwarder or ProxyForwarder()
        self.audit_logger = audit_logger or StructuredAuditLogger()
        self.state_coordinator = state_coordinator or payment_verifier.state_coordinator

    def handle_request(
        self,
        method: str,
        path: str,
        headers: Mapping[str, str],
        query_string: str | None = None,
        content: bytes | None = None,
        *,
        client_ip: str | None = None,
        scheme: str = "http",
        client_host: str | None = None,
    ) -> GatewayResponse:
        """
        Execute end-to-end gateway proxy pipeline for an inbound HTTP request.
        """
        request_id = f"req_{uuid.uuid4().hex}"

        # 1. Log inbound request
        self.audit_logger.log_request_received(
            request_id=request_id,
            method=method,
            path=path,
            client_ip=client_ip,
            headers=headers,
        )

        # 2. Match Route (Longest Prefix Match)
        try:
            match = self.route_matcher.match(
                method=method,
                path=path,
                query_string=query_string,
            )
        except RouteNotFoundError:
            return GatewayResponse(
                status_code=404,
                headers={"Content-Type": "application/json", "X-Request-Id": request_id},
                content=b'{"error":"not_found","message":"No gateway route configured for path"}',
                request_id=request_id,
            )
        except MethodNotAllowedError as exc:
            return GatewayResponse(
                status_code=405,
                headers={
                    "Content-Type": "application/json",
                    "Allow": ", ".join(exc.allowed_methods),
                    "X-Request-Id": request_id,
                },
                content=json.dumps(
                    {
                        "error": "method_not_allowed",
                        "message": f"Method {method} not allowed on route {path}",
                        "allowed_methods": exc.allowed_methods,
                    }
                ).encode("utf-8"),
                request_id=request_id,
            )

        # 3. Process Payment Lifecycle (x402 Challenge or Verification)
        pay_result = self.payment_verifier.process_request(
            route=match.route,
            resource_uri=path,
            headers=headers,
        )

        # 3a. Payment Required -> Issue HTTP 402 with Challenge
        if pay_result.status == PaymentStatus.CHALLENGE_REQUIRED:
            challenge = pay_result.challenge
            resp_headers: dict[str, str] = {
                "Content-Type": "application/json",
                "X-Request-Id": request_id,
            }
            if pay_result.challenge_header:
                resp_headers["PAYMENT-REQUIRED"] = pay_result.challenge_header

            body_dict = {
                "error": "payment_required",
                "message": pay_result.error_message or "Payment required to access this resource",
                "challenge": challenge.model_dump(by_alias=True, exclude_none=True) if challenge else None,
            }

            if challenge:
                self.audit_logger.log_challenge_issued(
                    request_id=request_id,
                    path=path,
                    challenge_id=challenge.challenge_id,
                    price_base_units=int(challenge.amount),
                    payee=challenge.payee,
                )

            return GatewayResponse(
                status_code=402,
                headers=resp_headers,
                content=json.dumps(body_dict).encode("utf-8"),
                request_id=request_id,
            )

        # 3b. Cache Hit -> Return previously delivered upstream response immediately
        if pay_result.status == PaymentStatus.CACHE_HIT:
            resp_headers = {"X-Request-Id": request_id, "X-Cache": "HIT"}
            if pay_result.cached_headers_json:
                try:
                    loaded = json.loads(pay_result.cached_headers_json)
                    resp_headers.update(loaded)
                except Exception:
                    pass

            return GatewayResponse(
                status_code=pay_result.cached_status_code or 200,
                headers=resp_headers,
                content=pay_result.cached_body_blob or b"",
                request_id=request_id,
            )

        # 3c. Payment Rejected or Fatal Client Error
        if pay_result.status == PaymentStatus.REJECTED:
            return GatewayResponse(
                status_code=pay_result.http_status_code,
                headers={"Content-Type": "application/json", "X-Request-Id": request_id},
                content=json.dumps(
                    {
                        "error": "payment_rejected",
                        "message": pay_result.error_message or "Payment verification was rejected",
                    }
                ).encode("utf-8"),
                request_id=request_id,
            )

        # 3d. Gateway or Facilitator Error
        if pay_result.status == PaymentStatus.ERROR:
            return GatewayResponse(
                status_code=pay_result.http_status_code,
                headers={"Content-Type": "application/json", "X-Request-Id": request_id},
                content=json.dumps(
                    {
                        "error": "gateway_error",
                        "message": pay_result.error_message or "Gateway payment processing error",
                    }
                ).encode("utf-8"),
                request_id=request_id,
            )

        # 4. Payment Approved (or Free Route): Prepare Upstream Forwarding
        settlement_id: str | None = None
        fulfillment_id: str | None = None

        if pay_result.verification:
            settlement_id = pay_result.verification.settlement_id
            fulfillment_id = pay_result.verification.fulfillment_id
            self.audit_logger.log_payment_verified(
                request_id=request_id,
                payer=pay_result.verification.verified_payer,
                settlement_id=settlement_id,
                fulfillment_id=fulfillment_id,
                idempotency_key=pay_result.idempotency_key,
            )
        elif pay_result.status == PaymentStatus.FREE_REPLAY:
            settlement_id = pay_result.settlement_id
            fulfillment_id = pay_result.fulfillment_id
            self.audit_logger.log_event(
                "free_replay_accepted",
                request_id=request_id,
                payer=pay_result.payer_address,
                settlement_id=settlement_id,
                fulfillment_id=fulfillment_id,
                idempotency_key=pay_result.idempotency_key,
            )

        # 5. Header Sanitization & Upstream Secret Injection
        sanitized_req_headers = sanitize_request_headers(
            client_headers=headers,
            upstream_headers=match.route.upstream_headers,
            client_ip=client_ip,
            client_host=client_host,
            scheme=scheme,
            payer_address=pay_result.payer_address,
            settlement_id=settlement_id,
            fulfillment_id=fulfillment_id,
            request_id=request_id,
        )

        secret_keys = set(match.route.upstream_headers.keys()) if match.route.upstream_headers else set()

        # 6. Start Fulfillment in State Machine
        if self.state_coordinator is not None and fulfillment_id:
            try:
                self.state_coordinator.start_fulfillment(fulfillment_id)
            except Exception:
                pass

        # 7. Dispatch to Upstream
        proxy_resp = self.forwarder.forward(
            method=method,
            url=match.upstream_url,
            headers=sanitized_req_headers,
            content=content,
            timeout_seconds=match.route.timeout_seconds,
            upstream_secret_keys=secret_keys,
        )

        # 8. Synchronize State Machine with Upstream Outcome
        if self.state_coordinator is not None and fulfillment_id:
            if not proxy_resp.is_error and proxy_resp.status_code < 500:
                # Upstream Succeeded: record delivered payload in idempotency cache
                try:
                    self.state_coordinator.complete_fulfillment(
                        fulfillment_id=fulfillment_id,
                        upstream_status_code=proxy_resp.status_code,
                        response_body_blob=proxy_resp.content,
                        response_headers_json=json.dumps(proxy_resp.headers),
                    )
                except Exception:
                    pass
                self.audit_logger.log_upstream_completed(
                    request_id=request_id,
                    upstream_url=match.upstream_url,
                    status_code=proxy_resp.status_code,
                    latency_ms=proxy_resp.latency_ms,
                    response_bytes=len(proxy_resp.content),
                    fulfillment_id=fulfillment_id,
                )
            else:
                # Upstream Failed: transition to RECOVERY_REQUIRED for free replay
                try:
                    self.state_coordinator.fail_fulfillment(
                        fulfillment_id=fulfillment_id,
                        upstream_status_code=proxy_resp.status_code,
                        error_message=proxy_resp.error_message,
                    )
                except Exception:
                    raise
                self.audit_logger.log_upstream_failed(
                    request_id=request_id,
                    upstream_url=match.upstream_url,
                    error=proxy_resp.error_message or f"Upstream error {proxy_resp.status_code}",
                    status_code=proxy_resp.status_code,
                    latency_ms=proxy_resp.latency_ms,
                    fulfillment_id=fulfillment_id,
                )
        else:
            if proxy_resp.is_error:
                self.audit_logger.log_upstream_failed(
                    request_id=request_id,
                    upstream_url=match.upstream_url,
                    error=proxy_resp.error_message or f"Upstream error {proxy_resp.status_code}",
                    status_code=proxy_resp.status_code,
                    latency_ms=proxy_resp.latency_ms,
                )
            else:
                self.audit_logger.log_upstream_completed(
                    request_id=request_id,
                    upstream_url=match.upstream_url,
                    status_code=proxy_resp.status_code,
                    latency_ms=proxy_resp.latency_ms,
                    response_bytes=len(proxy_resp.content),
                )

        # 9. Return Response to Downstream Client
        out_headers = dict(proxy_resp.headers)
        out_headers["X-Request-Id"] = request_id

        return GatewayResponse(
            status_code=proxy_resp.status_code,
            headers=out_headers,
            content=proxy_resp.content,
            request_id=request_id,
            latency_ms=proxy_resp.latency_ms,
        )

    def as_transport(self) -> httpx.BaseTransport:
        """Create a synchronous httpx transport directly connected to this gateway."""
        return GatewayTransport(self)

    def as_async_transport(self) -> httpx.AsyncBaseTransport:
        """Create an asynchronous httpx transport directly connected to this gateway."""
        return AsyncGatewayTransport(self)


class GatewayTransport(httpx.BaseTransport):
    """Synchronous in-process HTTP transport routing client traffic into GatewayProxyService."""

    def __init__(self, service: GatewayProxyService) -> None:
        super().__init__()
        self.service = service

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        query_str = request.url.query.decode("ascii") if request.url.query else None
        headers = dict(request.headers)
        body = request.read()
        res = self.service.handle_request(
            method=request.method,
            path=request.url.path,
            headers=headers,
            query_string=query_str,
            content=body,
            scheme=request.url.scheme,
            client_host=request.url.host,
        )
        return httpx.Response(
            status_code=res.status_code,
            headers=res.headers,
            content=res.content,
            request=request,
        )


class AsyncGatewayTransport(httpx.AsyncBaseTransport):
    """Asynchronous in-process HTTP transport routing client traffic into GatewayProxyService."""

    def __init__(self, service: GatewayProxyService) -> None:
        super().__init__()
        self.service = service

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        query_str = request.url.query.decode("ascii") if request.url.query else None
        headers = dict(request.headers)
        body = await request.aread()
        res = self.service.handle_request(
            method=request.method,
            path=request.url.path,
            headers=headers,
            query_string=query_str,
            content=body,
            scheme=request.url.scheme,
            client_host=request.url.host,
        )
        return httpx.Response(
            status_code=res.status_code,
            headers=res.headers,
            content=res.content,
            request=request,
        )
