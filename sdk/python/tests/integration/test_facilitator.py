"""
Unit tests for MockFacilitatorService, FacilitatorClient, and AsyncFacilitatorClient.
"""

from __future__ import annotations

import httpx
import pytest
from agentpay.client.signer import LocalSigner
from agentpay.core.models import PaymentChallenge
from agentpay.policy.models import ReservationToken
from agentpay_gateway.facilitator.client import AsyncFacilitatorClient, FacilitatorClient
from agentpay_gateway.facilitator.errors import (
    ChallengeExpiredError,
    InvalidRequestError,
    InvalidSignatureError,
    NonceAlreadyUsedError,
    SettlementNotFoundError,
)
from agentpay_gateway.facilitator.models import (
    SettlementState,
    SettleRequest,
    VerifyRequest,
)
from agentpay_gateway.facilitator.service import MockFacilitatorService


@pytest.fixture
def current_time() -> int:
    return 1729000100


@pytest.fixture
def clock(current_time: int):
    # Simulated clock
    curr = [current_time]

    def _clock() -> int:
        return curr[0]

    _clock.set = lambda t: curr.__setitem__(0, t)  # type: ignore[attr-defined]
    _clock.advance = lambda d: curr.__setitem__(0, curr[0] + d)  # type: ignore[attr-defined]
    return _clock


@pytest.fixture
def signer() -> LocalSigner:
    # Deterministic test wallet
    return LocalSigner("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")


@pytest.fixture
def challenge() -> PaymentChallenge:
    return PaymentChallenge(
        version="2.0",
        challengeId="ch_fac_test_01",
        network="eip155:84532",
        asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",
        assetSymbol="USDC",
        decimals=6,
        amount="50000",
        payee="0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
        scheme="EIP-3009",
        validAfter=1729000000,
        validBefore=1729000300,
        facilitatorUrl="https://facilitator.example.com",
        resourceUri="/v1/market-data",
    )


@pytest.fixture
def reservation(challenge: PaymentChallenge) -> ReservationToken:
    return ReservationToken(
        reservation_id="res_fac_test",
        intent_id=challenge.challenge_id,
        policy_id="pol_default",
        amount=challenge.amount_base_units,
        expires_at=challenge.valid_before + 60,
        created_at=challenge.valid_after,
    )


@pytest.fixture
def service(clock) -> MockFacilitatorService:
    return MockFacilitatorService(clock=clock)


class TestMockFacilitatorServiceDirect:
    def test_verify_success(
        self,
        service: MockFacilitatorService,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        auth = signer.sign_payment_authorization(challenge, reservation)
        req = VerifyRequest(challenge=challenge, authorization=auth)
        res = service.verify(req)

        assert res.valid is True
        assert res.verified_payer == signer.address
        assert res.verification_token.startswith("vtok_")

    def test_verify_rejects_invalid_signature(
        self,
        service: MockFacilitatorService,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        auth = signer.sign_payment_authorization(challenge, reservation)
        # Tamper payer address
        tampered_auth = auth.model_copy(update={"payer": "0x70997970c51812dc3a010c7d01b50e0d17dc79c8"})
        req = VerifyRequest(challenge=challenge, authorization=tampered_auth)

        with pytest.raises(InvalidSignatureError):
            service.verify(req)

    def test_verify_rejects_expired_challenge(
        self,
        service: MockFacilitatorService,
        clock,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        auth = signer.sign_payment_authorization(challenge, reservation)
        # Advance time past validBefore
        clock.set(challenge.valid_before + 10)

        req = VerifyRequest(challenge=challenge, authorization=auth)
        with pytest.raises(ChallengeExpiredError):
            service.verify(req)

    def test_verify_rejects_not_yet_valid_challenge(
        self,
        service: MockFacilitatorService,
        clock,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        auth = signer.sign_payment_authorization(challenge, reservation)
        # Rewind time before validAfter
        clock.set(challenge.valid_after - 10)

        req = VerifyRequest(challenge=challenge, authorization=auth)
        with pytest.raises(InvalidRequestError):
            service.verify(req)

    def test_settle_lifecycle_and_replay_defense(
        self,
        service: MockFacilitatorService,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        auth = signer.sign_payment_authorization(challenge, reservation)
        v_res = service.verify(VerifyRequest(challenge=challenge, authorization=auth))

        # Settle first time
        s_req = SettleRequest(
            verificationToken=v_res.verification_token,
            authorization=auth,
            idempotencyKey="idem_1",
        )
        s_res = service.settle(s_req)

        assert s_res.status == SettlementState.SUBMITTED
        assert s_res.settlement_id.startswith("stl_")

        # Idempotent replay with same key returns identical result
        s_res2 = service.settle(s_req)
        assert s_res2.settlement_id == s_res.settlement_id

        # Replaying same nonce with a different verification token must raise NonceAlreadyUsedError
        new_ch = challenge.model_copy(update={"challenge_id": "ch_replay"})
        new_res = reservation.model_copy(update={"intent_id": "ch_replay"})
        # Reuse same auth nonce
        auth_replay = signer.sign_payment_authorization(new_ch, new_res, nonce=auth.nonce)
        with pytest.raises(NonceAlreadyUsedError):
            service.verify(VerifyRequest(challenge=new_ch, authorization=auth_replay))

    def test_advance_and_get_settlement(
        self,
        service: MockFacilitatorService,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        auth = signer.sign_payment_authorization(challenge, reservation)
        v_res = service.verify(VerifyRequest(challenge=challenge, authorization=auth))
        s_res = service.settle(SettleRequest(verificationToken=v_res.verification_token, authorization=auth))

        st = service.get_settlement(s_res.settlement_id)
        assert st.status == SettlementState.SUBMITTED

        # Simulate on-chain confirmation
        service.advance_settlement(
            s_res.settlement_id,
            SettlementState.CONFIRMED,
            tx_hash="0xabcd1234",
        )
        updated = service.get_settlement(s_res.settlement_id)
        assert updated.status == SettlementState.CONFIRMED
        assert updated.tx_hash == "0xabcd1234"


class TestFacilitatorClientsWithASGITransport:
    def test_sync_client_full_flow(
        self,
        service: MockFacilitatorService,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        transport = service.create_sync_transport()
        with FacilitatorClient("http://facilitator.internal", transport=transport) as client:
            # 1. Health check
            h = client.health()
            assert h["status"] == "ok"

            # 2. Verify
            auth = signer.sign_payment_authorization(challenge, reservation)
            v_res = client.verify(challenge, auth)
            assert v_res.valid is True
            assert v_res.verified_payer == signer.address

            # 3. Settle
            s_res = client.settle(
                auth,
                v_res.verification_token,
                idempotency_key="test_idem_client",
            )
            assert s_res.status == SettlementState.SUBMITTED

            # 4. Get Settlement
            st = client.get_settlement(s_res.settlement_id)
            assert st.status == SettlementState.SUBMITTED

    def test_sync_client_error_handling(
        self,
        service: MockFacilitatorService,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        transport = service.create_sync_transport()
        with FacilitatorClient("http://facilitator.internal", transport=transport) as client:
            # 404 Settlement not found
            with pytest.raises(SettlementNotFoundError):
                client.get_settlement("stl_nonexistent")

            # 401 Invalid signature
            auth = signer.sign_payment_authorization(challenge, reservation)
            tampered_auth = auth.model_copy(update={"signature": "0xinvalid"})
            with pytest.raises(InvalidSignatureError):
                client.verify(challenge, tampered_auth)

    def test_async_client_full_flow(
        self,
        service: MockFacilitatorService,
        signer: LocalSigner,
        challenge: PaymentChallenge,
        reservation: ReservationToken,
    ) -> None:
        import asyncio

        async def _run() -> None:
            transport = httpx.ASGITransport(app=service.asgi_app)  # type: ignore[arg-type]
            async with AsyncFacilitatorClient("http://facilitator.internal", transport=transport) as client:
                auth = signer.sign_payment_authorization(challenge, reservation)
                v_res = await client.verify(challenge, auth)
                assert v_res.valid is True

                s_res = await client.settle(auth, v_res.verification_token)
                assert s_res.status == SettlementState.SUBMITTED

                st = await client.get_settlement(s_res.settlement_id)
                assert st.status == SettlementState.SUBMITTED

        asyncio.run(_run())
