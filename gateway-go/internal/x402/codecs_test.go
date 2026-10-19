package x402

import (
	"strings"
	"testing"
)

func TestCodecsBase64URLHelpers(t *testing.T) {
	raw := []byte(`{"hello":"world","price":50000}`)
	encoded := Base64URLEncode(raw)
	if strings.Contains(encoded, "=") {
		t.Errorf("expected unpadded base64url, got '=' in %s", encoded)
	}

	decoded, err := Base64URLDecode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if string(decoded) != string(raw) {
		t.Errorf("roundtrip mismatch: got %s, want %s", decoded, raw)
	}

	// Test decoding with padding tolerated
	padded := encoded + "=="
	decodedPadded, err := Base64URLDecode(padded)
	if err != nil {
		t.Fatalf("padded decode failed: %v", err)
	}
	if string(decodedPadded) != string(raw) {
		t.Errorf("padded mismatch: got %s, want %s", decodedPadded, raw)
	}
}

func TestChallengeCodecsRoundtrip(t *testing.T) {
	ch := &PaymentChallenge{
		Version:     "2.0",
		ChallengeID: "ch_test_123",
		Network:     "eip155:84532",
		Asset:       "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		AssetSymbol: "USDC",
		Decimals:    6,
		Amount:      "50000",
		Payee:       "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		Scheme:      "EIP-3009",
		ValidAfter:  1729000000,
		ValidBefore: 1729000300,
	}

	// 1. Base64URL serialization
	b64Header, err := EncodeChallenge(ch, true)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if strings.HasPrefix(b64Header, "{") {
		t.Errorf("expected base64 string, got json")
	}

	parsed, err := DecodeChallenge(b64Header)
	if err != nil {
		t.Fatalf("decode challenge failed: %v", err)
	}
	if parsed.ChallengeID != ch.ChallengeID || parsed.Amount != ch.Amount {
		t.Errorf("decoded challenge fields mismatch: %+v", parsed)
	}

	// 2. Raw JSON string serialization
	jsonHeader, err := EncodeChallenge(ch, false)
	if err != nil {
		t.Fatalf("encode json failed: %v", err)
	}
	parsedJSON, err := DecodeChallenge(jsonHeader)
	if err != nil {
		t.Fatalf("decode json challenge failed: %v", err)
	}
	if parsedJSON.ChallengeID != ch.ChallengeID {
		t.Errorf("parsed json challenge mismatch")
	}

	// 3. Reject invalid versions & malformed payloads
	_, err = DecodeChallenge("not-valid-base64-or-json@@@")
	if err == nil {
		t.Errorf("expected error for malformed challenge")
	}

	_, err = DecodeChallenge(`{"version":"1.0","challengeId":"ch_1"}`)
	if err == nil {
		t.Errorf("expected error for unsupported version 1.0")
	}
}

func TestAuthorizationCodecsRoundtrip(t *testing.T) {
	auth := &PaymentAuthorization{
		Version:     "2.0",
		ChallengeID: "ch_test_123",
		Payer:       "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		Scheme:      "EIP-3009",
		Nonce:       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		ValidAfter:  1729000000,
		ValidBefore: 1729000300,
		Signature:   "0xd90ab6b3serialized",
	}

	encoded, err := EncodeAuthorization(auth, true)
	if err != nil {
		t.Fatalf("encode auth failed: %v", err)
	}

	parsed, err := DecodeAuthorization(encoded)
	if err != nil {
		t.Fatalf("decode auth failed: %v", err)
	}
	if parsed.Payer != auth.Payer || parsed.Nonce != auth.Nonce {
		t.Errorf("auth fields mismatch: %+v", parsed)
	}
}
