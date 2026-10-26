"""
Unit tests for Gateway Payment Challenge Generation and Facilitator Verification (T5.2).
"""

from __future__ import annotations

import pytest
from agentpay.client.signer import LocalSigner, generate_nonce
from agentpay.core.codecs import (
    HEADER_PAYMENT_REQUIRED,
    HEADER_PAYMENT_SIGNATURE,
    decode_challenge,
    encode_authorization,
)
from agentpay.core.models import PaymentAuthorization, PaymentChallenge
from agentpay.policy.models import ReservationToken
from agentpay_gateway.config.models import FacilitatorConfig, RouteConfig, SettlementDefaults
from agentpay_gateway.facilitator.service import MockFacilitatorService
from agentpay_gateway.payment.challenge import ChallengeManager
from agentpay_gateway.payment.errors import (
    ChallengeMismatchError,
)
from agentpay_gateway.payment.models import PaymentStatus
from agentpay_gateway.payment.verifier import PaymentVerifier
from agentpay_gateway.state.coordinator import GatewayStateCoordinator
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database

# Standard test fixtures
MERCHANT_PAY_TO = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
BASE_USDC_ASSET = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
NETWORK_ID = "eip155:84532"
BUYER_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"


@pytest.fixture
def mock_clock() -> list[int]:
    # Mutable time container for controlled testing
    return [1729000000]


@pytest.fixture
def get_time(mock_clock: list[int]):
    return lambda: mock_clock[0]


@pytest.fixture
def facilitator_service(get_time) -> MockFacilitatorService:
    return MockFacilitatorService(clock=get_time)


@pytest.fixture
def challenge_manager(get_time) -> ChallengeManager:
    fac_cfg = FacilitatorConfig(url="http://127.0.0.1:8001")
    settlement_defaults = SettlementDefaults(
        default_network=NETWORK_ID,
        default_asset=BASE_USDC_ASSET,
        default_pay_to=MERCHANT_PAY_TO,
        default_challenge_ttl_seconds=120,
    )
    return ChallengeManager(
        facilitator_config=fac_cfg,
        settlement_defaults=settlement_defaults,
        clock=get_time,
    )


@pytest.fixture
def test_route() -> RouteConfig:
    return RouteConfig(
        path="/v1/market-data",
        upstream="http://upstream.internal/market-data",
        price="0.001 USDC",  # 1000 base units
        pay_to=MERCHANT_PAY_TO,
        network=NETWORK_ID,
        asset=BASE_USDC_ASSET,
        asset_decimals=6,
        challenge_ttl_seconds=120,
    )


@pytest.fixture
def free_route() -> RouteConfig:
    return RouteConfig(
        path="/v1/status",
        upstream="http://upstream.internal/status",
        price=0,
        pay_to=MERCHANT_PAY_TO,
        network=NETWORK_ID,
        asset=BASE_USDC_ASSET,
    )


@pytest.fixture
def payment_verifier(
    challenge_manager: ChallengeManager, facilitator_service: MockFacilitatorService, get_time
) -> PaymentVerifier:
    conn = create_gateway_connection(":memory:")
    init_gateway_database(conn)
    coordinator = GatewayStateCoordinator(conn, clock=get_time)

    return PaymentVerifier(
        challenge_manager=challenge_manager,
        facilitator_client=facilitator_service,
        state_coordinator=coordinator,
        clock=get_time,
    )


def _sign_challenge(
    challenge: PaymentChallenge,
    signer: LocalSigner,
    *,
    nonce: str | None = None,
    valid_before_offset: int = 120,
) -> PaymentAuthorization:
    res = ReservationToken(
        reservation_id=f"res_{challenge.challenge_id}",
        intent_id=challenge.challenge_id,
        policy_id="pol_default",
        amount=challenge.amount_base_units,
        expires_at=challenge.valid_before + 60,
        created_at=challenge.valid_after,
    )
    return signer.sign_payment_authorization(
        challenge,
        res,
        nonce=nonce or generate_nonce(),
        token_name="USD Coin",
        token_version="2",
    )


# ==============================================================================
# ChallengeManager Tests
# ==============================================================================


class TestChallengeManager:
    """Tests for x402 challenge generation and lifecycle."""

    def test_generate_challenge_from_route(
        self,
        challenge_manager: ChallengeManager,
        test_route: RouteConfig,
        mock_clock: list[int],
    ) -> None:
        challenge = challenge_manager.generate_challenge(test_route, "/v1/market-data")
        assert challenge.version == "2.0"
        assert challenge.challenge_id.startswith("ch_")
        assert challenge.network == NETWORK_ID
        assert challenge.asset == BASE_USDC_ASSET.lower()
        assert challenge.amount == "1000"
        assert challenge.payee == MERCHANT_PAY_TO.lower()
        assert challenge.valid_after == mock_clock[0]
        assert challenge.valid_before == mock_clock[0] + 120
        assert challenge.facilitator_url == "http://127.0.0.1:8001"

    def test_challenge_retrieval_and_caching(
        self,
        challenge_manager: ChallengeManager,
        test_route: RouteConfig,
    ) -> None:
        challenge = challenge_manager.generate_challenge(test_route, "/v1/market-data")
        cached = challenge_manager.get_challenge(challenge.challenge_id)
        assert cached is not None
        assert cached.challenge_id == challenge.challenge_id

    def test_challenge_expiration_pruning(
        self,
        challenge_manager: ChallengeManager,
        test_route: RouteConfig,
        mock_clock: list[int],
    ) -> None:
        challenge = challenge_manager.generate_challenge(test_route, "/v1/market-data")
        # Advance clock beyond valid_before
        mock_clock[0] += 150
        assert challenge_manager.get_challenge(challenge.challenge_id) is None

    def test_build_402_response_dict(
        self,
        challenge_manager: ChallengeManager,
        test_route: RouteConfig,
    ) -> None:
        challenge = challenge_manager.generate_challenge(test_route, "/v1/market-data")
        code, headers, body = challenge_manager.build_402_response_dict(challenge)
        assert code == 402
        assert HEADER_PAYMENT_REQUIRED in headers
        assert body["error"] == "payment_required"

        # Decode header and check roundtrip
        decoded = decode_challenge(headers[HEADER_PAYMENT_REQUIRED])
        assert decoded.challenge_id == challenge.challenge_id
        assert decoded.amount == "1000"


# ==============================================================================
# PaymentVerifier Tests
# ==============================================================================


class TestPaymentVerifier:
    """Tests for authorization extraction, verification, Facilitator clearing."""

    def test_process_free_route_bypasses_payment(
        self,
        payment_verifier: PaymentVerifier,
        free_route: RouteConfig,
    ) -> None:
        result = payment_verifier.process_request(
            route=free_route,
            resource_uri="/v1/status",
            headers={},
        )
        assert result.status == PaymentStatus.FREE
        assert result.http_status_code == 200

    def test_process_missing_authorization_returns_402(
        self,
        payment_verifier: PaymentVerifier,
        test_route: RouteConfig,
    ) -> None:
        result = payment_verifier.process_request(
            route=test_route,
            resource_uri="/v1/market-data",
            headers={},
        )
        assert result.status == PaymentStatus.CHALLENGE_REQUIRED
        assert result.http_status_code == 402
        assert result.challenge is not None
        assert result.challenge.amount == "1000"
        assert result.challenge_header is not None

    def test_process_valid_authorization_success(
        self,
        payment_verifier: PaymentVerifier,
        test_route: RouteConfig,
    ) -> None:
        # Step 1: Request without auth to get challenge
        res_402 = payment_verifier.process_request(test_route, "/v1/market-data", {})
        challenge = res_402.challenge
        assert challenge is not None

        # Step 2: Buyer signs the challenge
        signer = LocalSigner(BUYER_PRIVATE_KEY)
        auth = _sign_challenge(challenge, signer)
        auth_header = encode_authorization(auth)

        # Step 3: Replay with PAYMENT-SIGNATURE
        result = payment_verifier.process_request(
            route=test_route,
            resource_uri="/v1/market-data",
            headers={HEADER_PAYMENT_SIGNATURE: auth_header},
        )
        assert result.status == PaymentStatus.VERIFIED
        assert result.http_status_code == 200
        assert result.verification is not None
        assert result.verification.verified_payer == signer.address
        assert result.verification.fulfillment_id is not None

    def test_replay_attack_rejected(
        self,
        payment_verifier: PaymentVerifier,
        test_route: RouteConfig,
    ) -> None:
        # Sign a challenge
        challenge = payment_verifier.challenge_manager.generate_challenge(test_route, "/v1/market-data")
        signer = LocalSigner(BUYER_PRIVATE_KEY)
        auth = _sign_challenge(challenge, signer)
        auth_header = encode_authorization(auth)

        # First request succeeds
        res1 = payment_verifier.process_request(
            route=test_route,
            resource_uri="/v1/market-data",
            headers={HEADER_PAYMENT_SIGNATURE: auth_header, "Idempotency-Key": "first_try"},
        )
        assert res1.status == PaymentStatus.VERIFIED

        # Replay with new idempotency key but SAME signed nonce
        res2 = payment_verifier.process_request(
            route=test_route,
            resource_uri="/v1/market-data",
            headers={HEADER_PAYMENT_SIGNATURE: auth_header, "Idempotency-Key": "replay_attempt"},
        )
        # Facilitator / Gateway rejects nonce reuse
        assert res2.status == PaymentStatus.REJECTED
        assert res2.http_status_code == 402
        assert (
            "already been used" in (res2.error_message or "").lower() or "replay" in (res2.error_message or "").lower()
        )

    def test_expired_authorization_reissues_402(
        self,
        payment_verifier: PaymentVerifier,
        test_route: RouteConfig,
        mock_clock: list[int],
    ) -> None:
        challenge = payment_verifier.challenge_manager.generate_challenge(test_route, "/v1/market-data")
        signer = LocalSigner(BUYER_PRIVATE_KEY)
        auth = _sign_challenge(challenge, signer)
        auth_header = encode_authorization(auth)

        # Fast forward time beyond valid_before
        mock_clock[0] += 200

        result = payment_verifier.process_request(
            route=test_route,
            resource_uri="/v1/market-data",
            headers={HEADER_PAYMENT_SIGNATURE: auth_header},
        )
        assert result.status == PaymentStatus.CHALLENGE_REQUIRED
        assert result.http_status_code == 402
        assert result.challenge is not None
        assert "expired" in (result.error_message or "").lower()

    def test_amount_mismatch_rejected(
        self,
        payment_verifier: PaymentVerifier,
        test_route: RouteConfig,
    ) -> None:
        # Create a cheaper challenge (e.g. 500 base units instead of route required 1000)
        low_route = RouteConfig(
            path="/v1/market-data",
            upstream="http://upstream.internal",
            price=500,
            pay_to=MERCHANT_PAY_TO,
            network=NETWORK_ID,
            asset=BASE_USDC_ASSET,
        )
        low_challenge = payment_verifier.challenge_manager.generate_challenge(low_route, "/v1/market-data")
        signer = LocalSigner(BUYER_PRIVATE_KEY)
        auth = _sign_challenge(low_challenge, signer)

        with pytest.raises(ChallengeMismatchError, match="Authorized amount.*is less than required price"):
            payment_verifier.verify_authorization(route=test_route, auth=auth, challenge=low_challenge)

    def test_idempotency_cache_hit(
        self,
        payment_verifier: PaymentVerifier,
        test_route: RouteConfig,
    ) -> None:
        assert payment_verifier.state_coordinator is not None

        # Manually seed a completed idempotency cache entry
        payer = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
        idem_key = "test_cache_key"
        now = payment_verifier.get_time()

        conn = payment_verifier.state_coordinator._conn
        conn.execute(
            """
            INSERT INTO ap_gateway_settlement (
                settlement_id, idempotency_key, payer_address, payee_address,
                asset_address, network_id, amount_base_units, nonce,
                valid_after, valid_before, raw_signature, status,
                created_at, updated_at
            ) VALUES ('stl_done', ?, ?, ?, ?, ?, 1000, '0xnonce123', ?, ?, '0xsig', 'CONFIRMED', ?, ?);
            """,
            (idem_key, payer, MERCHANT_PAY_TO.lower(), BASE_USDC_ASSET.lower(), NETWORK_ID, now, now + 120, now, now),
        )
        conn.execute(
            """
            INSERT INTO ap_gateway_fulfillment (
                fulfillment_id, settlement_id, status, upstream_status_code,
                response_hash, retry_count, delivered_at, created_at, updated_at
            ) VALUES ('ful_done', 'stl_done', 'DELIVERED', 200, 'hash', 0, ?, ?, ?);
            """,
            (now, now, now),
        )
        conn.execute(
            """
            INSERT INTO ap_idempotency_cache (
                payer_address, idempotency_key, settlement_id, status,
                request_hash, response_status_code, response_headers_json,
                response_body_blob, locked_until, expires_at, created_at
            ) VALUES (?, ?, 'stl_done', 'COMPLETED', 'hash', 200, '{"content-type":"application/json"}', ?, ?, ?, ?);
            """,
            (payer, idem_key, b'{"result":"cached_depth_data"}', now + 1000, now + 86400, now),
        )
        conn.commit()

        # Generate auth with matching payer
        challenge = payment_verifier.challenge_manager.generate_challenge(test_route, "/v1/market-data")
        signer = LocalSigner(BUYER_PRIVATE_KEY)
        auth = _sign_challenge(challenge, signer)
        auth_header = encode_authorization(auth)

        res = payment_verifier.process_request(
            route=test_route,
            resource_uri="/v1/market-data",
            headers={HEADER_PAYMENT_SIGNATURE: auth_header, "Idempotency-Key": idem_key},
        )
        assert res.status == PaymentStatus.CACHE_HIT
        assert res.cached_status_code == 200
        assert res.cached_body_blob == b'{"result":"cached_depth_data"}'
