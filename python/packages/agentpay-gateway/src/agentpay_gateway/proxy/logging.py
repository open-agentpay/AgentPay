"""
Structured JSON Logging and Audit Trail for AgentPay Gateway.

Provides:
- High-performance JSON-Lines audit logging
- Automatic redaction of sensitive credentials (API keys, authorization headers, private keys)
- Trace correlation via request_id, settlement_id, and fulfillment_id
- Synchronous or thread-safe file appending to audit_log_path
"""

from __future__ import annotations

import datetime
import json
import logging
from collections.abc import Mapping
from pathlib import Path
from typing import Any

logger = logging.getLogger("agentpay_gateway.audit")

SENSITIVE_HEADER_PATTERNS = frozenset(
    {
        "authorization",
        "x-internal-secret",
        "x-api-key",
        "api-key",
        "secret",
        "password",
        "private_key",
        "raw_signature",
    }
)


def redact_sensitive_headers(headers: Mapping[str, str]) -> dict[str, str]:
    """Redact known sensitive credentials in headers dictionary for safe logging."""
    redacted: dict[str, str] = {}
    for key, value in headers.items():
        k_lower = key.lower()
        if any(pattern in k_lower for pattern in SENSITIVE_HEADER_PATTERNS):
            if k_lower == "authorization" and value.lower().startswith("bearer "):
                redacted[key] = "Bearer [REDACTED]"
            else:
                redacted[key] = "[REDACTED]"
        else:
            redacted[key] = value
    return redacted


class StructuredAuditLogger:
    """
    Structured JSON audit event logger for proxy requests, payment challenges,
    verification clearing, and upstream responses.
    """

    def __init__(self, audit_log_path: str | Path | None = None) -> None:
        self.audit_log_path: Path | None = Path(audit_log_path) if audit_log_path else None
        if self.audit_log_path:
            self.audit_log_path.parent.mkdir(parents=True, exist_ok=True)

    def log_event(self, event_type: str, **fields: Any) -> dict[str, Any]:
        """
        Record a structured audit event. Emits to Python logger and appends to audit file.

        Args:
            event_type: Type of event (e.g. 'request_received', 'challenge_issued', 'upstream_forwarded').
            fields: Event metadata fields.

        Returns:
            The serialized event dictionary.
        """
        now = datetime.datetime.now(datetime.timezone.utc)
        record: dict[str, Any] = {
            "timestamp": now.isoformat(),
            "event": event_type,
            **fields,
        }

        # Redact headers if present
        if "headers" in record and isinstance(record["headers"], Mapping):
            record["headers"] = redact_sensitive_headers(record["headers"])

        json_line = json.dumps(record, ensure_ascii=False)

        # 1. Output to standard logging
        logger.info(json_line)

        # 2. Append to audit file if configured
        if self.audit_log_path:
            try:
                with open(self.audit_log_path, "a", encoding="utf-8") as f:
                    f.write(json_line + "\n")
            except OSError as exc:
                logger.error(f"Failed to append to audit log file '{self.audit_log_path}': {exc}")

        return record

    def log_request_received(
        self,
        request_id: str,
        method: str,
        path: str,
        client_ip: str | None = None,
        headers: Mapping[str, str] | None = None,
    ) -> dict[str, Any]:
        return self.log_event(
            "request_received",
            request_id=request_id,
            method=method,
            path=path,
            client_ip=client_ip,
            headers=headers or {},
        )

    def log_challenge_issued(
        self,
        request_id: str,
        path: str,
        challenge_id: str,
        price_base_units: int,
        payee: str,
    ) -> dict[str, Any]:
        return self.log_event(
            "challenge_issued",
            request_id=request_id,
            path=path,
            challenge_id=challenge_id,
            price_base_units=price_base_units,
            payee=payee,
        )

    def log_payment_verified(
        self,
        request_id: str,
        payer: str,
        settlement_id: str,
        fulfillment_id: str | None = None,
        idempotency_key: str | None = None,
    ) -> dict[str, Any]:
        return self.log_event(
            "payment_verified",
            request_id=request_id,
            payer=payer,
            settlement_id=settlement_id,
            fulfillment_id=fulfillment_id,
            idempotency_key=idempotency_key,
        )

    def log_upstream_completed(
        self,
        request_id: str,
        upstream_url: str,
        status_code: int,
        latency_ms: float,
        response_bytes: int = 0,
        fulfillment_id: str | None = None,
    ) -> dict[str, Any]:
        return self.log_event(
            "upstream_completed",
            request_id=request_id,
            upstream_url=upstream_url,
            status_code=status_code,
            latency_ms=round(latency_ms, 2),
            response_bytes=response_bytes,
            fulfillment_id=fulfillment_id,
        )

    def log_upstream_failed(
        self,
        request_id: str,
        upstream_url: str,
        error: str,
        status_code: int = 502,
        latency_ms: float = 0.0,
        fulfillment_id: str | None = None,
    ) -> dict[str, Any]:
        return self.log_event(
            "upstream_failed",
            request_id=request_id,
            upstream_url=upstream_url,
            error=error,
            status_code=status_code,
            latency_ms=round(latency_ms, 2),
            fulfillment_id=fulfillment_id,
        )
