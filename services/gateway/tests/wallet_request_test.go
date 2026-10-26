package tests

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

var walletTestNonce atomic.Uint64

// serveWalletRequest exercises the public two-step wallet authorization flow:
// request a priced challenge, then retry the identical request with its binding.
func serveWalletRequest(t *testing.T, handler http.Handler, recorder *httptest.ResponseRecorder, req *http.Request) {
	t.Helper()
	serveWalletRequestWith(t, recorder, req, func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) })
}

func serveWalletRequestWith(t *testing.T, recorder *httptest.ResponseRecorder, req *http.Request, serve func(http.ResponseWriter, *http.Request)) {
	t.Helper()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read wallet request body: %v", err)
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	challengeRecorder := httptest.NewRecorder()
	serve(challengeRecorder, req)
	if challengeRecorder.Code != http.StatusPaymentRequired || challengeRecorder.Header().Get("PAYMENT-REQUIRED") == "" {
		*recorder = *challengeRecorder
		return
	}

	var response struct {
		Error struct {
			Details struct {
				Challenge struct {
					Version     string `json:"version"`
					ChallengeID string `json:"challengeId"`
					Scheme      string `json:"scheme"`
					Amount      string `json:"amount"`
					ValidAfter  int64  `json:"validAfter"`
					ValidBefore int64  `json:"validBefore"`
					ResourceURI string `json:"resourceUri"`
					Extensions  struct {
						RequestBinding string `json:"requestBinding"`
					} `json:"extensions"`
				} `json:"challenge"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(challengeRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode wallet challenge: %v", err)
	}
	challenge := response.Error.Details.Challenge
	if challenge.ChallengeID == "" || challenge.Extensions.RequestBinding == "" {
		t.Fatalf("wallet challenge response missing authorization fields: %s", challengeRecorder.Body.String())
	}
	bodyHash := sha256.Sum256(body)
	canonical := strings.ToUpper(req.Method) + "\n" + req.URL.EscapedPath() + "\n" + req.URL.RawQuery + "\n" + hex.EncodeToString(bodyHash[:])
	bindingHash := sha256.Sum256([]byte(canonical))
	binding := "sha256:" + hex.EncodeToString(bindingHash[:])
	if binding != challenge.Extensions.RequestBinding {
		t.Fatalf("request binding mismatch: locally computed %s, challenge returned %s", binding, challenge.Extensions.RequestBinding)
	}
	nonce := fmt.Sprintf("0x%064x", walletTestNonce.Add(1))
	authorization, err := json.Marshal(map[string]interface{}{
		"version": challenge.Version, "challengeId": challenge.ChallengeID, "scheme": challenge.Scheme,
		"nonce": nonce, "validAfter": challenge.ValidAfter, "validBefore": challenge.ValidBefore,
		"amount": challenge.Amount, "resourceUri": challenge.ResourceURI, "requestBinding": binding,
	})
	if err != nil {
		t.Fatalf("encode wallet authorization: %v", err)
	}
	retry := req.Clone(req.Context())
	retry.Body = io.NopCloser(bytes.NewReader(body))
	retry.Header = req.Header.Clone()
	retry.Header.Set("PAYMENT-SIGNATURE", string(authorization))
	serve(recorder, retry)
}
