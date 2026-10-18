"""
Mock Facilitator Service implementation for AgentPay.

Provides reference in-memory implementation of the Facilitator clearing node:
  - POST /v1/verify: Cryptographic signature verification, nonce replay defense, and expiration check.
  - POST /v1/settle: Idempotent settlement queuing and nonce commitment.
  - GET  /v1/settle/{id}: Settlement status inspection.

Compatible with ASGI 3.0 for zero-overhead in-memory HTTP testing via httpx.ASGITransport.
"""

from __future__ import annotations

import json
import secrets
import time
import urllib.parse
from collections.abc import Callable
from typing import Any

import httpx
from agentpay.client.signer import LocalSigner

from agentpay_gateway.facilitator.errors import (
    ChallengeExpiredError,
    InvalidRequestError,
    InvalidSignatureError,
    NonceAlreadyUsedError,
    SettlementNotFoundError,
)
from agentpay_gateway.facilitator.models import (
    SettlementState,
    SettlementStatus,
    SettleRequest,
    SettleResponse,
    VerifyRequest,
    VerifyResponse,
)


class MockFacilitatorService:
    """
    Reference in-memory Facilitator implementation.

    Enforces:
      1. EIP-712/3009 cryptographic verification (via LocalSigner)
      2. P0-7: UNIQUE(network_id, asset_address, payer_address, nonce) replay defense
      3. Valid window enforcement (validAfter <= now <= validBefore)
      4. Idempotent settlement queuing
    """

    def __init__(self, *, clock: Callable[[], int] | None = None) -> None:
        self._clock = clock or (lambda: int(time.time()))
        # Set of used nonces: (network, asset, payer, nonce) in lowercase
        self._used_nonces: set[tuple[str, str, str, str]] = set()
        # Active verification tokens: vtok -> dict
        self._verification_tokens: dict[str, dict[str, Any]] = {}
        # Settlements: settlement_id -> SettlementStatus
        self._settlements: dict[str, SettlementStatus] = {}
        # Idempotency cache: (payer, idempotency_key) -> SettleResponse
        self._idempotency_cache: dict[tuple[str, str], SettleResponse] = {}

    def get_time(self) -> int:
        """Get current simulated or wall-clock time."""
        return self._clock()

    def verify(self, request: VerifyRequest) -> VerifyResponse:
        """
        Verify payment challenge and client authorization.

        Raises:
            NonceAlreadyUsedError: If nonce was already consumed.
            ChallengeExpiredError: If challenge has expired.
            InvalidRequestError: If validAfter is in the future.
            InvalidSignatureError: If cryptographic signature is invalid or signer mismatches.
        """
        ch = request.challenge
        auth = request.authorization
        now = self.get_time()

        # 1. Nonce Replay Check (P0-7)
        nonce_key = (
            ch.network.lower(),
            ch.asset.lower(),
            auth.payer.lower(),
            auth.nonce.lower(),
        )
        if nonce_key in self._used_nonces:
            raise NonceAlreadyUsedError(f"Nonce {auth.nonce} has already been used for payer {auth.payer}")

        # 2. Expiration and time window check
        if now > ch.valid_before:
            raise ChallengeExpiredError(f"Challenge expired: validBefore ({ch.valid_before}) < now ({now})")
        if now < ch.valid_after:
            raise InvalidRequestError(f"Challenge not yet valid: validAfter ({ch.valid_after}) > now ({now})")

        # 3. Challenge ID matching
        if auth.challenge_id != ch.challenge_id:
            raise InvalidRequestError(
                f"Authorization challengeId ({auth.challenge_id}) does not match challenge ({ch.challenge_id})"
            )

        # 4. Cryptographic signature check
        is_valid = LocalSigner.verify_authorization(
            auth,
            ch,
            expected_payer=auth.payer,
        )
        if not is_valid:
            raise InvalidSignatureError(f"Signature verification failed for payer {auth.payer}")

        # 5. Issue verification token
        vtok = f"vtok_{secrets.token_hex(16)}"
        self._verification_tokens[vtok] = {
            "challenge": ch,
            "authorization": auth,
            "nonce_key": nonce_key,
            "verified_at": now,
            "expires_at": ch.valid_before,
        }

        return VerifyResponse(
            valid=True,
            verifiedPayer=auth.payer,
            verifiedAt=now,
            verificationToken=vtok,
        )

    def settle(self, request: SettleRequest) -> SettleResponse:
        """
        Submit a verified payment authorization to the clearing queue.

        Raises:
            InvalidRequestError: If verification token is unknown or expired.
            NonceAlreadyUsedError: If nonce was already consumed.
        """
        now = self.get_time()
        auth = request.authorization
        vtok = request.verification_token

        # Check idempotency first
        if request.idempotency_key:
            idem_key = (auth.payer.lower(), request.idempotency_key)
            if idem_key in self._idempotency_cache:
                return self._idempotency_cache[idem_key]

        # Validate token
        record = self._verification_tokens.get(vtok)
        if not record:
            raise InvalidRequestError(f"Unknown or invalid verification token: {vtok}")

        if now > record["expires_at"]:
            raise ChallengeExpiredError("Verification token has expired")

        nonce_key = record["nonce_key"]
        if nonce_key in self._used_nonces:
            raise NonceAlreadyUsedError("Nonce has already been settled")

        # Mark nonce consumed
        self._used_nonces.add(nonce_key)

        settlement_id = f"stl_{secrets.token_hex(12)}"
        position = len(self._settlements) + 1

        tx_hash = f"0x{secrets.token_hex(32)}"
        response = SettleResponse(
            settlementId=settlement_id,
            status=SettlementState.SUBMITTED,
            queuePosition=position,
            txHash=tx_hash,
        )

        self._settlements[settlement_id] = SettlementStatus(
            settlementId=settlement_id,
            status=SettlementState.SUBMITTED,
            txHash=tx_hash,
            createdAt=now,
            updatedAt=now,
        )

        if request.idempotency_key:
            self._idempotency_cache[(auth.payer.lower(), request.idempotency_key)] = response

        return response

    def get_settlement(self, settlement_id: str) -> SettlementStatus:
        """Retrieve settlement status by ID."""
        record = self._settlements.get(settlement_id)
        if not record:
            raise SettlementNotFoundError(f"Settlement {settlement_id!r} not found")
        return record

    def advance_settlement(
        self,
        settlement_id: str,
        new_status: SettlementState,
        *,
        tx_hash: str | None = None,
        error: str | None = None,
    ) -> SettlementStatus:
        """Simulate blockchain progress (for testing state machine transitions)."""
        curr = self.get_settlement(settlement_id)
        updated = SettlementStatus(
            settlementId=settlement_id,
            status=new_status,
            txHash=tx_hash or curr.tx_hash,
            error=error or curr.error,
            createdAt=curr.created_at,
            updatedAt=self.get_time(),
        )
        self._settlements[settlement_id] = updated
        return updated

    def is_nonce_used(self, network: str, asset: str, payer: str, nonce: str) -> bool:
        """Check whether an EVM nonce has already been consumed or settled."""
        nonce_key = (
            network.lower(),
            asset.lower(),
            payer.lower(),
            nonce.lower(),
        )
        return nonce_key in self._used_nonces

    def create_sync_transport(self) -> MockFacilitatorTransport:
        """Create a synchronous in-memory httpx.BaseTransport for FacilitatorClient."""
        return MockFacilitatorTransport(self)

    # -----------------------------------------------------------------------
    # Native ASGI 3.0 Application Handler
    # -----------------------------------------------------------------------

    async def asgi_app(
        self,
        scope: dict[str, Any],
        receive: Callable[..., Any],
        send: Callable[..., Any],
    ) -> None:
        """Standard ASGI 3.0 handler for in-memory HTTP transports."""
        if scope["type"] != "http":
            return

        path = scope.get("path", "")
        method = scope.get("method", "GET").upper()

        async def send_response(status_code: int, body_dict: dict[str, Any]) -> None:
            body_bytes = json.dumps(body_dict).encode("utf-8")
            await send(
                {
                    "type": "http.response.start",
                    "status": status_code,
                    "headers": [
                        [b"content-type", b"application/json"],
                        [b"content-length", str(len(body_bytes)).encode("ascii")],
                    ],
                }
            )
            await send(
                {
                    "type": "http.response.body",
                    "body": body_bytes,
                }
            )

        # Read request body
        body = b""
        more_body = True
        while more_body:
            message = await receive()
            body += message.get("body", b"")
            more_body = message.get("more_body", False)

        try:
            # Route: POST /v1/verify
            if method == "POST" and path == "/v1/verify":
                data = json.loads(body.decode("utf-8")) if body else {}
                req = VerifyRequest.model_validate(data)
                res = self.verify(req)
                await send_response(200, res.model_dump(by_alias=True))
                return

            # Route: POST /v1/settle
            if method == "POST" and path == "/v1/settle":
                data = json.loads(body.decode("utf-8")) if body else {}
                req_s = SettleRequest.model_validate(data)
                res_s = self.settle(req_s)
                await send_response(202, res_s.model_dump(by_alias=True))
                return

            # Route: GET /v1/settle/{id}
            if method == "GET" and path.startswith("/v1/settle/"):
                settlement_id = path.removeprefix("/v1/settle/")
                res_st = self.get_settlement(settlement_id)
                await send_response(200, res_st.model_dump(by_alias=True))
                return

            # Route: GET /v1/nonce
            if method == "GET" and path in ("/v1/nonce", "/v1/nonces"):
                query_string = scope.get("query_string", b"").decode("utf-8")
                params = dict(urllib.parse.parse_qsl(query_string))
                used = self.is_nonce_used(
                    params.get("network", ""),
                    params.get("asset", ""),
                    params.get("payer", ""),
                    params.get("nonce", ""),
                )
                await send_response(200, {"isUsed": used})
                return

            # Route: GET /health or /v1/health
            if method == "GET" and path in ("/health", "/v1/health"):
                await send_response(200, {"status": "ok", "time": self.get_time()})
                return

            # 404 for unknown endpoints
            await send_response(404, {"error": "Endpoint not found", "path": path})

        except NonceAlreadyUsedError as exc:
            await send_response(409, {"error": "NONCE_ALREADY_USED", "message": exc.message})
        except ChallengeExpiredError as exc:
            await send_response(410, {"error": "CHALLENGE_EXPIRED", "message": exc.message})
        except InvalidSignatureError as exc:
            await send_response(401, {"error": "INVALID_SIGNATURE", "message": exc.message})
        except InvalidRequestError as exc:
            await send_response(400, {"error": "INVALID_REQUEST", "message": exc.message})
        except SettlementNotFoundError as exc:
            await send_response(404, {"error": "SETTLEMENT_NOT_FOUND", "message": exc.message})
        except Exception as exc:  # noqa: BLE001
            await send_response(500, {"error": "INTERNAL_SERVER_ERROR", "message": str(exc)})


class MockFacilitatorTransport(httpx.BaseTransport):
    """Synchronous in-memory HTTP transport wrapping MockFacilitatorService."""

    def __init__(self, service: MockFacilitatorService) -> None:
        super().__init__()
        self.service = service

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        path = request.url.path
        method = request.method.upper()

        def make_response(status_code: int, data: dict[str, Any]) -> httpx.Response:
            return httpx.Response(
                status_code=status_code,
                json=data,
                request=request,
            )

        try:
            body_bytes = request.content
            if method == "POST" and path == "/v1/verify":
                data = json.loads(body_bytes.decode("utf-8")) if body_bytes else {}
                req = VerifyRequest.model_validate(data)
                res = self.service.verify(req)
                return make_response(200, res.model_dump(by_alias=True))

            if method == "POST" and path == "/v1/settle":
                data = json.loads(body_bytes.decode("utf-8")) if body_bytes else {}
                req_s = SettleRequest.model_validate(data)
                res_s = self.service.settle(req_s)
                return make_response(202, res_s.model_dump(by_alias=True))

            if method == "GET" and path.startswith("/v1/settle/"):
                settlement_id = path.removeprefix("/v1/settle/")
                res_st = self.service.get_settlement(settlement_id)
                return make_response(200, res_st.model_dump(by_alias=True))

            if method == "GET" and path in ("/v1/nonce", "/v1/nonces"):
                network = request.url.params.get("network", "")
                asset = request.url.params.get("asset", "")
                payer = request.url.params.get("payer", "")
                nonce = request.url.params.get("nonce", "")
                used = self.service.is_nonce_used(network, asset, payer, nonce)
                return make_response(200, {"isUsed": used})

            if method == "GET" and path in ("/health", "/v1/health"):
                return make_response(200, {"status": "ok", "time": self.service.get_time()})

            return make_response(404, {"error": "Endpoint not found", "path": path})

        except NonceAlreadyUsedError as exc:
            return make_response(409, {"error": "NONCE_ALREADY_USED", "message": exc.message})
        except ChallengeExpiredError as exc:
            return make_response(410, {"error": "CHALLENGE_EXPIRED", "message": exc.message})
        except InvalidSignatureError as exc:
            return make_response(401, {"error": "INVALID_SIGNATURE", "message": exc.message})
        except InvalidRequestError as exc:
            return make_response(400, {"error": "INVALID_REQUEST", "message": exc.message})
        except SettlementNotFoundError as exc:
            return make_response(404, {"error": "SETTLEMENT_NOT_FOUND", "message": exc.message})
        except Exception as exc:  # noqa: BLE001
            return make_response(500, {"error": "INTERNAL_SERVER_ERROR", "message": str(exc)})
