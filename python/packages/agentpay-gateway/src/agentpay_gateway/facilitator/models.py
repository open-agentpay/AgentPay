"""
Facilitator Request and Response Pydantic Models.

Adheres strictly to docs/api-specs.md §2 and specs/payment-state-machine.md.
"""

from __future__ import annotations

import enum

from agentpay.core.models import PaymentAuthorization, PaymentChallenge
from pydantic import BaseModel, ConfigDict, Field


class SettlementState(str, enum.Enum):
    """Settlement states managed by the Facilitator (specs/payment-state-machine.md §3)."""

    NOT_STARTED = "NOT_STARTED"
    SUBMITTED = "SUBMITTED"
    PENDING = "PENDING"
    CONFIRMED = "CONFIRMED"
    FAILED = "FAILED"
    REORGED = "REORGED"


class VerifyRequest(BaseModel):
    """Payload for POST /v1/verify."""

    model_config = ConfigDict(frozen=True, populate_by_name=True)

    challenge: PaymentChallenge
    authorization: PaymentAuthorization


class VerifyResponse(BaseModel):
    """Response from POST /v1/verify (HTTP 200 OK)."""

    model_config = ConfigDict(frozen=True, populate_by_name=True)

    valid: bool = True
    verified_payer: str = Field(..., alias="verifiedPayer")
    verified_at: int = Field(..., alias="verifiedAt")
    verification_token: str = Field(..., alias="verificationToken")


class SettleRequest(BaseModel):
    """Payload for POST /v1/settle."""

    model_config = ConfigDict(frozen=True, populate_by_name=True)

    verification_token: str = Field(..., alias="verificationToken")
    authorization: PaymentAuthorization
    idempotency_key: str | None = Field(default=None, alias="idempotencyKey")


class SettleResponse(BaseModel):
    """Response from POST /v1/settle (HTTP 202 Accepted)."""

    model_config = ConfigDict(frozen=True, populate_by_name=True)

    settlement_id: str = Field(..., alias="settlementId")
    status: SettlementState = SettlementState.SUBMITTED
    queue_position: int | None = Field(default=None, alias="queuePosition")
    tx_hash: str | None = Field(default=None, alias="txHash")


class SettlementStatus(BaseModel):
    """Response from GET /v1/settle/{settlement_id}."""

    model_config = ConfigDict(frozen=True, populate_by_name=True)

    settlement_id: str = Field(..., alias="settlementId")
    status: SettlementState
    tx_hash: str | None = Field(default=None, alias="txHash")
    error: str | None = None
    created_at: int = Field(default=0, alias="createdAt")
    updated_at: int = Field(default=0, alias="updatedAt")
