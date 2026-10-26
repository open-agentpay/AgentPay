package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SettlementState reflects states defined in specs/payment-state-machine.md §3.
type SettlementState string

const (
	SettlementStateNotStarted SettlementState = "NOT_STARTED"
	SettlementStateSubmitted  SettlementState = "SUBMITTED"
	SettlementStatePending    SettlementState = "PENDING"
	SettlementStateConfirmed  SettlementState = "CONFIRMED"
	SettlementStateFailed     SettlementState = "FAILED"
	SettlementStateReorged    SettlementState = "REORGED"
)

// PaymentChallenge represents the structured 402 challenge issued to the buyer.
type PaymentChallenge struct {
	Version        string                 `json:"version"`
	ChallengeID    string                 `json:"challengeId"`
	Network        string                 `json:"network"` // CAIP-2, e.g. eip155:84532 or eip155:421614
	Asset          string                 `json:"asset"`   // Token contract address (e.g. USDC)
	AssetSymbol    string                 `json:"assetSymbol,omitempty"`
	Decimals       int                    `json:"decimals"`
	Amount         string                 `json:"amount"` // Base units integer string, e.g. "50000"
	Payee          string                 `json:"payee"`  // Payee treasury address
	Scheme         string                 `json:"scheme"` // EIP-3009
	ValidAfter     int64                  `json:"validAfter"`
	ValidBefore    int64                  `json:"validBefore"`
	FacilitatorURL string                 `json:"facilitatorUrl,omitempty"`
	ResourceURI    string                 `json:"resourceUri,omitempty"`
	Extensions     map[string]interface{} `json:"extensions,omitempty"`
}

// SignatureData holds components of the EIP-712 / ECDSA signature.
type SignatureData struct {
	V          uint8  `json:"v"`
	R          string `json:"r"`
	S          string `json:"s"`
	Serialized string `json:"serialized,omitempty"`
	Compact    string `json:"compact,omitempty"`
}

// UnmarshalJSON supports signature as either an object or a hex string.
func (s *SignatureData) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var hexStr string
		if err := json.Unmarshal(data, &hexStr); err != nil {
			return err
		}
		s.Serialized = strings.TrimPrefix(hexStr, "0x")
		s.Compact = "0x" + s.Serialized
		return nil
	}

	type Alias SignatureData
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*s = SignatureData(alias)
	return nil
}

// PaymentAuthorization represents the client EIP-712 payment authorization payload.
type PaymentAuthorization struct {
	Version     string        `json:"version"`
	ChallengeID string        `json:"challengeId"`
	Payer       string        `json:"payer"` // EVM 0x address
	Scheme      string        `json:"scheme"`
	Nonce       string        `json:"nonce"` // 32-byte hex nonce
	ValidAfter  int64         `json:"validAfter"`
	ValidBefore int64         `json:"validBefore"`
	Signature   SignatureData `json:"signature"`
}

// VerifyRequest is the payload for POST /v1/verify.
type VerifyRequest struct {
	Challenge     PaymentChallenge     `json:"challenge"`
	Authorization PaymentAuthorization `json:"authorization"`
}

// VerifyResponse is returned by POST /v1/verify on success.
// It includes aliases for backwards compatibility with both Go Gateway and Python Facilitator Client.
type VerifyResponse struct {
	Valid             bool   `json:"valid"`
	VerifiedPayer     string `json:"verifiedPayer"`
	Payer             string `json:"payer,omitempty"`
	VerifiedAt        int64  `json:"verifiedAt"`
	VerificationToken string `json:"verificationToken"`
}

// SettleRequest is the payload for POST /v1/settle.
type SettleRequest struct {
	VerificationToken string               `json:"verificationToken"`
	Authorization     PaymentAuthorization `json:"authorization"`
	IdempotencyKey    string               `json:"idempotencyKey,omitempty"`
}

// SettleResponse is returned by POST /v1/settle (HTTP 202 Accepted).
type SettleResponse struct {
	SettlementID  string          `json:"settlementId"`
	Status        SettlementState `json:"status"`
	QueuePosition int             `json:"queuePosition,omitempty"`
	TxHash        string          `json:"txHash,omitempty"`
}

// SettlementStatus is returned by GET /v1/settle/{id}.
type SettlementStatus struct {
	SettlementID string          `json:"settlementId"`
	Status       SettlementState `json:"status"`
	TxHash       string          `json:"txHash,omitempty"`
	Error        string          `json:"error,omitempty"`
	CreatedAt    int64           `json:"createdAt"`
	UpdatedAt    int64           `json:"updatedAt"`
}

// ErrorResponse standardizes error responses.
type ErrorResponse struct {
	Valid     bool   `json:"valid"`
	Error     string `json:"error"`
	ErrorCode string `json:"errorCode,omitempty"`
	Message   string `json:"message"`
}

func (e *ErrorResponse) String() string {
	return fmt.Sprintf("[%s] %s", e.Error, e.Message)
}
