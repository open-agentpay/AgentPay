"""
Challenge Generation and Management for x402 V2 Gateway.
"""

from __future__ import annotations

import time
import uuid
from collections.abc import Callable
from typing import Any

from agentpay.core.codecs import (
    HEADER_PAYMENT_REQUIRED,
    encode_challenge,
)
from agentpay.core.models import PaymentChallenge

from agentpay_gateway.config.models import FacilitatorConfig, RouteConfig, SettlementDefaults


class ChallengeManager:
    """
    Generates, caches, and encodes x402 V2 payment challenges.
    """

    def __init__(
        self,
        facilitator_config: FacilitatorConfig,
        settlement_defaults: SettlementDefaults | None = None,
        *,
        clock: Callable[[], int] | None = None,
    ) -> None:
        self.facilitator_config = facilitator_config
        self.settlement_defaults = settlement_defaults or SettlementDefaults()
        self._clock = clock or (lambda: int(time.time()))
        # challenge_id -> (PaymentChallenge, expires_at)
        self._active_challenges: dict[str, tuple[PaymentChallenge, int]] = {}

    def get_time(self) -> int:
        return self._clock()

    def generate_challenge(
        self,
        route: RouteConfig,
        resource_uri: str,
        *,
        challenge_ttl_seconds: int | None = None,
    ) -> PaymentChallenge:
        """
        Create a new x402 V2 PaymentChallenge for the target route.

        Args:
            route: Matched RouteConfig.
            resource_uri: Inbound request resource URI path.
            challenge_ttl_seconds: Optional challenge expiration TTL override.

        Returns:
            Configured PaymentChallenge instance.
        """
        now = self.get_time()
        ttl = (
            challenge_ttl_seconds
            or route.challenge_ttl_seconds
            or self.settlement_defaults.default_challenge_ttl_seconds
        )
        valid_after = now
        valid_before = now + ttl

        challenge_id = f"ch_{uuid.uuid4().hex}"

        challenge = PaymentChallenge(
            version="2.0",
            challengeId=challenge_id,
            network=route.network or self.settlement_defaults.default_network,
            asset=route.asset or self.settlement_defaults.default_asset,
            assetSymbol=route.asset_symbol or self.settlement_defaults.default_asset_symbol,
            decimals=route.asset_decimals,
            amount=str(route.price_base_units),
            payee=route.pay_to or (self.settlement_defaults.default_pay_to or ""),
            scheme="EIP-3009",
            validAfter=valid_after,
            validBefore=valid_before,
            facilitatorUrl=self.facilitator_config.url,
            resourceUri=resource_uri,
        )

        # Store in active cache
        self._active_challenges[challenge_id] = (challenge, valid_before)
        self._prune_expired(now)

        return challenge

    def get_challenge(self, challenge_id: str) -> PaymentChallenge | None:
        """
        Retrieve an active challenge by challenge_id if not expired.

        Args:
            challenge_id: Unique challenge identifier.

        Returns:
            PaymentChallenge if active and unexpired, else None.
        """
        now = self.get_time()
        record = self._active_challenges.get(challenge_id)
        if not record:
            return None
        challenge, expires_at = record
        if now > expires_at:
            self._active_challenges.pop(challenge_id, None)
            return None
        return challenge

    def _prune_expired(self, current_time: int) -> None:
        """Prune expired challenges to avoid memory accumulation."""
        expired = [cid for cid, (_, exp) in self._active_challenges.items() if current_time > exp]
        for cid in expired:
            self._active_challenges.pop(cid, None)

    def build_402_response_dict(
        self,
        challenge: PaymentChallenge,
        *,
        base64_url: bool = True,
    ) -> tuple[int, dict[str, str], dict[str, Any]]:
        """
        Build standard HTTP 402 response components (status_code, headers, body_dict).

        Args:
            challenge: PaymentChallenge to encode.
            base64_url: Whether to use RFC 4648 Base64URL encoding (default True).

        Returns:
            Tuple of (status_code: 402, headers: dict, body: dict).
        """
        encoded_header = encode_challenge(challenge, base64_url=base64_url)
        headers = {
            HEADER_PAYMENT_REQUIRED: encoded_header,
            "Content-Type": "application/json",
        }
        body = {
            "error": "payment_required",
            "message": "Payment required to access this resource",
            "challenge": challenge.model_dump(by_alias=True, exclude_none=True),
        }
        return 402, headers, body
