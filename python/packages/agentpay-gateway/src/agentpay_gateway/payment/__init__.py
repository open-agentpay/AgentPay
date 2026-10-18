"""
AgentPay Gateway Payment and Facilitator Verification Module.

Provides:
- x402 V2 Challenge generation, caching, and serialization (ChallengeManager)
- Payment authorization verification via Facilitator (PaymentVerifier)
- Idempotency inspection and settlement state coordination
- Standardized payment errors and evaluation results
"""

from __future__ import annotations

from agentpay_gateway.payment.challenge import ChallengeManager
from agentpay_gateway.payment.errors import (
    ChallengeExpiredError,
    ChallengeMismatchError,
    FacilitatorUnavailableError,
    IdempotencyConflictError,
    InvalidAuthorizationHeaderError,
    MissingPaymentError,
    PaymentGatewayError,
    PaymentRejectedError,
)
from agentpay_gateway.payment.models import (
    PaymentProcessResult,
    PaymentStatus,
    SettlementTask,
    VerificationResult,
)
from agentpay_gateway.payment.settlement import AsyncSettlementWorker
from agentpay_gateway.payment.verifier import PaymentVerifier

__all__ = [
    # Challenge Engine
    "ChallengeManager",
    # Verification Engine
    "PaymentVerifier",
    # Settlement Worker
    "AsyncSettlementWorker",
    # Models
    "PaymentStatus",
    "SettlementTask",
    "VerificationResult",
    "PaymentProcessResult",
    # Errors
    "PaymentGatewayError",
    "MissingPaymentError",
    "InvalidAuthorizationHeaderError",
    "ChallengeMismatchError",
    "ChallengeExpiredError",
    "PaymentRejectedError",
    "FacilitatorUnavailableError",
    "IdempotencyConflictError",
]
