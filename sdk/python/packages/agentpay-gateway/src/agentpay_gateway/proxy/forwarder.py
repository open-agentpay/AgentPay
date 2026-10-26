"""
Upstream HTTP Dispatcher & Reverse Proxy Forwarder.

Enforces:
- Anti-SSRF guardrail: follow_redirects=False (RFC-compliant proxying)
- Upstream timeout isolation (per-route override)
- Connection error and timeout mapping to 502 Bad Gateway / 504 Gateway Timeout
- Sub-millisecond latency tracking
"""

from __future__ import annotations

import time
from collections.abc import Mapping

import httpx
from pydantic import BaseModel, ConfigDict

from agentpay_gateway.proxy.headers import sanitize_response_headers


class ProxyResponse(BaseModel):
    """Result of forwarding an HTTP request to an upstream service."""

    model_config = ConfigDict(frozen=True)

    status_code: int
    headers: dict[str, str]
    content: bytes
    latency_ms: float
    is_error: bool = False
    error_message: str | None = None


class ProxyForwarder:
    """
    Dispatches sanitized HTTP requests to upstream services.
    Supports both synchronous and asynchronous execution.
    """

    def __init__(
        self,
        *,
        client: httpx.Client | None = None,
        async_client: httpx.AsyncClient | None = None,
        transport: httpx.BaseTransport | None = None,
        async_transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self._sync_client = client or httpx.Client(
            transport=transport,
            trust_env=False,  # Internal gateway: ignore external environment SOCKS/HTTP proxies
            follow_redirects=False,  # Anti-SSRF: Never follow upstream redirects automatically
        )
        self._async_client = async_client or httpx.AsyncClient(
            transport=async_transport,
            trust_env=False,
            follow_redirects=False,
        )

    def close(self) -> None:
        """Close underlying HTTP clients."""
        self._sync_client.close()

    async def aclose(self) -> None:
        """Close underlying async HTTP client."""
        await self._async_client.aclose()

    def forward(
        self,
        method: str,
        url: str,
        headers: Mapping[str, str],
        content: bytes | None = None,
        *,
        timeout_seconds: float = 10.0,
        upstream_secret_keys: set[str] | None = None,
    ) -> ProxyResponse:
        """
        Synchronously dispatch request to upstream service.

        Args:
            method: HTTP method (e.g. 'GET', 'POST').
            url: Fully resolved upstream target URL.
            headers: Sanitized headers including upstream credentials.
            content: Raw request payload body.
            timeout_seconds: Upstream timeout cutoff.
            upstream_secret_keys: Keys to sanitize from upstream response headers.

        Returns:
            ProxyResponse with status code, sanitized headers, body, and latency.
        """
        start = time.perf_counter()
        try:
            resp = self._sync_client.request(
                method=method,
                url=url,
                headers=dict(headers),
                content=content,
                timeout=timeout_seconds,
            )
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            cleaned_headers = sanitize_response_headers(resp.headers, upstream_secret_keys)
            is_5xx = resp.status_code >= 500

            return ProxyResponse(
                status_code=resp.status_code,
                headers=cleaned_headers,
                content=resp.content,
                latency_ms=elapsed_ms,
                is_error=is_5xx,
            )
        except httpx.ConnectError as exc:
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            return ProxyResponse(
                status_code=502,
                headers={"Content-Type": "application/json"},
                content=b'{"error":"bad_gateway","message":"Upstream service connection failed"}',
                latency_ms=elapsed_ms,
                is_error=True,
                error_message=f"Upstream connection failed: {exc}",
            )
        except httpx.TimeoutException as exc:
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            return ProxyResponse(
                status_code=504,
                headers={"Content-Type": "application/json"},
                content=b'{"error":"gateway_timeout","message":"Upstream service timed out"}',
                latency_ms=elapsed_ms,
                is_error=True,
                error_message=f"Upstream request timed out: {exc}",
            )
        except Exception as exc:
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            return ProxyResponse(
                status_code=502,
                headers={"Content-Type": "application/json"},
                content=b'{"error":"bad_gateway","message":"Upstream proxy dispatch error"}',
                latency_ms=elapsed_ms,
                is_error=True,
                error_message=f"Proxy error: {exc}",
            )

    async def async_forward(
        self,
        method: str,
        url: str,
        headers: Mapping[str, str],
        content: bytes | None = None,
        *,
        timeout_seconds: float = 10.0,
        upstream_secret_keys: set[str] | None = None,
    ) -> ProxyResponse:
        """
        Asynchronously dispatch request to upstream service.
        """
        start = time.perf_counter()
        try:
            resp = await self._async_client.request(
                method=method,
                url=url,
                headers=dict(headers),
                content=content,
                timeout=timeout_seconds,
            )
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            cleaned_headers = sanitize_response_headers(resp.headers, upstream_secret_keys)
            is_5xx = resp.status_code >= 500

            return ProxyResponse(
                status_code=resp.status_code,
                headers=cleaned_headers,
                content=resp.content,
                latency_ms=elapsed_ms,
                is_error=is_5xx,
            )
        except httpx.ConnectError as exc:
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            return ProxyResponse(
                status_code=502,
                headers={"Content-Type": "application/json"},
                content=b'{"error":"bad_gateway","message":"Upstream service connection failed"}',
                latency_ms=elapsed_ms,
                is_error=True,
                error_message=f"Upstream connection failed: {exc}",
            )
        except httpx.TimeoutException as exc:
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            return ProxyResponse(
                status_code=504,
                headers={"Content-Type": "application/json"},
                content=b'{"error":"gateway_timeout","message":"Upstream service timed out"}',
                latency_ms=elapsed_ms,
                is_error=True,
                error_message=f"Upstream request timed out: {exc}",
            )
        except Exception as exc:
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            return ProxyResponse(
                status_code=502,
                headers={"Content-Type": "application/json"},
                content=b'{"error":"bad_gateway","message":"Upstream proxy dispatch error"}',
                latency_ms=elapsed_ms,
                is_error=True,
                error_message=f"Proxy error: {exc}",
            )
