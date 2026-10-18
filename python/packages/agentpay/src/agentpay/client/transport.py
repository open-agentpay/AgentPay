"""
httpx transparent 402 interceptor transports for AgentPay.

Implements the autonomous financial lifecycle:
  Intercept 402 -> Policy Evaluate & Reserve -> Sign Authorization -> Replay with PAYMENT-SIGNATURE -> Commit or Hold

Adheres strictly to CLAUDE.md safety rules:
  - Fail-closed evaluation
  - Pre-evaluation signing lock
  - No automatic release once signature is transmitted on the wire
"""

from __future__ import annotations

import time
from collections.abc import Callable

import httpx

from agentpay.client.interfaces import ISigner
from agentpay.core.codecs import (
    extract_challenge_from_headers,
    inject_authorization_to_headers,
)
from agentpay.core.models import PaymentAuthorization
from agentpay.policy.errors import PolicyErrorCode, PolicyViolationError
from agentpay.policy.interfaces import IPolicyEngine


def _clone_request_with_authorization(request: httpx.Request, authorization: PaymentAuthorization) -> httpx.Request:
    """Clone an httpx request and inject the PAYMENT-SIGNATURE header."""
    headers = dict(request.headers)
    inject_authorization_to_headers(headers, authorization)
    content = request.read()
    return httpx.Request(
        method=request.method,
        url=request.url,
        headers=headers,
        content=content,
        extensions=dict(request.extensions),
    )


class AgentPayTransport(httpx.BaseTransport):
    """
    Synchronous httpx transport interceptor for AgentPay.

    Transparently catches HTTP 402 Payment Required responses, evaluates the
    challenge against the local policy firewall, signs with EIP-712/3009,
    and replays the request with payment credentials.
    """

    def __init__(
        self,
        inner_transport: httpx.BaseTransport,
        signer: ISigner,
        policy: IPolicyEngine,
        *,
        auto_retry_on_402: bool = True,
        raise_on_policy_rejection: bool = False,
        clock: Callable[[], int] | None = None,
    ) -> None:
        super().__init__()
        self._inner_transport = inner_transport
        self._signer = signer
        self._policy = policy
        self._auto_retry = auto_retry_on_402
        self._raise_on_rejection = raise_on_policy_rejection
        self._clock = clock or getattr(policy, "_clock", None) or (lambda: int(time.time()))

    def close(self) -> None:
        self._inner_transport.close()

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        response = self._inner_transport.handle_request(request)

        # Only intercept 402 Payment Required if auto_retry is enabled
        if response.status_code != 402 or not self._auto_retry:
            return response

        # 1. Extract x402 challenge from PAYMENT-REQUIRED header
        challenge = extract_challenge_from_headers(response.headers)
        if challenge is None:
            # Not a compliant x402 challenge, pass through raw response
            return response

        # Check challenge validity window
        now = self._clock()
        if challenge.is_expired(now):
            return response

        # 2. Policy firewall evaluation & two-phase atomic reservation
        payment_request = challenge.to_payment_request(domain=request.url.host)
        decision = self._policy.reserve(
            intent_id=payment_request.intent_id,
            amount=payment_request.amount,
            payee=payment_request.payee,
            asset=payment_request.asset,
            network=payment_request.network,
            domain=payment_request.domain,
            ttl_seconds=payment_request.ttl_seconds,
        )

        if not decision.approved:
            if self._raise_on_rejection:
                err_code = decision.error_code or PolicyErrorCode.SYSTEM_LOCKED
                raise PolicyViolationError(
                    err_code,
                    f"Payment policy rejected 402 challenge: {err_code.value}",
                )
            return response

        assert decision.reservation is not None
        reservation = decision.reservation

        # 3. Cryptographic signing under firewall protection
        try:
            authorization = self._signer.sign_payment_authorization(challenge, reservation)
        except Exception:
            # Signature generation failed BEFORE transmission: safe to release quota
            self._policy.release(reservation.intent_id)
            raise

        # 4. Inject PAYMENT-SIGNATURE and replay request
        retry_request = _clone_request_with_authorization(request, authorization)

        # Transmission safety boundary: once sent on the wire, do NOT auto-release on failure.
        # If transport raises an error, it propagates out naturally while preserving RESERVED state.
        retry_response = self._inner_transport.handle_request(retry_request)

        # 5. Handle settlement decision
        if 200 <= retry_response.status_code < 300:
            # Payment accepted and resource delivered successfully
            self._policy.commit(reservation.intent_id)

        return retry_response


class AsyncAgentPayTransport(httpx.AsyncBaseTransport):
    """
    Asynchronous httpx transport interceptor for AgentPay.

    Async equivalent of AgentPayTransport for AsyncClient and ASGI applications.
    """

    def __init__(
        self,
        inner_transport: httpx.AsyncBaseTransport,
        signer: ISigner,
        policy: IPolicyEngine,
        *,
        auto_retry_on_402: bool = True,
        raise_on_policy_rejection: bool = False,
        clock: Callable[[], int] | None = None,
    ) -> None:
        super().__init__()
        self._inner_transport = inner_transport
        self._signer = signer
        self._policy = policy
        self._auto_retry = auto_retry_on_402
        self._raise_on_rejection = raise_on_policy_rejection
        if clock is not None:
            self._clock = clock
        else:
            policy_clock = getattr(policy, "_clock", None)
            if callable(policy_clock):
                self._clock = policy_clock
            else:
                self._clock = lambda: int(time.time())

    async def aclose(self) -> None:
        await self._inner_transport.aclose()

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        response = await self._inner_transport.handle_async_request(request)

        if response.status_code != 402 or not self._auto_retry:
            return response

        challenge = extract_challenge_from_headers(response.headers)
        if challenge is None:
            return response

        now = self._clock()
        if challenge.is_expired(now):
            return response

        payment_request = challenge.to_payment_request(domain=request.url.host)
        decision = self._policy.reserve(
            intent_id=payment_request.intent_id,
            amount=payment_request.amount,
            payee=payment_request.payee,
            asset=payment_request.asset,
            network=payment_request.network,
            domain=payment_request.domain,
            ttl_seconds=payment_request.ttl_seconds,
        )

        if not decision.approved:
            if self._raise_on_rejection:
                err_code = decision.error_code or PolicyErrorCode.SYSTEM_LOCKED
                raise PolicyViolationError(
                    err_code,
                    f"Payment policy rejected 402 challenge: {err_code.value}",
                )
            return response

        assert decision.reservation is not None
        reservation = decision.reservation

        try:
            authorization = self._signer.sign_payment_authorization(challenge, reservation)
        except Exception:
            self._policy.release(reservation.intent_id)
            raise

        retry_request = _clone_request_with_authorization(request, authorization)
        retry_response = await self._inner_transport.handle_async_request(retry_request)

        if 200 <= retry_response.status_code < 300:
            self._policy.commit(reservation.intent_id)

        return retry_response
