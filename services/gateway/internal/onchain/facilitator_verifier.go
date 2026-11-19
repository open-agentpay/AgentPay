package onchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/open-agentpay/agent-core-sdk/gateway/x402"
)

const (
	ErrCodeFacilitatorUnavailable = "FACILITATOR_UNAVAILABLE"
)

// FacilitatorVerifier executes verification by delegating to the Facilitator service.
type FacilitatorVerifier struct {
	facilitatorURL string
	apiKey         string
	client         *http.Client
}

// NewFacilitatorVerifier creates a verifier backed by Facilitator /v1/verify.
func NewFacilitatorVerifier(url string, timeout time.Duration) *FacilitatorVerifier {
	return NewFacilitatorVerifierWithAuth(url, "", timeout)
}

// NewFacilitatorVerifierWithAuth creates an authenticated verifier backed by Facilitator.
func NewFacilitatorVerifierWithAuth(url, apiKey string, timeout time.Duration) *FacilitatorVerifier {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("AGENTPAY_FACILITATOR_API_KEY"))
	}
	return &FacilitatorVerifier{
		facilitatorURL: strings.TrimRight(url, "/"),
		apiKey:         strings.TrimSpace(apiKey),
		client:         &http.Client{Timeout: timeout},
	}
}

// SetAPIKey configures the API key used to authenticate against Facilitator endpoints.
func (fv *FacilitatorVerifier) SetAPIKey(apiKey string) {
	if fv != nil {
		fv.apiKey = strings.TrimSpace(apiKey)
	}
}

// SetHTTPClient configures the underlying HTTP client (e.g. for testing with custom RoundTripper).
func (fv *FacilitatorVerifier) SetHTTPClient(client *http.Client) {
	if fv != nil && client != nil {
		fv.client = client
	}
}

func (fv *FacilitatorVerifier) applyAuthHeaders(req *http.Request) {
	if fv != nil && fv.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+fv.apiKey)
		req.Header.Set("X-Facilitator-Key", fv.apiKey)
	}
}

func (fv *FacilitatorVerifier) VerifyPayment(ctx context.Context, ch *x402.PaymentChallenge, auth *x402.PaymentAuthorization) (*x402.VerificationResult, error) {
	if auth == nil || ch == nil {
		return &x402.VerificationResult{Valid: false, ErrorCode: x402.ErrCodeSignatureInvalid, Message: "nil payload"}, nil
	}

	// 1. Static timestamp bounds check
	now := time.Now().Unix()
	if now < auth.ValidAfter || now > auth.ValidBefore {
		return &x402.VerificationResult{
			Valid:     false,
			ErrorCode: x402.ErrCodeSignatureExpired,
			Message:   fmt.Sprintf("authorization expired or not yet valid (now=%d, validAfter=%d, validBefore=%d)", now, auth.ValidAfter, auth.ValidBefore),
		}, nil
	}

	// 2. Challenge ID correlation check
	if auth.ChallengeID != ch.ChallengeID {
		return &x402.VerificationResult{
			Valid:     false,
			ErrorCode: x402.ErrCodeSignatureInvalid,
			Message:   fmt.Sprintf("challengeId mismatch: auth=%s, challenge=%s", auth.ChallengeID, ch.ChallengeID),
		}, nil
	}

	// 3. Delegate signature verification to Facilitator /v1/verify
	reqBody := map[string]interface{}{
		"challenge":     ch,
		"authorization": auth,
	}
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode verification request: %w", err)
	}

	endpoint := fv.facilitatorURL + "/v1/verify"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to construct facilitator request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	fv.applyAuthHeaders(req)

	resp, err := fv.client.Do(req)
	if err != nil {
		return &x402.VerificationResult{
			Valid:     false,
			ErrorCode: ErrCodeFacilitatorUnavailable,
			Message:   fmt.Sprintf("facilitator connection error: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read facilitator response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var verifyResp struct {
			Valid             bool   `json:"valid"`
			Payer             string `json:"payer"`
			VerifiedPayer     string `json:"verifiedPayer"`
			VerificationToken string `json:"verificationToken"`
			ErrorCode         string `json:"errorCode"`
			Message           string `json:"message"`
		}
		if err := json.Unmarshal(bodyBytes, &verifyResp); err != nil {
			return nil, fmt.Errorf("failed to decode facilitator verification response: %w", err)
		}
		payer := verifyResp.VerifiedPayer
		if payer == "" {
			payer = verifyResp.Payer
		}
		return &x402.VerificationResult{
			Valid:             verifyResp.Valid,
			Payer:             payer,
			VerifiedPayer:     payer,
			VerificationToken: verifyResp.VerificationToken,
			ErrorCode:         verifyResp.ErrorCode,
			Message:           verifyResp.Message,
		}, nil
	}

	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(bodyBytes, &errResp)

	errCode := x402.ErrCodeSignatureInvalid
	if resp.StatusCode >= 500 {
		errCode = ErrCodeFacilitatorUnavailable
	}

	msg := errResp.Message
	if msg == "" {
		msg = fmt.Sprintf("facilitator rejected verification with HTTP %d", resp.StatusCode)
	}

	return &x402.VerificationResult{
		Valid:     false,
		ErrorCode: errCode,
		Message:   msg,
	}, nil
}

// SettlePayment delegates asynchronous settlement to Facilitator POST /v1/settle.
func (fv *FacilitatorVerifier) SettlePayment(ctx context.Context, vToken string, auth *x402.PaymentAuthorization, idemKey string) (*x402.SettleResult, error) {
	if auth == nil {
		return nil, fmt.Errorf("payment authorization cannot be nil")
	}

	reqBody := map[string]interface{}{
		"verificationToken": vToken,
		"authorization":     auth,
		"idempotencyKey":    idemKey,
	}
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode settlement request: %w", err)
	}

	endpoint := fv.facilitatorURL + "/v1/settle"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to construct facilitator settle request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	fv.applyAuthHeaders(req)

	resp, err := fv.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facilitator connection error during settlement: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read facilitator settle response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		var errResp struct {
			Code    string `json:"code"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(bodyBytes, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = errResp.Error
		}
		if msg == "" {
			msg = fmt.Sprintf("facilitator rejected settlement with HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		}
		return nil, fmt.Errorf("facilitator settle failed (%d): %s", resp.StatusCode, msg)
	}

	var settleResp struct {
		SettlementID string `json:"settlementId"`
		Status       string `json:"status"`
		TxHash       string `json:"txHash"`
	}
	if err := json.Unmarshal(bodyBytes, &settleResp); err != nil {
		return nil, fmt.Errorf("failed to decode facilitator settle response: %w", err)
	}

	return &x402.SettleResult{
		SettlementID: settleResp.SettlementID,
		Status:       settleResp.Status,
		TxHash:       settleResp.TxHash,
	}, nil
}

// GetSettlementStatus queries settlement receipt from Facilitator GET /v1/settle/{id}.
func (fv *FacilitatorVerifier) GetSettlementStatus(ctx context.Context, settlementID string) (*x402.SettlementStatus, error) {
	if settlementID == "" {
		return nil, fmt.Errorf("settlement ID cannot be empty")
	}

	endpoint := fmt.Sprintf("%s/v1/settle/%s", fv.facilitatorURL, settlementID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct settlement status request: %w", err)
	}
	fv.applyAuthHeaders(req)

	resp, err := fv.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facilitator connection error during settlement status query: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read settlement status response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facilitator status query failed (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var raw struct {
		SettlementID string `json:"settlementId"`
		Status       string `json:"status"`
		TxHash       string `json:"txHash,omitempty"`
		Error        string `json:"error,omitempty"`
		ErrorMessage string `json:"errorMessage,omitempty"`
	}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode settlement status response: %w", err)
	}

	errMsg := raw.ErrorMessage
	if errMsg == "" {
		errMsg = raw.Error
	}

	return &x402.SettlementStatus{
		SettlementID: raw.SettlementID,
		Status:       raw.Status,
		TxHash:       raw.TxHash,
		ErrorMessage: errMsg,
	}, nil
}

// GetSettlementStatusByNonce queries settlement status by nonce from Facilitator GET /v1/settle/lookup.
func (fv *FacilitatorVerifier) GetSettlementStatusByNonce(ctx context.Context, network, asset, payer, nonce string) (*x402.SettlementStatus, error) {
	endpoint := fmt.Sprintf("%s/v1/settle/lookup?network=%s&asset=%s&payer=%s&nonce=%s",
		fv.facilitatorURL,
		url.QueryEscape(network),
		url.QueryEscape(asset),
		url.QueryEscape(payer),
		url.QueryEscape(nonce),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct settlement lookup request: %w", err)
	}
	fv.applyAuthHeaders(req)

	resp, err := fv.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facilitator connection error during settlement lookup: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read settlement lookup response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facilitator lookup failed (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var raw struct {
		SettlementID string `json:"settlementId"`
		Status       string `json:"status"`
		TxHash       string `json:"txHash,omitempty"`
		Error        string `json:"error,omitempty"`
		ErrorMessage string `json:"errorMessage,omitempty"`
	}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode settlement lookup response: %w", err)
	}

	errMsg := raw.ErrorMessage
	if errMsg == "" {
		errMsg = raw.Error
	}

	return &x402.SettlementStatus{
		SettlementID: raw.SettlementID,
		Status:       raw.Status,
		TxHash:       raw.TxHash,
		ErrorMessage: errMsg,
	}, nil
}

var _ x402.PaymentVerifier = (*FacilitatorVerifier)(nil)
var _ x402.AsyncSettlementVerifier = (*FacilitatorVerifier)(nil)

// FacilitatorConfig defines configuration for the on-chain settlement facilitator.
type FacilitatorConfig struct {
	URL            string  `yaml:"url"`
	APIKey         string  `yaml:"api_key"`
	TimeoutSeconds float64 `yaml:"timeout_seconds"`
}

type onchainGatewayConfigFile struct {
	Facilitator FacilitatorConfig `yaml:"facilitator"`
}

// LoadFacilitatorConfig loads facilitator configuration from config YAML file and environment variables.
func LoadFacilitatorConfig(path string) FacilitatorConfig {
	var fileCfg onchainGatewayConfigFile
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = yaml.Unmarshal(data, &fileCfg)
		}
	}
	cfg := fileCfg.Facilitator
	if u := os.Getenv("AGENTPAY_FACILITATOR_URL"); u != "" {
		cfg.URL = u
	}
	if k := os.Getenv("AGENTPAY_FACILITATOR_API_KEY"); k != "" {
		cfg.APIKey = k
	}
	return cfg
}
