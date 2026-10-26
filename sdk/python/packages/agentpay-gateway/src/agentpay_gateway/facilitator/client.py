"""
HTTP Client for Facilitator clearing services.

Supports both synchronous and asynchronous modes, with optional in-memory ASGITransport.
"""

from __future__ import annotations

import json
from typing import Any

import httpx
from agentpay.core.models import PaymentAuthorization, PaymentChallenge

from agentpay_gateway.facilitator.errors import (
    ChallengeExpiredError,
    FacilitatorAPIError,
    FacilitatorConnectionError,
    InvalidRequestError,
    InvalidSignatureError,
    NonceAlreadyUsedError,
    SettlementNotFoundError,
)
from agentpay_gateway.facilitator.models import (
    SettlementStatus,
    SettleRequest,
    SettleResponse,
    VerifyRequest,
    VerifyResponse,
)


def _handle_error_response(response: httpx.Response) -> None:
    """Map HTTP error status codes to explicit Facilitator exceptions."""
    try:
        body = response.json()
        message = body.get("message") or body.get("error") or response.text
    except (ValueError, TypeError, json.JSONDecodeError):
        body = {}
        message = response.text or f"HTTP {response.status_code}"

    status = response.status_code
    if status == 400:
        raise InvalidRequestError(message=message, detail=str(body))
    elif status == 401:
        raise InvalidSignatureError(message=message, detail=str(body))
    elif status == 409:
        raise NonceAlreadyUsedError(message=message, detail=str(body))
    elif status == 410:
        raise ChallengeExpiredError(message=message, detail=str(body))
    elif status == 404:
        raise SettlementNotFoundError(message=message, detail=str(body))
    else:
        raise FacilitatorAPIError(status_code=status, message=message, detail=str(body))


class FacilitatorClient:
    """Synchronous HTTP client for Facilitator clearing service."""

    def __init__(
        self,
        base_url: str = "http://localhost:8000",
        *,
        transport: httpx.BaseTransport | None = None,
        timeout: float = 10.0,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self._client = httpx.Client(
            base_url=self.base_url,
            transport=transport,
            timeout=timeout,
        )

    def close(self) -> None:
        """Close the underlying HTTP client session."""
        self._client.close()

    def __enter__(self) -> FacilitatorClient:
        return self

    def __exit__(self, *args: object) -> None:
        self.close()

    def verify(
        self,
        challenge: PaymentChallenge,
        authorization: PaymentAuthorization,
    ) -> VerifyResponse:
        """
        Verify payment authorization against the challenge via POST /v1/verify.

        Returns:
            VerifyResponse with valid=True and verificationToken.

        Raises:
            InvalidSignatureError: (401) Signature invalid or signer mismatch.
            NonceAlreadyUsedError: (409) Nonce already consumed.
            ChallengeExpiredError: (410) Challenge has expired.
            FacilitatorConnectionError: Network failure or timeout.
        """
        req = VerifyRequest(challenge=challenge, authorization=authorization)
        try:
            resp = self._client.post(
                "/v1/verify",
                json=req.model_dump(by_alias=True),
            )
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise FacilitatorConnectionError(f"Failed to connect to Facilitator: {exc}") from exc

        if resp.is_error:
            _handle_error_response(resp)

        return VerifyResponse.model_validate(resp.json())

    def settle(
        self,
        authorization: PaymentAuthorization,
        verification_token: str,
        *,
        idempotency_key: str | None = None,
    ) -> SettleResponse:
        """
        Submit authorization for blockchain settlement via POST /v1/settle.

        Returns:
            SettleResponse with settlementId and queue status.
        """
        req = SettleRequest(
            verificationToken=verification_token,
            authorization=authorization,
            idempotencyKey=idempotency_key,
        )
        try:
            resp = self._client.post(
                "/v1/settle",
                json=req.model_dump(by_alias=True),
            )
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise FacilitatorConnectionError(f"Failed to connect to Facilitator: {exc}") from exc

        if resp.is_error:
            _handle_error_response(resp)

        return SettleResponse.model_validate(resp.json())

    def get_settlement(self, settlement_id: str) -> SettlementStatus:
        """Retrieve settlement status via GET /v1/settle/{id}."""
        try:
            resp = self._client.get(f"/v1/settle/{settlement_id}")
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise FacilitatorConnectionError(f"Failed to connect to Facilitator: {exc}") from exc

        if resp.is_error:
            _handle_error_response(resp)

        return SettlementStatus.model_validate(resp.json())

    def is_nonce_used(
        self,
        network: str,
        asset: str,
        payer: str,
        nonce: str,
    ) -> bool:
        """Query whether an EVM authorization nonce has already been consumed."""
        try:
            resp = self._client.get(
                "/v1/nonce",
                params={
                    "network": network,
                    "asset": asset,
                    "payer": payer,
                    "nonce": nonce,
                },
            )
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise FacilitatorConnectionError(f"Failed to connect to Facilitator: {exc}") from exc

        if resp.is_error:
            _handle_error_response(resp)

        data = resp.json()
        return bool(data.get("isUsed", False))

    def health(self) -> dict[str, Any]:
        """Check Facilitator service health via GET /health."""
        try:
            resp = self._client.get("/health")
            resp.raise_for_status()
            return resp.json()
        except Exception as exc:
            raise FacilitatorConnectionError(f"Health check failed: {exc}") from exc


class AsyncFacilitatorClient:
    """Asynchronous HTTP client for Facilitator clearing service."""

    def __init__(
        self,
        base_url: str = "http://localhost:8000",
        *,
        transport: httpx.AsyncBaseTransport | None = None,
        timeout: float = 10.0,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self._client = httpx.AsyncClient(
            base_url=self.base_url,
            transport=transport,
            timeout=timeout,
        )

    async def close(self) -> None:
        """Close the underlying HTTP client session."""
        await self._client.aclose()

    async def __aenter__(self) -> AsyncFacilitatorClient:
        return self

    async def __aexit__(self, *args: object) -> None:
        await self.close()

    async def verify(
        self,
        challenge: PaymentChallenge,
        authorization: PaymentAuthorization,
    ) -> VerifyResponse:
        """Verify payment authorization asynchronously."""
        req = VerifyRequest(challenge=challenge, authorization=authorization)
        try:
            resp = await self._client.post(
                "/v1/verify",
                json=req.model_dump(by_alias=True),
            )
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise FacilitatorConnectionError(f"Failed to connect to Facilitator: {exc}") from exc

        if resp.is_error:
            _handle_error_response(resp)

        return VerifyResponse.model_validate(resp.json())

    async def settle(
        self,
        authorization: PaymentAuthorization,
        verification_token: str,
        *,
        idempotency_key: str | None = None,
    ) -> SettleResponse:
        """Submit settlement asynchronously."""
        req = SettleRequest(
            verificationToken=verification_token,
            authorization=authorization,
            idempotencyKey=idempotency_key,
        )
        try:
            resp = await self._client.post(
                "/v1/settle",
                json=req.model_dump(by_alias=True),
            )
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise FacilitatorConnectionError(f"Failed to connect to Facilitator: {exc}") from exc

        if resp.is_error:
            _handle_error_response(resp)

        return SettleResponse.model_validate(resp.json())

    async def get_settlement(self, settlement_id: str) -> SettlementStatus:
        """Retrieve settlement status asynchronously."""
        try:
            resp = await self._client.get(f"/v1/settle/{settlement_id}")
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise FacilitatorConnectionError(f"Failed to connect to Facilitator: {exc}") from exc

        if resp.is_error:
            _handle_error_response(resp)

        return SettlementStatus.model_validate(resp.json())

    async def is_nonce_used(
        self,
        network: str,
        asset: str,
        payer: str,
        nonce: str,
    ) -> bool:
        """Query whether an EVM authorization nonce has already been consumed asynchronously."""
        try:
            resp = await self._client.get(
                "/v1/nonce",
                params={
                    "network": network,
                    "asset": asset,
                    "payer": payer,
                    "nonce": nonce,
                },
            )
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise FacilitatorConnectionError(f"Failed to connect to Facilitator: {exc}") from exc

        if resp.is_error:
            _handle_error_response(resp)

        data = resp.json()
        return bool(data.get("isUsed", False))
