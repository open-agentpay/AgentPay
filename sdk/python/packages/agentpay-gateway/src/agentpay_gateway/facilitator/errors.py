"""
Facilitator service and client exceptions.
"""

from __future__ import annotations


class FacilitatorError(Exception):
    """Base exception for all Facilitator operations."""


class FacilitatorConnectionError(FacilitatorError):
    """Raised when connection to Facilitator fails or times out."""


class FacilitatorAPIError(FacilitatorError):
    """Raised when Facilitator returns an unexpected HTTP error."""

    def __init__(self, status_code: int, message: str, detail: str | None = None) -> None:
        super().__init__(f"Facilitator error {status_code}: {message}")
        self.status_code = status_code
        self.message = message
        self.detail = detail


class InvalidRequestError(FacilitatorAPIError):
    """HTTP 400 Bad Request: Missing or invalid request parameters."""

    def __init__(self, message: str = "Invalid request payload", detail: str | None = None) -> None:
        super().__init__(400, message, detail)


class InvalidSignatureError(FacilitatorAPIError):
    """HTTP 401 Unauthorized: Cryptographic signature mismatch or unrecoverable."""

    def __init__(self, message: str = "Signature verification failed", detail: str | None = None) -> None:
        super().__init__(401, message, detail)


class NonceAlreadyUsedError(FacilitatorAPIError):
    """HTTP 409 Conflict: Nonce has already been submitted or consumed (replay attack)."""

    def __init__(self, message: str = "Nonce already used", detail: str | None = None) -> None:
        super().__init__(409, message, detail)


class ChallengeExpiredError(FacilitatorAPIError):
    """HTTP 410 Gone: Current timestamp is after validBefore."""

    def __init__(self, message: str = "Payment challenge expired", detail: str | None = None) -> None:
        super().__init__(410, message, detail)


class SettlementNotFoundError(FacilitatorAPIError):
    """HTTP 404 Not Found: Settlement record not found."""

    def __init__(self, message: str = "Settlement not found", detail: str | None = None) -> None:
        super().__init__(404, message, detail)
