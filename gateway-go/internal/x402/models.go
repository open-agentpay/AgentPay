// Package x402 implements x402 V2 payment challenge generation, header codecs,
// cryptographic verification, and dual-path execution for AgentPay Go Gateway (T2.3).
package x402

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	HeaderPaymentRequired  = "PAYMENT-REQUIRED"
	HeaderPaymentSignature = "PAYMENT-SIGNATURE"
	HeaderIdempotencyKey   = "Idempotency-Key"
	DefaultVersion         = "2.0"
	DefaultScheme          = "EIP-3009"
	DefaultChallengeTTL    = 300 * time.Second // 5 minutes
)

// PaymentChallenge represents the structured 402 challenge issued to the buyer.
type PaymentChallenge struct {
	Version        string `json:"version"`
	ChallengeID    string `json:"challengeId"`
	Network        string `json:"network"` // CAIP-2, e.g. eip155:84532
	Asset          string `json:"asset"`   // Token contract address
	AssetSymbol    string `json:"assetSymbol"`
	Decimals       int    `json:"decimals"`
	Amount         string `json:"amount"` // Base units integer string
	Payee          string `json:"payee"`  // Payee treasury address
	Scheme         string `json:"scheme"` // EIP-3009
	ValidAfter     int64  `json:"validAfter"`
	ValidBefore    int64  `json:"validBefore"`
	FacilitatorURL string `json:"facilitatorUrl,omitempty"`
	ResourceURI    string `json:"resourceUri,omitempty"`
}

// PaymentAuthorization represents the buyer's signed payment authorization payload.
type PaymentAuthorization struct {
	Version     string `json:"version"`
	ChallengeID string `json:"challengeId"`
	Payer       string `json:"payer"`  // Payer signer address
	Scheme      string `json:"scheme"` // EIP-3009
	Nonce       string `json:"nonce"`  // 32-byte hex nonce
	ValidAfter  int64  `json:"validAfter"`
	ValidBefore int64  `json:"validBefore"`
	Signature   string `json:"signature"` // Serialized signature (0x...)
}

// NewChallenge generates a fresh PaymentChallenge with a random challenge ID.
func NewChallenge(network, asset, assetSymbol string, decimals int, amount, payee, facilitatorURL, resourceURI string, ttl time.Duration) *PaymentChallenge {
	if ttl <= 0 {
		ttl = DefaultChallengeTTL
	}
	now := time.Now().Unix()
	return &PaymentChallenge{
		Version:        DefaultVersion,
		ChallengeID:    generateChallengeID(),
		Network:        network,
		Asset:          asset,
		AssetSymbol:    assetSymbol,
		Decimals:       decimals,
		Amount:         amount,
		Payee:          payee,
		Scheme:         DefaultScheme,
		ValidAfter:     now,
		ValidBefore:    now + int64(ttl.Seconds()),
		FacilitatorURL: facilitatorURL,
		ResourceURI:    resourceURI,
	}
}

// ParseCAIP2 parses an EIP-155 CAIP-2 network string into namespace and chain ID.
func ParseCAIP2(network string) (string, int64, error) {
	parts := strings.Split(network, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid CAIP-2 identifier: %q (expected format 'namespace:reference')", network)
	}
	var chainID int64
	_, err := fmt.Sscanf(parts[1], "%d", &chainID)
	if err != nil || chainID <= 0 {
		return "", 0, fmt.Errorf("invalid CAIP-2 chain ID reference: %q", parts[1])
	}
	return parts[0], chainID, nil
}

func generateChallengeID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "ch_" + hex.EncodeToString(b)
}
