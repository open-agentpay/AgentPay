package server

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-agentpay/agentpay-facilitator-go/internal/config"
	agentevm "github.com/open-agentpay/agentpay-facilitator-go/internal/evm"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/models"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/service"
)

func TestHTTPServerEndpoints(t *testing.T) {
	cfg := config.DefaultConfig()
	mockClient := agentevm.NewMockClient(big.NewInt(84532))

	currentTime := int64(1729000100)
	svc, err := service.NewFacilitatorService(service.Options{
		Clock:            func() int64 { return currentTime },
		EVMClient:        mockClient,
		DefaultChainID:   big.NewInt(84532),
		DefaultTokenAddr: "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		AutoPollReceipts: false,
	})
	if err != nil {
		t.Fatalf("failed to initialize service: %v", err)
	}

	srv := New(cfg, svc)
	handler := srv.Handler()

	// 1. Health check
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /health, got %d", rec.Code)
	}

	// 2. POST /v1/verify
	ch := models.PaymentChallenge{
		Version:     "2.0",
		ChallengeID: "ch_http_test",
		Network:     "eip155:84532",
		Asset:       "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
		AssetSymbol: "USDC",
		Decimals:    6,
		Amount:      "100000",
		Payee:       "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		Scheme:      "EIP-3009",
		ValidAfter:  1729000000,
		ValidBefore: 1729000300,
	}

	auth := models.PaymentAuthorization{
		Version:     "2.0",
		ChallengeID: "ch_http_test",
		Payer:       "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		Scheme:      "EIP-3009",
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

	verifyBody, _ := json.Marshal(models.VerifyRequest{
		Challenge:     ch,
		Authorization: auth,
	})

	vReq := httptest.NewRequest(http.MethodPost, "/v1/verify", bytes.NewBuffer(verifyBody))
	vRec := httptest.NewRecorder()
	handler.ServeHTTP(vRec, vReq)

	if vRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /v1/verify, got %d (body: %s)", vRec.Code, vRec.Body.String())
	}

	var verifyResult models.VerifyResponse
	if err := json.NewDecoder(vRec.Body).Decode(&verifyResult); err != nil {
		t.Fatalf("failed to decode verify response: %v", err)
	}
	if !verifyResult.Valid || verifyResult.VerificationToken == "" {
		t.Fatalf("expected valid verifyResult, got %+v", verifyResult)
	}

	// 3. POST /v1/settle
	settleBody, _ := json.Marshal(models.SettleRequest{
		VerificationToken: verifyResult.VerificationToken,
		Authorization:     auth,
		IdempotencyKey:    "test_idem_http",
	})

	sReq := httptest.NewRequest(http.MethodPost, "/v1/settle", bytes.NewBuffer(settleBody))
	sRec := httptest.NewRecorder()
	handler.ServeHTTP(sRec, sReq)

	if sRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted from /v1/settle, got %d (body: %s)", sRec.Code, sRec.Body.String())
	}

	var settleResult models.SettleResponse
	if err := json.NewDecoder(sRec.Body).Decode(&settleResult); err != nil {
		t.Fatalf("failed to decode settle response: %v", err)
	}

	// 4. GET /v1/settle/{id}
	stReq := httptest.NewRequest(http.MethodGet, "/v1/settle/"+settleResult.SettlementID, nil)
	stRec := httptest.NewRecorder()
	handler.ServeHTTP(stRec, stReq)

	if stRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /v1/settle/{id}, got %d", stRec.Code)
	}

	var stStatus models.SettlementStatus
	if err := json.NewDecoder(stRec.Body).Decode(&stStatus); err != nil {
		t.Fatalf("failed to decode settlement status: %v", err)
	}
	if stStatus.SettlementID != settleResult.SettlementID {
		t.Fatalf("mismatched settlement status ID")
	}

	// 5. GET /v1/nonce check
	nReq := httptest.NewRequest(http.MethodGet, "/v1/nonce?network="+ch.Network+"&asset="+ch.Asset+"&payer="+auth.Payer+"&nonce="+auth.Nonce, nil)
	nRec := httptest.NewRecorder()
	handler.ServeHTTP(nRec, nReq)

	var nonceResult map[string]bool
	if err := json.NewDecoder(nRec.Body).Decode(&nonceResult); err != nil {
		t.Fatalf("failed to decode nonce response: %v", err)
	}
	if !nonceResult["isUsed"] {
		t.Fatalf("expected nonce to be reported used")
	}
}
