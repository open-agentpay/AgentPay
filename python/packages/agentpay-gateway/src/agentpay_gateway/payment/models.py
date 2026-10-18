"""
Data models and state representations for Gateway payment evaluation.
"""

from __future__ import annotations

import enum

from agentpay.core.models import PaymentAuthorization, PaymentChallenge
from pydantic import BaseModel, ConfigDict

from agentpay_gateway.config.models import RouteConfig


class PaymentStatus(str, enum.Enum):
    """Result status of payment evaluation on an inbound request."""

    FREE = "FREE"  # Route price is 0 base units; no payment required
    CHALLENGE_REQUIRED = "CHALLENGE_REQUIRED"  # No authorization provided; issue HTTP 402
    VERIFIED = "VERIFIED"  # Payment authorization verified by Facilitator
    CACHE_HIT = "CACHE_HIT"  # Idempotent response already cached (free return)
    FREE_REPLAY = "FREE_REPLAY"  # Previous upstream failed, free retry permitted
    REJECTED = "REJECTED"  # Payment rejected (invalid signature, expired, nonce consumed)
    ERROR = "ERROR"  # Facilitator unavailable or internal processing error


class VerificationResult(BaseModel):
    """Successful payment verification result from Facilitator."""

    model_config = ConfigDict(frozen=True)

    verified_payer: str
    verification_token: str
    verified_at: int
    settlement_id: str
    idempotency_key: str
    route: RouteConfig
    challenge: PaymentChallenge
    authorization: PaymentAuthorization
    fulfillment_id: str | None = None


class SettlementTask(BaseModel):
    """Task queued for in-process asynchronous blockchain settlement."""

    model_config = ConfigDict(frozen=True)

    settlement_id: str
    verification_token: str
    authorization: PaymentAuthorization
    idempotency_key: str | None = None
    enqueued_at: int = 0
    retry_count: int = 0


class PaymentProcessResult(BaseModel):
    """Outcome of processing request payment headers."""

    model_config = ConfigDict(frozen=True)

    status: PaymentStatus
    http_status_code: int = 200
    challenge: PaymentChallenge | None = None
    challenge_header: str | None = None
    verification: VerificationResult | None = None
    payer_address: str | None = None
    idempotency_key: str | None = None
    settlement_id: str | None = None
    fulfillment_id: str | None = None
    error_message: str | None = None
    cached_status_code: int | None = None
    cached_headers_json: str | None = None
    cached_body_blob: bytes | None = None
