"""
AgentPay Gateway Reverse Proxy & Forwarding Engine.

Provides:
- Hop-by-hop and payment protocol header sanitization
- Upstream credential injection and isolation
- Anti-SSRF reverse proxy dispatch
- Structured JSON-Lines audit logging
- Integrated GatewayProxyService request handler
"""

from __future__ import annotations

from agentpay_gateway.proxy.forwarder import ProxyForwarder, ProxyResponse
from agentpay_gateway.proxy.headers import (
    HOP_BY_HOP_HEADERS,
    PAYMENT_PROTOCOL_HEADERS,
    sanitize_request_headers,
    sanitize_response_headers,
)
from agentpay_gateway.proxy.logging import (
    StructuredAuditLogger,
    redact_sensitive_headers,
)
from agentpay_gateway.proxy.service import (
    AsyncGatewayTransport,
    GatewayProxyService,
    GatewayResponse,
    GatewayTransport,
)

__all__ = [
    # Headers
    "HOP_BY_HOP_HEADERS",
    "PAYMENT_PROTOCOL_HEADERS",
    "sanitize_request_headers",
    "sanitize_response_headers",
    # Logging
    "StructuredAuditLogger",
    "redact_sensitive_headers",
    # Forwarder
    "ProxyForwarder",
    "ProxyResponse",
    # Service
    "GatewayProxyService",
    "GatewayResponse",
    "GatewayTransport",
    "AsyncGatewayTransport",
]
