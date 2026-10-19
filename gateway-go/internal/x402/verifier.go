package x402

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// VerificationResult contains cryptographic verification output.
type VerificationResult struct {
	Valid     bool   `json:"valid"`
	Payer     string `json:"payer,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
	Message   string `json:"message,omitempty"`
}

// PaymentVerifier verifies EIP-712 / EIP-3009 authorizations.
type PaymentVerifier interface {
	VerifyPayment(ctx context.Context, ch *PaymentChallenge, auth *PaymentAuthorization) (*VerificationResult, error)
}

// FacilitatorVerifier executes verification by delegating to the Facilitator service.
type FacilitatorVerifier struct {
	facilitatorURL string
	client         *http.Client
}

// NewFacilitatorVerifier creates a verifier backed by Facilitator /v1/verify.
func NewFacilitatorVerifier(url string, timeout time.Duration) *FacilitatorVerifier {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &FacilitatorVerifier{
		facilitatorURL: url,
		client:         &http.Client{Timeout: timeout},
	}
}

func (fv *FacilitatorVerifier) VerifyPayment(ctx context.Context, ch *PaymentChallenge, auth *PaymentAuthorization) (*VerificationResult, error) {
	if auth == nil || ch == nil {
		return &VerificationResult{Valid: false, ErrorCode: ErrCodeSignatureInvalid, Message: "nil payload"}, nil
	}

	// 1. Static timestamp bounds check
	now := time.Now().Unix()
	if now < auth.ValidAfter || now > auth.ValidBefore {
		return &VerificationResult{
			Valid:     false,
			ErrorCode: ErrCodeSignatureExpired,
			Message:   fmt.Sprintf("authorization expired or not yet valid (now=%d, validAfter=%d, validBefore=%d)", now, auth.ValidAfter, auth.ValidBefore),
		}, nil
	}

	// 2. Challenge ID correlation check
	if auth.ChallengeID != ch.ChallengeID {
		return &VerificationResult{
			Valid:     false,
			ErrorCode: ErrCodeSignatureInvalid,
			Message:   fmt.Sprintf("challengeId mismatch: auth=%s, challenge=%s", auth.ChallengeID, ch.ChallengeID),
		}, nil
	}

	// 3. Delegate signature verification to Facilitator /v1/verify
	reqBody, _ := json.Marshal(map[string]interface{}{
		"challenge":     ch,
		"authorization": auth,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", fv.facilitatorURL+"/v1/verify", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create facilitator request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := fv.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facilitator verify call failed: %w", err)
	}
	defer resp.Body.Close()

	var result VerificationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode facilitator response: %w", err)
	}

	return &result, nil
}

// MockPaymentVerifier enables deterministic testing without a live facilitator.
type MockPaymentVerifier struct {
	ShouldPass bool
	Payer      string
	ErrorCode  string
}

func (m *MockPaymentVerifier) VerifyPayment(ctx context.Context, ch *PaymentChallenge, auth *PaymentAuthorization) (*VerificationResult, error) {
	now := time.Now().Unix()
	if now > auth.ValidBefore {
		return &VerificationResult{
			Valid:     false,
			ErrorCode: ErrCodeSignatureExpired,
			Message:   "authorization has expired",
		}, nil
	}
	if !m.ShouldPass {
		errCode := m.ErrorCode
		if errCode == "" {
			errCode = ErrCodeSignatureInvalid
		}
		return &VerificationResult{
			Valid:     false,
			ErrorCode: errCode,
			Message:   "mock signature verification failed",
		}, nil
	}
	payer := m.Payer
	if payer == "" {
		payer = auth.Payer
	}
	return &VerificationResult{
		Valid: true,
		Payer: payer,
	}, nil
}

// NonceStore tracks spent nonces to prevent replay attacks.
type NonceStore interface {
	CheckAndRecordNonce(network, asset, payer, nonce string) (bool, error)
}

// MemoryNonceStore provides thread-safe in-memory anti-replay tracking.
type MemoryNonceStore struct {
	mu     sync.RWMutex
	nonces map[string]bool
}

// NewMemoryNonceStore creates an in-memory nonce store.
func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{
		nonces: make(map[string]bool),
	}
}

func (s *MemoryNonceStore) CheckAndRecordNonce(network, asset, payer, nonce string) (bool, error) {
	key := fmt.Sprintf("%s:%s:%s:%s", network, asset, payer, nonce)
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nonces[key] {
		return false, nil // Already used!
	}
	s.nonces[key] = true
	return true, nil
}
