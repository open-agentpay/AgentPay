"""
Production ASGI Server for AgentPay Reverse Proxy Gateway.

Launches via:
  uvicorn agentpay_gateway.server:app --host 0.0.0.0 --port 8000

Environment variables:
  - GATEWAY_CONFIG: Path to gateway.yaml configuration file
  - FACILITATOR_URL: URL to Facilitator clearing service (e.g. http://facilitator:8001)
"""

from __future__ import annotations

import logging
import os
from collections.abc import Callable
from pathlib import Path
from typing import Any

from agentpay_gateway.config.loader import load_gateway_config
from agentpay_gateway.config.models import FacilitatorConfig
from agentpay_gateway.config.router import RouteMatcher
from agentpay_gateway.facilitator.client import FacilitatorClient
from agentpay_gateway.payment.challenge import ChallengeManager
from agentpay_gateway.payment.verifier import PaymentVerifier
from agentpay_gateway.proxy.forwarder import ProxyForwarder
from agentpay_gateway.proxy.logging import StructuredAuditLogger
from agentpay_gateway.proxy.service import GatewayProxyService
from agentpay_gateway.state.coordinator import GatewayStateCoordinator
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database

logger = logging.getLogger("agentpay_gateway.server")


def create_gateway_service() -> GatewayProxyService:
    """Build and wire GatewayProxyService from environment configuration."""
    config_path = os.environ.get("GATEWAY_CONFIG", "gateway.yaml")
    if not Path(config_path).is_file():
        # Check fallback example paths
        for fallback in [
            "examples/paid-api/gateway-arbitrum-sepolia.yaml",
            "examples/paid-api/gateway.yaml",
        ]:
            if Path(fallback).is_file():
                config_path = fallback
                break

    logger.info(f"Loading AgentPay Gateway configuration from: {config_path}")
    config = load_gateway_config(config_path)

    # State Coordinator
    db_conn = create_gateway_connection(config.database.path)
    init_gateway_database(db_conn)
    state_coordinator = GatewayStateCoordinator(conn=db_conn)

    # Route Matcher
    route_matcher = RouteMatcher(config.routes)

    # Facilitator Client
    facilitator_url = os.environ.get("FACILITATOR_URL", config.facilitator.url)
    facilitator_cfg = FacilitatorConfig(url=facilitator_url, timeout_seconds=config.facilitator.timeout_seconds)
    facilitator_client = FacilitatorClient(base_url=facilitator_url, timeout=config.facilitator.timeout_seconds)

    # Challenge Manager & Payment Verifier
    challenge_manager = ChallengeManager(
        facilitator_config=facilitator_cfg,
        settlement_defaults=config.defaults,
    )
    payment_verifier = PaymentVerifier(
        challenge_manager=challenge_manager,
        state_coordinator=state_coordinator,
        facilitator_client=facilitator_client,
    )

    # Forwarder & Audit Logger
    forwarder = ProxyForwarder(timeout_seconds=config.server.timeout_seconds)
    audit_logger = StructuredAuditLogger()

    return GatewayProxyService(
        route_matcher=route_matcher,
        payment_verifier=payment_verifier,
        forwarder=forwarder,
        audit_logger=audit_logger,
        state_coordinator=state_coordinator,
    )


# Default proxy service instance
gateway_service = create_gateway_service()


async def app(
    scope: dict[str, Any],
    receive: Callable[..., Any],
    send: Callable[..., Any],
) -> None:
    """Standard ASGI 3.0 gateway application."""
    if scope["type"] != "http":
        return

    path = scope.get("path", "/")
    method = scope.get("method", "GET").upper()
    query_string = scope.get("query_string", b"").decode("utf-8")
    client = scope.get("client")
    client_ip = client[0] if client else None

    # Collect headers
    headers: dict[str, str] = {}
    for raw_name, raw_value in scope.get("headers", []):
        name = raw_name.decode("latin1").lower()
        val = raw_value.decode("latin1")
        headers[name] = val

    # Read body
    body = b""
    more_body = True
    while more_body:
        msg = await receive()
        body += msg.get("body", b"")
        more_body = msg.get("more_body", False)

    # Dispatch to GatewayProxyService
    res = gateway_service.handle_request(
        method=method,
        path=path,
        headers=headers,
        query_string=query_string if query_string else None,
        content=body,
        client_ip=client_ip,
        scheme=scope.get("scheme", "http"),
    )

    # Send response start
    resp_headers = []
    for k, v in res.headers.items():
        resp_headers.append([k.encode("latin1"), v.encode("latin1")])

    await send(
        {
            "type": "http.response.start",
            "status": res.status_code,
            "headers": resp_headers,
        }
    )

    # Send response body
    await send(
        {
            "type": "http.response.body",
            "body": res.content,
        }
    )
