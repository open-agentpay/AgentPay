"""
Header Sanitization and Upstream Secret Isolation for AgentPay Gateway.

Enforces:
- RFC 7230 Hop-by-hop header removal (Connection, Keep-Alive, TE, etc.)
- Protocol header isolation (PAYMENT-SIGNATURE, PAYMENT-REQUIRED stripped before forwarding)
- Upstream Secret Isolation: Injects configured upstream API keys and secrets, preventing client tampering
- Tracing Context Injection: X-Forwarded-For, X-Forwarded-Proto, X-AgentPay-Payer, etc.
- Safe Response Header Sanitization: Strips internal echo tokens before returning to client
"""

from __future__ import annotations

from collections.abc import Mapping

HOP_BY_HOP_HEADERS: frozenset[str] = frozenset(
    {
        "connection",
        "keep-alive",
        "proxy-authenticate",
        "proxy-authorization",
        "te",
        "trailer",
        "trailers",
        "transfer-encoding",
        "upgrade",
        "proxy-connection",
        "content-length",
        "host",
    }
)

PAYMENT_PROTOCOL_HEADERS: frozenset[str] = frozenset(
    {
        "payment-required",
        "payment-signature",
        "payment-authorization",
        "x-payment-signature",
        "x-payment-authorization",
    }
)


def sanitize_request_headers(
    client_headers: Mapping[str, str],
    upstream_headers: Mapping[str, str] | None = None,
    *,
    client_ip: str | None = None,
    client_host: str | None = None,
    scheme: str = "http",
    payer_address: str | None = None,
    settlement_id: str | None = None,
    fulfillment_id: str | None = None,
    request_id: str | None = None,
) -> dict[str, str]:
    """
    Sanitize inbound client headers and securely inject upstream credentials and context.

    Args:
        client_headers: Raw headers received from client.
        upstream_headers: Static upstream headers/secrets configured in RouteConfig.
        client_ip: Inbound client IP address.
        client_host: Original Host header requested by client.
        scheme: Inbound HTTP scheme ('http' or 'https').
        payer_address: Verified on-chain payer address.
        settlement_id: Unique settlement ID.
        fulfillment_id: Unique fulfillment ID.
        request_id: Tracing request ID.

    Returns:
        Sanitized headers dictionary ready for upstream dispatch.
    """
    cleaned: dict[str, str] = {}

    # Step 1: Strip hop-by-hop and payment protocol headers
    for key, value in client_headers.items():
        k_lower = key.strip().lower()
        if k_lower in HOP_BY_HOP_HEADERS or k_lower in PAYMENT_PROTOCOL_HEADERS:
            continue
        cleaned[key.strip()] = value

    # Step 2: Inject Tracing & Proxy Context headers
    if client_ip:
        existing_xff = cleaned.get("X-Forwarded-For")
        cleaned["X-Forwarded-For"] = f"{existing_xff}, {client_ip}" if existing_xff else client_ip

    cleaned["X-Forwarded-Proto"] = scheme
    if client_host:
        cleaned["X-Forwarded-Host"] = client_host

    if request_id:
        cleaned["X-Request-Id"] = request_id

    # Step 3: Inject verified payment credentials into context
    if payer_address:
        cleaned["X-AgentPay-Payer"] = payer_address.lower()
    if settlement_id:
        cleaned["X-AgentPay-Settlement-Id"] = settlement_id
    if fulfillment_id:
        cleaned["X-AgentPay-Fulfillment-Id"] = fulfillment_id

    # Step 4: Upstream Secret Isolation (Priority Overwrite)
    # Guaranteed: Upstream secrets configured in RouteConfig ALWAYS override client-provided headers
    if upstream_headers:
        for secret_key, secret_val in upstream_headers.items():
            # Remove any case-insensitive collision from client first
            matches = [k for k in cleaned if k.lower() == secret_key.lower()]
            for m in matches:
                del cleaned[m]
            cleaned[secret_key] = secret_val

    return cleaned


def sanitize_response_headers(
    upstream_response_headers: Mapping[str, str],
    upstream_secret_keys: set[str] | None = None,
) -> dict[str, str]:
    """
    Sanitize upstream response headers before returning them to the downstream client.

    Args:
        upstream_response_headers: Raw headers received from upstream.
        upstream_secret_keys: Header names that carry internal secrets and must not leak.

    Returns:
        Sanitized headers dictionary for downstream response.
    """
    secret_lower = {k.lower() for k in (upstream_secret_keys or set())}
    cleaned: dict[str, str] = {}

    for key, value in upstream_response_headers.items():
        k_lower = key.strip().lower()
        if k_lower in HOP_BY_HOP_HEADERS:
            continue
        if k_lower in secret_lower:
            continue
        cleaned[key.strip()] = value

    return cleaned
