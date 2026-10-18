"""
Unit tests for x402 V2 HTTP header codecs and extraction utilities.
"""

from __future__ import annotations

import json

import pytest
from agentpay.core.codecs import (
    HEADER_PAYMENT_SIGNATURE,
    base64url_decode,
    base64url_encode,
    decode_authorization,
    decode_challenge,
    encode_authorization,
    encode_challenge,
    extract_authorization_from_headers,
    extract_challenge_from_headers,
    inject_authorization_to_headers,
)
from agentpay.core.errors import X402DecodeError, X402ErrorCode, X402ValidationError
from agentpay.core.models import PaymentAuthorization, PaymentChallenge


@pytest.fixture
def sample_challenge() -> PaymentChallenge:
    return PaymentChallenge(
        version="2.0",
        challengeId="ch_test123",
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
def sample_authorization() -> PaymentAuthorization:
    return PaymentAuthorization(
        version="2.0",
        challengeId="ch_test123",
        payer="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
        scheme="EIP-3009",
        nonce="0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
        validAfter=1729000000,
        validBefore=1729000300,
        signature="0xd90ab6b3...serialized",
    )


class TestBase64URLHelpers:
    def test_encode_decode_roundtrip(self) -> None:
        raw = b'{"hello": "world", "price": 100}'
        encoded = base64url_encode(raw)
        assert "=" not in encoded  # unpadded
        decoded = base64url_decode(encoded)
        assert decoded == raw

    def test_decode_with_padding(self) -> None:
        raw = b"test string"
        encoded = base64url_encode(raw)
        padded = encoded + "=" * ((4 - len(encoded) % 4) % 4)
        assert base64url_decode(padded) == raw


class TestChallengeCodecs:
    def test_encode_and_decode_base64_roundtrip(self, sample_challenge: PaymentChallenge) -> None:
        header_val = encode_challenge(sample_challenge, base64_url=True)
        assert isinstance(header_val, str)
        # Should not start with '{' when base64 encoded
        assert not header_val.startswith("{")

        decoded = decode_challenge(header_val)
        assert decoded == sample_challenge

    def test_encode_and_decode_json_roundtrip(self, sample_challenge: PaymentChallenge) -> None:
        header_val = encode_challenge(sample_challenge, base64_url=False)
        assert header_val.startswith("{")

        decoded = decode_challenge(header_val)
        assert decoded == sample_challenge

    def test_decode_malformed_header(self) -> None:
        with pytest.raises(X402DecodeError) as exc:
            decode_challenge("!!!not_base64_or_json!!!")
        assert exc.value.code == X402ErrorCode.MALFORMED_HEADER

    def test_decode_empty_header(self) -> None:
        with pytest.raises(X402DecodeError) as exc:
            decode_challenge("   ")
        assert exc.value.code == X402ErrorCode.MALFORMED_HEADER

    def test_decode_missing_field_raises_validation_error(self) -> None:
        incomplete = {
            "version": "2.0",
            "challengeId": "ch_bad",
            "network": "eip155:84532",
            # missing asset, amount, payee, etc.
        }
        encoded = base64url_encode(json.dumps(incomplete).encode("utf-8"))
        with pytest.raises(X402ValidationError) as exc:
            decode_challenge(encoded)
        assert exc.value.code == X402ErrorCode.MISSING_REQUIRED_FIELD


class TestAuthorizationCodecs:
    def test_encode_and_decode_base64_roundtrip(self, sample_authorization: PaymentAuthorization) -> None:
        header_val = encode_authorization(sample_authorization, base64_url=True)
        decoded = decode_authorization(header_val)
        assert decoded == sample_authorization

    def test_encode_and_decode_json_roundtrip(self, sample_authorization: PaymentAuthorization) -> None:
        header_val = encode_authorization(sample_authorization, base64_url=False)
        decoded = decode_authorization(header_val)
        assert decoded == sample_authorization


class TestHeaderExtractionAndInjection:
    def test_extract_challenge_case_insensitive(self, sample_challenge: PaymentChallenge) -> None:
        encoded = encode_challenge(sample_challenge)
        headers = {
            "Content-Type": "application/json",
            "payment-required": encoded,
        }
        extracted = extract_challenge_from_headers(headers)
        assert extracted is not None
        assert extracted == sample_challenge

    def test_extract_challenge_missing(self) -> None:
        headers = {"Content-Type": "application/json"}
        assert extract_challenge_from_headers(headers) is None

    def test_extract_authorization_case_insensitive(self, sample_authorization: PaymentAuthorization) -> None:
        encoded = encode_authorization(sample_authorization)
        headers = {
            "Payment-Signature": encoded,
        }
        extracted = extract_authorization_from_headers(headers)
        assert extracted is not None
        assert extracted == sample_authorization

    def test_inject_authorization(self, sample_authorization: PaymentAuthorization) -> None:
        headers: dict[str, str] = {}
        inject_authorization_to_headers(headers, sample_authorization)
        assert HEADER_PAYMENT_SIGNATURE in headers
        extracted = extract_authorization_from_headers(headers)
        assert extracted == sample_authorization
