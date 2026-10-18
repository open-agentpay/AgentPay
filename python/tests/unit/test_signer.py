"""
Unit and conformance tests for LocalSigner, EIP-712/3009 signing, and safety guardrails.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest
from agentpay.client.errors import (
    InvalidPrivateKeyError,
    ReservationMismatchError,
    UnsupportedNetworkError,
)
from agentpay.client.signer import LocalSigner, generate_nonce, parse_caip2_chain_id
from agentpay.core.models import PaymentChallenge
from agentpay.policy.models import ReservationToken


@pytest.fixture
def test_vectors_dir() -> Path:
    repo_root = Path(__file__).resolve().parent.parent.parent.parent
    primary = repo_root / "specs" / "test-vectors"
    return primary if primary.exists() else (repo_root / "specs" / "conformance" / "test-vectors")


@pytest.fixture
def eip712_vectors(test_vectors_dir: Path) -> dict:
    with open(test_vectors_dir / "eip712-authorization.json", "r", encoding="utf-8") as f:
        return json.load(f)


@pytest.fixture
def sample_challenge() -> PaymentChallenge:
    return PaymentChallenge(
        version="2.0",
        challengeId="ch_01j7xyz890abcdef12345678",
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
def sample_reservation() -> ReservationToken:
    return ReservationToken(
        reservation_id="res_01j7xyz123",
        intent_id="ch_01j7xyz890abcdef12345678",
        policy_id="pol_default",
        amount=50000,
        expires_at=1729000360,
        created_at=1729000000,
    )


class TestLocalSignerBasics:
    def test_signer_initialization(self) -> None:
        priv_key = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
        signer = LocalSigner(priv_key)
        assert signer.address == "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"

    def test_signer_generate(self) -> None:
        signer1 = LocalSigner.generate()
        signer2 = LocalSigner.generate()
        assert signer1.address != signer2.address
        assert signer1.address.startswith("0x")
        assert len(signer1.address) == 42

    def test_invalid_private_key_rejected(self) -> None:
        with pytest.raises(InvalidPrivateKeyError):
            LocalSigner("0xinvalidhex")

        with pytest.raises(InvalidPrivateKeyError):
            LocalSigner("short")

    def test_private_key_redaction(self) -> None:
        priv_key = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
        signer = LocalSigner(priv_key)
        repr_str = repr(signer)
        str_str = str(signer)

        assert "ac0974" not in repr_str
        assert "ac0974" not in str_str
        assert signer.address in repr_str
        assert signer.address in str_str

    def test_nonce_generation(self) -> None:
        n1 = generate_nonce()
        n2 = generate_nonce()
        assert n1 != n2
        assert n1.startswith("0x")
        assert len(n1) == 66  # 0x + 64 hex chars = 32 bytes

    def test_caip2_chain_id_parsing(self) -> None:
        assert parse_caip2_chain_id("eip155:1") == 1
        assert parse_caip2_chain_id("eip155:84532") == 84532

        with pytest.raises(UnsupportedNetworkError):
            parse_caip2_chain_id("solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp")


class TestLocalSignerConformanceVectors:
    def test_eip712_conformance_cases(self, eip712_vectors: dict) -> None:
        test_wallet = eip712_vectors["testWallet"]
        signer = LocalSigner(test_wallet["privateKey"])
        assert signer.address == test_wallet["address"].lower()

        domain = eip712_vectors["domain"]
        token_name = domain["name"]
        token_version = domain["version"]

        for tc in eip712_vectors["testCases"]:
            if "expectedSignature" not in tc:
                continue

            tc_id = tc["id"]
            msg = tc["message"]
            expected_sig = tc["expectedSignature"]

            # Construct challenge matching message
            challenge = PaymentChallenge(
                version="2.0",
                challengeId=f"ch_{tc_id}",
                network="eip155:84532",
                asset=domain["verifyingContract"],
                assetSymbol="USDC",
                decimals=6,
                amount=msg["value"],
                payee=msg["to"],
                scheme="EIP-3009",
                validAfter=int(msg["validAfter"]),
                validBefore=int(msg["validBefore"]),
                facilitatorUrl="https://facilitator.example.com",
            )

            # Construct matching reservation
            reservation = ReservationToken(
                reservation_id="res_test",
                intent_id=challenge.challenge_id,
                policy_id="pol_default",
                amount=int(msg["value"]),
                expires_at=int(msg["validBefore"]) + 60,
                created_at=int(msg["validAfter"]),
            )

            auth = signer.sign_payment_authorization(
                challenge,
                reservation,
                nonce=msg["nonce"],
                token_name=token_name,
                token_version=token_version,
            )

            sig = auth.signature
            assert hasattr(sig, "v")
            assert sig.v == expected_sig["v"]
            assert int(sig.r, 16) == int(expected_sig["r"], 16)
            assert int(sig.s, 16) == int(expected_sig["s"], 16)
            assert sig.serialized == expected_sig["serialized"].lower()

            # Verify with verify_authorization
            is_valid = LocalSigner.verify_authorization(
                auth,
                challenge,
                token_name=token_name,
                token_version=token_version,
            )
            assert is_valid, f"Verification failed for {tc_id}"


class TestLocalSignerGuardrails:
    def test_refuse_on_intent_id_mismatch(
        self, sample_challenge: PaymentChallenge, sample_reservation: ReservationToken
    ) -> None:
        signer = LocalSigner.generate()
        bad_reservation = ReservationToken(
            reservation_id=sample_reservation.reservation_id,
            intent_id="different_intent_id",
            policy_id=sample_reservation.policy_id,
            amount=sample_reservation.amount,
            expires_at=sample_reservation.expires_at,
            created_at=sample_reservation.created_at,
        )

        with pytest.raises(ReservationMismatchError) as exc:
            signer.sign_payment_authorization(sample_challenge, bad_reservation)
        assert "does not match challenge_id" in str(exc.value)

    def test_refuse_on_amount_mismatch(
        self, sample_challenge: PaymentChallenge, sample_reservation: ReservationToken
    ) -> None:
        signer = LocalSigner.generate()
        bad_reservation = ReservationToken(
            reservation_id=sample_reservation.reservation_id,
            intent_id=sample_reservation.intent_id,
            policy_id=sample_reservation.policy_id,
            amount=sample_reservation.amount + 1000,  # Mismatched amount
            expires_at=sample_reservation.expires_at,
            created_at=sample_reservation.created_at,
        )

        with pytest.raises(ReservationMismatchError) as exc:
            signer.sign_payment_authorization(sample_challenge, bad_reservation)
        assert "does not match challenge amount" in str(exc.value)

    def test_refuse_on_insufficient_expiration_window(
        self, sample_challenge: PaymentChallenge, sample_reservation: ReservationToken
    ) -> None:
        signer = LocalSigner.generate()
        # Reservation expires before the challenge does!
        bad_reservation = ReservationToken(
            reservation_id=sample_reservation.reservation_id,
            intent_id=sample_reservation.intent_id,
            policy_id=sample_reservation.policy_id,
            amount=sample_reservation.amount,
            expires_at=sample_challenge.valid_before - 10,
            created_at=sample_reservation.created_at,
        )

        with pytest.raises(ReservationMismatchError) as exc:
            signer.sign_payment_authorization(sample_challenge, bad_reservation)
        assert "does not cover challenge valid_before" in str(exc.value)

    def test_refuse_on_unsupported_network(self, sample_reservation: ReservationToken) -> None:
        signer = LocalSigner.generate()
        non_evm_challenge = PaymentChallenge(
            version="2.0",
            challengeId=sample_reservation.intent_id,
            network="solana:mainnet",
            asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",
            assetSymbol="USDC",
            decimals=6,
            amount=str(sample_reservation.amount),
            payee="0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
            validAfter=1729000000,
            validBefore=1729000300,
            facilitatorUrl="https://facilitator.example.com",
        )

        with pytest.raises(UnsupportedNetworkError):
            signer.sign_payment_authorization(non_evm_challenge, sample_reservation)
