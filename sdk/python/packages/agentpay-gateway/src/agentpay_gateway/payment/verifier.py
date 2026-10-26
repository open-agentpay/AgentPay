"""
Payment Verification Engine and Facilitator Coordination for AgentPay Gateway.
"""

from __future__ import annotations

import inspect
import uuid
from collections.abc import Callable, Mapping
from typing import Any

from agentpay.core.codecs import (
    HEADER_PAYMENT_SIGNATURE,
    decode_authorization,
    extract_authorization_from_headers,
)
from agentpay.core.models import PaymentAuthorization, PaymentChallenge
from agentpay.core.state import GatewaySettlementRecord, SettlementState

from agentpay_gateway.config.models import RouteConfig
from agentpay_gateway.facilitator.client import FacilitatorClient
from agentpay_gateway.facilitator.errors import (
    ChallengeExpiredError as FacChallengeExpiredError,
)
from agentpay_gateway.facilitator.errors import (
    FacilitatorConnectionError,
    InvalidSignatureError,
    NonceAlreadyUsedError,
)
from agentpay_gateway.facilitator.models import VerifyRequest, VerifyResponse
from agentpay_gateway.payment.challenge import ChallengeManager
from agentpay_gateway.payment.errors import (
    ChallengeExpiredError,
    ChallengeMismatchError,
    FacilitatorUnavailableError,
    InvalidAuthorizationHeaderError,
    PaymentRejectedError,
)
from agentpay_gateway.payment.models import (
    PaymentProcessResult,
    PaymentStatus,
    SettlementTask,
    VerificationResult,
)
from agentpay_gateway.state.coordinator import GatewayStateCoordinator

AUTH_HEADER_ALIASES = [
    HEADER_PAYMENT_SIGNATURE,
    "PAYMENT-SIGNATURE",
    "PAYMENT-AUTHORIZATION",
    "X-PAYMENT-SIGNATURE",
    "X-PAYMENT-AUTHORIZATION",
]

IDEMPOTENCY_HEADER_ALIASES = [
    "IDEMPOTENCY-KEY",
    "X-IDEMPOTENCY-KEY",
]


class PaymentVerifier:
    """
    Orchestrates payment authorization extraction, challenge verification,
    Facilitator clearing, and Gateway state recording.
    """

    def __init__(
        self,
        challenge_manager: ChallengeManager,
        *,
        facilitator_client: FacilitatorClient | Any | None = None,
        state_coordinator: GatewayStateCoordinator | None = None,
        settlement_worker: Any | None = None,
        clock: Callable[[], int] | None = None,
    ) -> None:
        self.challenge_manager = challenge_manager
        self.state_coordinator = state_coordinator
        self.settlement_worker = settlement_worker
        self._clock = clock or challenge_manager.get_time

        if facilitator_client is not None:
            self._facilitator = facilitator_client
        else:
            self._facilitator = FacilitatorClient(
                base_url=challenge_manager.facilitator_config.url,
                timeout=challenge_manager.facilitator_config.timeout_seconds,
            )

    def get_time(self) -> int:
        return self._clock()

    def extract_authorization(
        self,
        headers: Mapping[str, str],
    ) -> PaymentAuthorization | None:
        """
        Extract and decode PaymentAuthorization from request headers.
        Searches standard PAYMENT-SIGNATURE and common aliases.
        """
        # Try standard extraction
        auth = extract_authorization_from_headers(headers)
        if auth is not None:
            return auth

        # Try case-insensitive alias lookups
        for key, value in headers.items():
            if key.upper() in AUTH_HEADER_ALIASES:
                try:
                    return decode_authorization(value)
                except Exception as exc:
                    raise InvalidAuthorizationHeaderError(
                        f"Failed to decode payment authorization header '{key}': {exc}"
                    ) from exc

            # Also check Authorization header if prefixed with 'Signature ' or json
            if key.upper() == "AUTHORIZATION":
                val = value.strip()
                if val.lower().startswith("signature "):
                    raw_val = val[10:].strip()
                    try:
                        return decode_authorization(raw_val)
                    except Exception as exc:
                        raise InvalidAuthorizationHeaderError(
                            f"Failed to decode Authorization: Signature header: {exc}"
                        ) from exc
                elif val.startswith("{") and "challengeId" in val:
                    try:
                        return decode_authorization(val)
                    except Exception as exc:
                        raise InvalidAuthorizationHeaderError(
                            f"Failed to decode Authorization JSON header: {exc}"
                        ) from exc

        return None

    def extract_idempotency_key(
        self,
        headers: Mapping[str, str],
        auth: PaymentAuthorization | None = None,
    ) -> str:
        """
        Extract idempotency key from headers or derive from unique authorization nonce.
        """
        for key, value in headers.items():
            if key.upper() in IDEMPOTENCY_HEADER_ALIASES:
                clean = value.strip()
                if clean:
                    return clean

        if auth is not None and auth.nonce:
            return f"idem_nonce_{auth.nonce}"

        return f"idem_{uuid.uuid4().hex}"

    def verify_authorization(
        self,
        route: RouteConfig,
        auth: PaymentAuthorization,
        challenge: PaymentChallenge | None = None,
        idempotency_key: str | None = None,
    ) -> VerificationResult:
        """
        Verify payment authorization against the challenge and Facilitator.

        Args:
            route: Matched RouteConfig.
            auth: Extracted PaymentAuthorization.
            challenge: Optional PaymentChallenge (retrieved from cache if omitted).
            idempotency_key: Optional client idempotency key.

        Returns:
            VerificationResult with Facilitator verification details.

        Raises:
            ChallengeMismatchError: If parameters don't match route pricing.
            ChallengeExpiredError: If authorization window has passed.
            PaymentRejectedError: If Facilitator rejects signature or nonce.
            FacilitatorUnavailableError: If Facilitator cannot be contacted.
        """
        now = self.get_time()

        # 1. Resolve Challenge
        if challenge is None:
            challenge = self.challenge_manager.get_challenge(auth.challenge_id)

        if challenge is None:
            # Reconstruct expected challenge from route parameters if not found in memory
            challenge = PaymentChallenge(
                version="2.0",
                challengeId=auth.challenge_id,
                network=route.network,
                asset=route.asset,
                assetSymbol=route.asset_symbol,
                decimals=route.asset_decimals,
                amount=str(route.price_base_units),
                payee=route.pay_to,
                scheme="EIP-3009",
                validAfter=auth.valid_after,
                validBefore=auth.valid_before,
                facilitatorUrl=self.challenge_manager.facilitator_config.url,
            )

        # 2. Pre-verification consistency checks
        if auth.valid_before < now:
            raise ChallengeExpiredError(f"Payment authorization expired at {auth.valid_before} (current time {now})")
        if auth.valid_after > now:
            raise ChallengeMismatchError(
                f"Payment authorization not yet valid (validAfter={auth.valid_after}, current={now})"
            )

        if int(challenge.amount) < route.price_base_units:
            raise ChallengeMismatchError(
                f"Authorized amount ({challenge.amount}) is less than required price ({route.price_base_units})"
            )
        if challenge.payee.lower() != route.pay_to.lower():
            raise ChallengeMismatchError(f"Payee mismatch: expected '{route.pay_to}', got '{challenge.payee}'")
        if challenge.asset.lower() != route.asset.lower():
            raise ChallengeMismatchError(f"Asset mismatch: expected '{route.asset}', got '{challenge.asset}'")
        if challenge.network != route.network:
            raise ChallengeMismatchError(f"Network mismatch: expected '{route.network}', got '{challenge.network}'")

        # 3. Call Facilitator
        try:
            if hasattr(self._facilitator, "verify"):
                # Select the adapter from the callable signature before invoking it. Catching
                # TypeError around the call itself can retry after an implementation has
                # already performed side effects and then raised TypeError internally.
                verify_method = self._facilitator.verify
                try:
                    inspect.signature(verify_method).bind(challenge=challenge, authorization=auth)
                except TypeError:
                    verify_req = VerifyRequest(challenge=challenge, authorization=auth)
                    verify_resp: VerifyResponse = verify_method(verify_req)
                else:
                    verify_resp: VerifyResponse = self._facilitator.verify(
                        challenge=challenge,
                        authorization=auth,
                    )
            elif callable(self._facilitator):
                # Custom callable handler
                verify_resp = self._facilitator(challenge, auth)
            else:
                raise FacilitatorUnavailableError("No valid Facilitator verification client configured")
        except InvalidSignatureError as exc:
            raise PaymentRejectedError(
                f"Invalid cryptographic signature: {exc}", error_code="INVALID_SIGNATURE"
            ) from exc
        except NonceAlreadyUsedError as exc:
            raise PaymentRejectedError(
                f"Nonce already used (replay rejected): {exc}", error_code="NONCE_REPLAY"
            ) from exc
        except FacChallengeExpiredError as exc:
            raise ChallengeExpiredError(f"Challenge expired according to Facilitator: {exc}") from exc
        except FacilitatorConnectionError as exc:
            raise FacilitatorUnavailableError(f"Failed to communicate with Facilitator: {exc}") from exc
        except Exception as exc:
            if isinstance(exc, (PaymentRejectedError, ChallengeExpiredError, FacilitatorUnavailableError)):
                raise
            raise PaymentRejectedError(f"Facilitator verification failed: {exc}") from exc

        # 4. Generate settlement IDs & record state if coordinator present
        settlement_id = f"stl_{uuid.uuid4().hex}"
        idem_key = idempotency_key or f"idem_{auth.nonce}"
        raw_sig = auth.signature if isinstance(auth.signature, str) else auth.signature.compact

        fulfillment_id: str | None = None
        if self.state_coordinator is not None:
            settlement_record = GatewaySettlementRecord(
                settlement_id=settlement_id,
                idempotency_key=idem_key,
                payer_address=auth.payer.lower(),
                payee_address=challenge.payee.lower(),
                asset_address=challenge.asset.lower(),
                network_id=challenge.network,
                amount_base_units=int(challenge.amount),
                nonce=auth.nonce,
                valid_after=auth.valid_after,
                valid_before=auth.valid_before,
                raw_signature=raw_sig,
                status=SettlementState.SUBMITTED,
                created_at=now,
                updated_at=now,
            )
            # Register inbound settlement and acquire idempotency lease
            try:
                fulfillment_id = self.state_coordinator.register_inbound_payment(settlement_record)
            except Exception as exc:
                err_str = str(exc).lower()
                if "nonce" in err_str or "unique constraint" in err_str:
                    raise PaymentRejectedError(
                        f"Nonce {auth.nonce} has already been used (replay rejected): {exc}",
                        error_code="NONCE_REPLAY",
                    ) from exc
                raise

        # Asynchronously enqueue settlement clearing to background worker
        if self.settlement_worker is not None:
            task = SettlementTask(
                settlement_id=settlement_id,
                verification_token=verify_resp.verification_token,
                authorization=auth,
                idempotency_key=idem_key,
                enqueued_at=now,
            )
            self.settlement_worker.submit(task)

        return VerificationResult(
            verified_payer=verify_resp.verified_payer,
            verification_token=verify_resp.verification_token,
            verified_at=verify_resp.verified_at,
            settlement_id=settlement_id,
            idempotency_key=idem_key,
            route=route,
            challenge=challenge,
            authorization=auth,
            fulfillment_id=fulfillment_id,
        )

    def process_request(
        self,
        route: RouteConfig,
        resource_uri: str,
        headers: Mapping[str, str],
    ) -> PaymentProcessResult:
        """
        Full-lifecycle request payment processing:
          1. Free route pass-through (price == 0).
          2. Challenge generation when authorization header is absent (HTTP 402).
          3. Idempotency evaluation (cache hit / free replay).
          4. Facilitator verification and state persistence.
        """
        # Step 1: Free route check
        if route.price_base_units == 0:
            return PaymentProcessResult(status=PaymentStatus.FREE, http_status_code=200)

        # Step 2: Extract authorization
        try:
            auth = self.extract_authorization(headers)
        except InvalidAuthorizationHeaderError as exc:
            return PaymentProcessResult(
                status=PaymentStatus.REJECTED,
                http_status_code=400,
                error_message=str(exc),
            )

        # Step 3: No authorization provided -> Issue 402 Challenge
        if auth is None:
            challenge = self.challenge_manager.generate_challenge(route, resource_uri)
            _, resp_headers, _ = self.challenge_manager.build_402_response_dict(challenge)
            challenge_header = resp_headers.get("PAYMENT-REQUIRED")

            return PaymentProcessResult(
                status=PaymentStatus.CHALLENGE_REQUIRED,
                http_status_code=402,
                challenge=challenge,
                challenge_header=challenge_header,
                error_message="Payment required to access this resource",
            )

        # Step 4: Extract idempotency key
        idem_key = self.extract_idempotency_key(headers, auth)

        # Step 5: Check Idempotency Cache (if state coordinator configured)
        if self.state_coordinator is not None:
            eval_result = self.state_coordinator.check_idempotency(auth.payer, idem_key)
            if eval_result.is_cache_hit:
                return PaymentProcessResult(
                    status=PaymentStatus.CACHE_HIT,
                    http_status_code=eval_result.cached_status_code or 200,
                    payer_address=auth.payer,
                    idempotency_key=idem_key,
                    cached_status_code=eval_result.cached_status_code,
                    cached_headers_json=eval_result.cached_headers_json,
                    cached_body_blob=eval_result.cached_body_blob,
                )
            if eval_result.is_free_replay:
                return PaymentProcessResult(
                    status=PaymentStatus.FREE_REPLAY,
                    http_status_code=200,
                    payer_address=auth.payer,
                    idempotency_key=idem_key,
                    settlement_id=eval_result.settlement_id,
                    fulfillment_id=eval_result.fulfillment_id,
                )
            if eval_result.is_in_flight:
                return PaymentProcessResult(
                    status=PaymentStatus.ERROR,
                    http_status_code=409,
                    payer_address=auth.payer,
                    idempotency_key=idem_key,
                    error_message="Concurrent request with the same idempotency key is in progress",
                )

        # Step 6: Verify Authorization with Facilitator
        try:
            verification = self.verify_authorization(
                route=route,
                auth=auth,
                idempotency_key=idem_key,
            )
            return PaymentProcessResult(
                status=PaymentStatus.VERIFIED,
                http_status_code=200,
                verification=verification,
                payer_address=verification.verified_payer,
                idempotency_key=idem_key,
            )
        except PaymentRejectedError as exc:
            return PaymentProcessResult(
                status=PaymentStatus.REJECTED,
                http_status_code=402,
                payer_address=auth.payer,
                idempotency_key=idem_key,
                error_message=str(exc),
            )
        except ChallengeExpiredError as exc:
            # Issue a fresh challenge on expired auth
            fresh_challenge = self.challenge_manager.generate_challenge(route, resource_uri)
            _, resp_headers, _ = self.challenge_manager.build_402_response_dict(fresh_challenge)
            return PaymentProcessResult(
                status=PaymentStatus.CHALLENGE_REQUIRED,
                http_status_code=402,
                challenge=fresh_challenge,
                challenge_header=resp_headers.get("PAYMENT-REQUIRED"),
                payer_address=auth.payer,
                idempotency_key=idem_key,
                error_message=f"Authorization expired: {exc}",
            )
        except ChallengeMismatchError as exc:
            return PaymentProcessResult(
                status=PaymentStatus.REJECTED,
                http_status_code=400,
                payer_address=auth.payer,
                idempotency_key=idem_key,
                error_message=str(exc),
            )
        except FacilitatorUnavailableError as exc:
            return PaymentProcessResult(
                status=PaymentStatus.ERROR,
                http_status_code=502,
                payer_address=auth.payer,
                idempotency_key=idem_key,
                error_message=f"Facilitator clearing service unavailable: {exc}",
            )
