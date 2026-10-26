package x402

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// VerificationResult contains cryptographic verification output.
type VerificationResult struct {
	Valid             bool   `json:"valid"`
	Payer             string `json:"payer,omitempty"`
	VerifiedPayer     string `json:"verifiedPayer,omitempty"`
	VerificationToken string `json:"verificationToken,omitempty"`
	ErrorCode         string `json:"errorCode,omitempty"`
	Message           string `json:"message,omitempty"`
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
	slog.Info("send facilitatorURL verify url ", slog.String("facilitatorURL", fv.facilitatorURL))
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

	if resp.StatusCode != http.StatusOK {
		var errPayload struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errPayload)
		errCode := errPayload.Error.Code
		if errCode == "" {
			errCode = errPayload.Code
		}
		if errCode == "" {
			errCode = ErrCodeSignatureInvalid
		}
		errMsg := errPayload.Error.Message
		if errMsg == "" {
			errMsg = errPayload.Message
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("facilitator rejected verification with HTTP %d", resp.StatusCode)
		}
		return &VerificationResult{
			Valid:     false,
			ErrorCode: errCode,
			Message:   errMsg,
		}, nil
	}

	var result VerificationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode facilitator response: %w", err)
	}

	if result.Payer == "" && result.VerifiedPayer != "" {
		result.Payer = result.VerifiedPayer
	}

	return &result, nil
}

// SettleResult contains the asynchronous settlement output from Facilitator.
type SettleResult struct {
	SettlementID string `json:"settlementId"`
	Status       string `json:"status"`
	TxHash       string `json:"txHash"`
}

// SettlePayment dispatches settlement to Facilitator POST /v1/settle to trigger on-chain transaction broadcast.
func (fv *FacilitatorVerifier) SettlePayment(ctx context.Context, vToken string, auth *PaymentAuthorization, idemKey string) (*SettleResult, error) {
	if fv == nil || fv.facilitatorURL == "" {
		return nil, fmt.Errorf("facilitator url not configured")
	}
	reqBody, err := json.Marshal(map[string]interface{}{
		"verificationToken": vToken,
		"authorization":     auth,
		"idempotencyKey":    idemKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal settle request: %w", err)
	}

	slog.Info("dispatching async settlement to facilitator",
		slog.String("facilitator_url", fv.facilitatorURL),
		slog.String("verification_token", vToken),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", fv.facilitatorURL+"/v1/settle", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create facilitator settle request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := fv.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facilitator settle call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var errResp struct {
			Error     string `json:"error"`
			ErrorCode string `json:"errorCode"`
			Message   string `json:"message"`
		}
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("facilitator settle rejected with HTTP %d [%s]: %s", resp.StatusCode, errResp.ErrorCode, errResp.Message)
		}
		return nil, fmt.Errorf("facilitator settle rejected with HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res SettleResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode facilitator settle response: %w", err)
	}
	return &res, nil
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
