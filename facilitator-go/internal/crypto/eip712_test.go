package crypto

import (
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/models"
)

type TestVectorSuite struct {
	Domain struct {
		Name              string `json:"name"`
		Version           string `json:"version"`
		ChainID           int64  `json:"chainId"`
		VerifyingContract string `json:"verifyingContract"`
	} `json:"domain"`
	TestCases []struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		Message     struct {
			From        string `json:"from"`
			To          string `json:"to"`
			Value       string `json:"value"`
			ValidAfter  string `json:"validAfter"`
			ValidBefore string `json:"validBefore"`
			Nonce       string `json:"nonce"`
		} `json:"message"`
		ExpectedSignature struct {
			V                uint8  `json:"v"`
			R                string `json:"r"`
			S                string `json:"s"`
			Serialized       string `json:"serialized"`
			RecoveredAddress string `json:"recoveredAddress"`
		} `json:"expectedSignature"`
		TamperedMessage *struct {
			From        string `json:"from"`
			To          string `json:"to"`
			Value       string `json:"value"`
			ValidAfter  string `json:"validAfter"`
			ValidBefore string `json:"validBefore"`
			Nonce       string `json:"nonce"`
		} `json:"tamperedMessage"`
		SignatureFromTC03 string `json:"signatureFromTC03"`
		ExpectedBehavior  string `json:"expectedBehavior"`
	} `json:"testCases"`
}

func TestArbitrumSepoliaLiveTxRecover(t *testing.T) {
	from := common.HexToAddress("0x90f8bf6a479f320ead074411a4b0e7944ea8c9c1")
	to := common.HexToAddress("0xa0b4af0a1fa90708a33eb127b6963a902020d2ed")
	value := big.NewInt(10000)
	validAfter := int64(1792843855)
	validBefore := int64(1792844155)
	nonceBytes, _ := NormalizeBytes32("0x5c0dab2252c6a8a603d4cc81dc33c60bcba867b8bc326aed099cd4396d6ec3e5")

	chainID := big.NewInt(421614)
	contract := common.HexToAddress("0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d")

	domainSep := ComputeDomainSeparator("USD Coin", "2", chainID, contract)
	structHash := ComputeTransferWithAuthorizationStructHash(
		from,
		to,
		value,
		validAfter,
		validBefore,
		nonceBytes,
	)
	digest := ComputeEIP712Digest(domainSep, structHash)
	t.Logf("domainSep:  0x%x", domainSep)
	t.Logf("structHash: 0x%x", structHash)
	t.Logf("digest:     0x%x", digest)

	sig := models.SignatureData{
		V: 27,
		R: "0x220b0f00666daa241656f952c92d887a8b50a1ec5132dd47803c4ea0b92395e0",
		S: "0x04f9f95e745d00cc1bea6dd821e85c8ade8918a9795c26231162ee54f23437f9",
	}

	recovered, err := RecoverPayer(digest, sig)
	t.Logf("err: %v", err)
	t.Logf("recovered: %s", recovered.Hex())
	t.Logf("expected from: %s", from.Hex())
	if !strings.EqualFold(recovered.Hex(), from.Hex()) {
		t.Fatalf("signer mismatch: recovered %s, expected %s", recovered.Hex(), from.Hex())
	}
}

func TestEIP712ConformanceVectors(t *testing.T) {
	data, err := os.ReadFile("../../../specs/test-vectors/eip712-authorization.json")
	if err != nil {
		t.Fatalf("failed to read test vectors: %v", err)
	}

	var suite TestVectorSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("failed to unmarshal test vectors: %v", err)
	}

	for _, tc := range suite.TestCases {
		t.Run(tc.ID, func(t *testing.T) {
			if tc.ExpectedBehavior == "RECOVERED_ADDRESS_MISMATCH" {
				// Negative test case: tampered message or signature replay
				if tc.TamperedMessage != nil {
					ch := &models.PaymentChallenge{
						Version:     "2.0",
						ChallengeID: "ch_neg",
						Network:     "eip155:84532",
						Asset:       suite.Domain.VerifyingContract,
						AssetSymbol: "USDC",
						Amount:      tc.TamperedMessage.Value,
						Payee:       tc.TamperedMessage.To,
						ValidAfter:  1729000000,
						ValidBefore: 1729000300,
					}
					auth := &models.PaymentAuthorization{
						Version:     "2.0",
						ChallengeID: "ch_neg",
						Payer:       tc.TamperedMessage.From,
						Nonce:       tc.TamperedMessage.Nonce,
						ValidAfter:  1729000000,
						ValidBefore: 1729000300,
						Signature: models.SignatureData{
							Serialized: tc.SignatureFromTC03,
						},
					}
					valid, _, _ := VerifyAuthorization(auth, ch, suite.Domain.Name, suite.Domain.Version)
					if valid {
						t.Errorf("expected tampered message to fail verification, but it passed!")
					}
				}
				return
			}

			if strings.TrimSpace(tc.Message.Value) == "" {
				return
			}

			ch := &models.PaymentChallenge{
				Version:     "2.0",
				ChallengeID: "ch_test",
				Network:     "eip155:84532",
				Asset:       suite.Domain.VerifyingContract,
				AssetSymbol: "USDC",
				Amount:      tc.Message.Value,
				Payee:       tc.Message.To,
				ValidAfter:  1729000000,
				ValidBefore: 1729000300,
			}

			auth := &models.PaymentAuthorization{
				Version:     "2.0",
				ChallengeID: "ch_test",
				Payer:       tc.Message.From,
				Nonce:       tc.Message.Nonce,
				ValidAfter:  1729000000,
				ValidBefore: 1729000300,
				Signature: models.SignatureData{
					V:          tc.ExpectedSignature.V,
					R:          tc.ExpectedSignature.R,
					S:          tc.ExpectedSignature.S,
					Serialized: tc.ExpectedSignature.Serialized,
				},
			}

			valid, recovered, err := VerifyAuthorization(auth, ch, suite.Domain.Name, suite.Domain.Version)
			if err != nil {
				t.Fatalf("verification returned error: %v", err)
			}
			if !valid {
				t.Errorf("expected valid signature, got invalid. recovered=%s, expected=%s", recovered, tc.ExpectedSignature.RecoveredAddress)
			}
		})
	}
}
