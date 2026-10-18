"""
Gateway Payment & Verification Exceptions.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

from agentpay_gateway.config.errors import GatewayConfigError

if TYPE_CHECKING:
    from agentpay.core.models import PaymentChallenge


class PaymentGatewayError(GatewayConfigError):
    """Base exception for all Gateway payment processing errors."""


class MissingPaymentError(PaymentGatewayError):
    """Raised when an inbound request requires payment but provides no authorization header."""

    def __init__(
        self,
        message: str = "Payment required to access this resource",
        *,
        challenge: PaymentChallenge | None = None,
    ) -> None:
        super().__init__(message)
        self.challenge = challenge


class InvalidAuthorizationHeaderError(PaymentGatewayError):
    """Raised when the authorization header is malformed, corrupted, or unparseable."""


class ChallengeMismatchError(PaymentGatewayError):
    """Raised when the authorization does not match the active challenge or route pricing."""


class ChallengeExpiredError(PaymentGatewayError):
    """Raised when the payment challenge or authorization window has expired."""


class PaymentRejectedError(PaymentGatewayError):
    """Raised when the Facilitator explicitly rejects the payment authorization."""

    def __init__(
        self,
        message: str,
        *,
        error_code: str | None = None,
        detail: str | None = None,
    ) -> None:
        super().__init__(message)
        self.error_code = error_code
        self.detail = detail


class FacilitatorUnavailableError(PaymentGatewayError):
    """Raised when the Facilitator clearing service cannot be reached or fails with 5xx."""


class IdempotencyConflictError(PaymentGatewayError):
    """Raised when an identical request is currently in-flight with an active lease."""

    def __init__(
        self,
        message: str = "Concurrent request with the same idempotency key is currently processing",
    ) -> None:
        super().__init__(message)
