"""
Unit tests for x402 V2 core models (PaymentChallenge, PaymentAuthorization, SignatureData).
"""

from __future__ import annotations

import pytest
from agentpay.core.errors import X402ErrorCode, X402ValidationError
from agentpay.core.models import (
    PaymentAuthorization,
    PaymentChallenge,
    SignatureData,
)
from pydantic import ValidationError


def _make_valid_challenge_dict() -> dict:
    return {
        "version": "2.0",
        "challengeId": "ch_01j7xyz890abcdef12345678",
        "network": "eip155:84532",
        "asset": "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
        "assetSymbol": "USDC",
        "decimals": 6,
        "amount": "50000",
        "payee": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
        "scheme": "EIP-3009",
        "validAfter": 1729000000,
        "validBefore": 1729000300,
        "facilitatorUrl": "https://facilitator.example.com",
        "resourceUri": "/v1/market-data",
    }


def _make_valid_auth_dict() -> dict:
    return {
        "version": "2.0",
        "challengeId": "ch_01j7xyz890abcdef12345678",
        "payer": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
        "scheme": "EIP-3009",
        "nonce": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        "validAfter": 1729000000,
        "validBefore": 1729000300,
        "signature": {
            "v": 28,
            "r": "0xd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83",
            "s": "0x530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e0",
            "serialized": "d90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e01c",
        },
    }


class TestPaymentChallengeModel:
    def test_valid_challenge_creation(self) -> None:
        data = _make_valid_challenge_dict()
        ch = PaymentChallenge(**data)
        assert ch.version == "2.0"
        assert ch.challenge_id == "ch_01j7xyz890abcdef12345678"
        assert ch.network == "eip155:84532"
        # Address auto-lowered
        assert ch.asset == "0x036cbd53842c5426634e7929541ec2318f3dcf7e"
        assert ch.payee == "0x70997970c51812dc3a010c7d01b50e0d17dc79c8"
        assert ch.amount == "50000"
        assert ch.amount_base_units == 50000
        assert ch.decimals == 6
        assert ch.valid_after == 1729000000
        assert ch.valid_before == 1729000300
        assert ch.facilitator_url == "https://facilitator.example.com"
        assert ch.resource_uri == "/v1/market-data"

    def test_challenge_immutability(self) -> None:
        ch = PaymentChallenge(**_make_valid_challenge_dict())
        with pytest.raises(ValidationError):
            ch.amount = "100000"  # type: ignore[misc]

    def test_challenge_is_expired(self) -> None:
        ch = PaymentChallenge(**_make_valid_challenge_dict())
        assert not ch.is_expired(1729000200)
        assert ch.is_expired(1729000301)

    def test_to_payment_request_conversion(self) -> None:
        ch = PaymentChallenge(**_make_valid_challenge_dict())
        req = ch.to_payment_request(domain="example.com", ttl_seconds=300)
        assert req.intent_id == ch.challenge_id
        assert req.amount == 50000
        assert req.payee == ch.payee
        assert req.asset == ch.asset
        assert req.network == ch.network
        assert req.domain == "example.com"
        assert req.ttl_seconds == 300

    def test_reject_unsupported_version(self) -> None:
        data = _make_valid_challenge_dict()
        data["version"] = "1.0"
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.UNSUPPORTED_VERSION

    def test_reject_empty_challenge_id(self) -> None:
        data = _make_valid_challenge_dict()
        data["challengeId"] = "   "
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.MISSING_REQUIRED_FIELD

    def test_reject_invalid_caip2_network(self) -> None:
        data = _make_valid_challenge_dict()
        data["network"] = "base-testnet"
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.NETWORK_NOT_WHITELISTED

    def test_reject_invalid_address(self) -> None:
        data = _make_valid_challenge_dict()
        data["asset"] = "0xnotanaddress"
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.INVALID_ADDRESS

    def test_reject_missing_or_invalid_amount(self) -> None:
        data = _make_valid_challenge_dict()
        data["amount"] = "0"
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.INVALID_AMOUNT_FORMAT

        data["amount"] = -100
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.INVALID_AMOUNT_FORMAT

        data["amount"] = 50.5
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.INVALID_AMOUNT_FORMAT

    def test_reject_inverted_time_window(self) -> None:
        data = _make_valid_challenge_dict()
        data["validAfter"] = 1729000300
        data["validBefore"] = 1729000000
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.INVALID_TIME_WINDOW

    def test_reject_equal_time_window(self) -> None:
        data = _make_valid_challenge_dict()
        data["validAfter"] = 1729000000
        data["validBefore"] = 1729000000
        with pytest.raises(X402ValidationError) as exc:
            PaymentChallenge(**data)
        assert exc.value.code == X402ErrorCode.INVALID_TIME_WINDOW


class TestPaymentAuthorizationModel:
    def test_valid_auth_creation(self) -> None:
        data = _make_valid_auth_dict()
        auth = PaymentAuthorization(**data)
        assert auth.version == "2.0"
        assert auth.challenge_id == "ch_01j7xyz890abcdef12345678"
        assert auth.payer == "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
        assert auth.scheme == "EIP-3009"
        assert auth.nonce == "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
        assert isinstance(auth.signature, SignatureData)
        assert auth.signature.v == 28

    def test_auth_with_compact_signature_string(self) -> None:
        data = _make_valid_auth_dict()
        data["signature"] = "0xd90ab6b3...compact"
        auth = PaymentAuthorization(**data)
        assert isinstance(auth.signature, str)
        assert auth.signature == "0xd90ab6b3...compact"

    def test_reject_invalid_payer(self) -> None:
        data = _make_valid_auth_dict()
        data["payer"] = "0xinvalid"
        with pytest.raises(X402ValidationError) as exc:
            PaymentAuthorization(**data)
        assert exc.value.code == X402ErrorCode.INVALID_ADDRESS

    def test_reject_invalid_nonce(self) -> None:
        data = _make_valid_auth_dict()
        data["nonce"] = "0x1234"  # Not 32 bytes
        with pytest.raises(X402ValidationError) as exc:
            PaymentAuthorization(**data)
        assert exc.value.code == X402ErrorCode.INVALID_NONCE


class TestSignatureData:
    def test_signature_serialization_auto_generation(self) -> None:
        sig = SignatureData(
            v=28,
            r="0xd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83",
            s="0x530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e0",
        )
        assert (
            sig.serialized
            == "d90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e01c"
        )
        assert sig.compact == "0x" + sig.serialized

    def test_signature_rejects_invalid_v(self) -> None:
        with pytest.raises(X402ValidationError) as exc:
            SignatureData(
                v=10,
                r="0xd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83",
                s="0x530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e0",
            )
        assert exc.value.code == X402ErrorCode.INVALID_SIGNATURE


class TestWalletFiatCurrencies:
    @pytest.mark.parametrize("currency", ["CNY", "USD", "EUR"])
    def test_wallet_challenge_accepts_supported_fiat_currencies(self, currency: str) -> None:
        challenge = PaymentChallenge(
            version="2.0",
            scheme="agentpay-wallet",
            challengeId="0x" + "ab" * 32,
            network="agentpay:internal-ledger",
            asset=currency,
            assetSymbol=currency,
            decimals=6,
            amount="350000",
            payee="agentpay:route:rt_cny_demo",
            validAfter=1700000000,
            validBefore=1700000300,
            resourceUri="/api/v1/chat",
        )
        assert challenge.asset == currency
        req = challenge.to_payment_request(domain="api.example.com")
        assert req.asset == currency
        assert req.is_agentpay_wallet is True
