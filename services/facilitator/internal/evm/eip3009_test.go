package evm

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/models"
)

func TestSelectors(t *testing.T) {
	expectedTransferSelector := "e3ee160e"
	if hex.EncodeToString(TransferWithAuthorizationSelector) != expectedTransferSelector {
		t.Fatalf("expected transfer selector %s, got %s", expectedTransferSelector, hex.EncodeToString(TransferWithAuthorizationSelector))
	}

	expectedAuthStateSelector := "e94a0102"
	if hex.EncodeToString(AuthorizationStateSelector) != expectedAuthStateSelector {
		t.Fatalf("expected authState selector %s, got %s", expectedAuthStateSelector, hex.EncodeToString(AuthorizationStateSelector))
	}
}

func TestEncodeTransferFromAuthorization(t *testing.T) {
	auth := &models.PaymentAuthorization{
		Version:     "2.0",
		ChallengeID: "ch_test",
		Payer:       "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		Nonce:       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		ValidAfter:  1729000000,
		ValidBefore: 1729000300,
		Signature: models.SignatureData{
			V:          27,
			R:          "0x31207f584bc8e976a0ce72ea1c9c85cdb4da7686401c5b6efdc27040e5b0f22",
			S:          "0x2eacb1d50c27d4f5052b776e4bb5f1b42ce5c17fb7f553d44d0c42105444f4f1",
			Serialized: "031207f584bc8e976a0ce72ea1c9c85cdb4da7686401c5b6efdc27040e5b0f222eacb1d50c27d4f5052b776e4bb5f1b42ce5c17fb7f553d44d0c42105444f4f11b",
		},
	}

	payee := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	amount := big.NewInt(100000)

	calldata, err := EncodeTransferFromAuthorization(auth, payee, amount)
	if err != nil {
		t.Fatalf("failed to encode calldata: %v", err)
	}

	if len(calldata) != 292 {
		t.Fatalf("expected calldata length 292 bytes, got %d", len(calldata))
	}
	if hex.EncodeToString(calldata[:4]) != "e3ee160e" {
		t.Fatalf("expected selector e3ee160e, got %x", calldata[:4])
	}
}

func TestAuthorizationStateEncoding(t *testing.T) {
	authorizer := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	var nonce [32]byte
	nonce[31] = 1

	calldata := EncodeAuthorizationState(authorizer, nonce)
	if len(calldata) != 68 {
		t.Fatalf("expected calldata length 68, got %d", len(calldata))
	}
	if hex.EncodeToString(calldata[:4]) != "e94a0102" {
		t.Fatalf("expected selector e94a0102, got %x", calldata[:4])
	}

	usedData := make([]byte, 32)
	usedData[31] = 1
	if !DecodeAuthorizationStateResult(usedData) {
		t.Fatalf("expected DecodeAuthorizationStateResult to return true for non-zero data")
	}

	unusedData := make([]byte, 32)
	if DecodeAuthorizationStateResult(unusedData) {
		t.Fatalf("expected DecodeAuthorizationStateResult to return false for all-zero data")
	}
}
