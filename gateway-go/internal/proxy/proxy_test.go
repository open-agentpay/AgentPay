package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/upstreamauth"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestForwardNormalRequest(t *testing.T) {
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"echo":"` + string(body) + `"}`)),
			Request:    r,
		}
		resp.Header.Set("Content-Type", "application/json")
		resp.Header.Set("X-Upstream-Header", "processed")
		return resp, nil
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:     "rou_test",
		UpstreamURL: "http://upstream.internal:8000/base",
		Status:      "active",
	}

	req := httptest.NewRequest("POST", "/v1/data", bytes.NewBufferString("hello-agentpay"))
	w := httptest.NewRecorder()

	handler.Forward(w, req, route, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hello-agentpay") {
		t.Errorf("expected echo in body, got: %s", w.Body.String())
	}
	if w.Header().Get("X-Upstream-Header") != "processed" {
		t.Errorf("expected X-Upstream-Header preserved")
	}
}

func TestHopByHopAndSpoofedHeadersStripped(t *testing.T) {
	var receivedHeaders http.Header
	var targetPath string

	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		receivedHeaders = r.Header.Clone()
		targetPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:     "rou_test",
		UpstreamURL: "http://upstream.internal:8000/service",
		Status:      "active",
	}

	req := httptest.NewRequest("GET", "/v1/test", nil)
	// Inject spoofed client headers & hop-by-hop headers
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("X-Payer-Address", "0xHACKER_ATTACKER_ADDRESS")
	req.Header.Set("X-Payment-Amount", "9999999999")
	req.Header.Set("X-AgentPay-Spoof", "true")
	req.Header.Set("X-Legit-Client-Header", "keep-me")

	w := httptest.NewRecorder()
	handler.Forward(w, req, route, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify target path mapped properly
	if targetPath != "/service/v1/test" {
		t.Errorf("expected target path /service/v1/test, got %s", targetPath)
	}

	// Verify spoofed headers stripped
	if receivedHeaders.Get("X-Payer-Address") != "" {
		t.Errorf("expected X-Payer-Address stripped, got: %s", receivedHeaders.Get("X-Payer-Address"))
	}
	if receivedHeaders.Get("X-Payment-Amount") != "" {
		t.Errorf("expected X-Payment-Amount stripped, got: %s", receivedHeaders.Get("X-Payment-Amount"))
	}
	if receivedHeaders.Get("X-AgentPay-Spoof") != "" {
		t.Errorf("expected X-AgentPay-Spoof stripped")
	}
	if receivedHeaders.Get("Upgrade") != "" {
		t.Errorf("expected hop-by-hop Upgrade stripped")
	}

	// Legitimate headers preserved
	if receivedHeaders.Get("X-Legit-Client-Header") != "keep-me" {
		t.Errorf("expected X-Legit-Client-Header preserved")
	}
	// Forwarded headers injected
	if receivedHeaders.Get("X-Forwarded-For") == "" {
		t.Errorf("expected X-Forwarded-For injected")
	}
}

func TestUpstreamCredentialIsolationAndPaymentContext(t *testing.T) {
	var capturedAuth string
	var capturedPayer string
	var capturedAmount string

	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedAuth = r.Header.Get("Authorization")
		capturedPayer = r.Header.Get("X-Payer-Address")
		capturedAmount = r.Header.Get("X-Payment-Amount")

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"success":true}`)),
			Request:    r,
		}
		resp.Header.Set("Server", "Internal-Private-Upstream/1.0")
		return resp, nil
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:     "rou_secure",
		UpstreamURL: "http://vllm-cluster.internal:8000",
		Status:      "active",
		UpstreamHeaders: map[string]string{
			"Authorization": "Bearer internal-vllm-secret-key-12345",
			"X-Internal-ID": "backend-node-42",
		},
	}

	pCtx := &PaymentContext{
		PayerAddress:    "0x1234567890123456789012345678901234567890",
		AmountBaseUnits: 5000,
		Asset:           "USDC",
		Network:         "eip155:84532",
		TxHash:          "0xabcdef",
		RequestID:       "req_test_01",
	}

	req := httptest.NewRequest("GET", "/v1/inference", nil)
	w := httptest.NewRecorder()

	handler.Forward(w, req, route, pCtx)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// 1. Upstream received isolated credentials
	if capturedAuth != "Bearer internal-vllm-secret-key-12345" {
		t.Errorf("upstream did not receive isolated credentials, got: %s", capturedAuth)
	}

	// 2. Upstream received verified payment context
	if capturedPayer != "0x1234567890123456789012345678901234567890" {
		t.Errorf("expected payer address injected, got: %s", capturedPayer)
	}
	if capturedAmount != "5000" {
		t.Errorf("expected amount 5000 injected, got: %s", capturedAmount)
	}

	// 3. Client response does NOT leak internal Server info or upstream secrets
	if w.Header().Get("Server") != "" {
		t.Errorf("expected Server header stripped from client response, got: %s", w.Header().Get("Server"))
	}
	if w.Header().Get("Authorization") != "" {
		t.Errorf("upstream Authorization credential leaked into response headers!")
	}
}

func TestMaxBodySizeLimit(t *testing.T) {
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		// Read entire body - this triggers MaxBytesReader if over limit
		_, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:      "rou_limit",
		UpstreamURL:  "http://upstream.internal:8000",
		Status:       "active",
		MaxBodyBytes: 100, // Limit to 100 bytes
	}

	// Send 1000 bytes payload
	largePayload := strings.Repeat("A", 1000)
	req := httptest.NewRequest("POST", "/v1/upload", bytes.NewBufferString(largePayload))
	w := httptest.NewRecorder()

	handler.Forward(w, req, route, nil)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 Request Entity Too Large, got %d, body: %s", w.Code, w.Body.String())
	}

	var errResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["error"] != "REQUEST_ENTITY_TOO_LARGE" {
		t.Errorf("expected error code REQUEST_ENTITY_TOO_LARGE, got %v", errResp["error"])
	}
}

func TestMethodRestriction(t *testing.T) {
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			Request:    r,
		}, nil
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:     "rou_post_only",
		UpstreamURL: "http://upstream.internal:8000",
		Status:      "active",
		Methods:     []string{"POST"},
	}

	// GET request against POST-only route
	req := httptest.NewRequest("GET", "/v1/post-only", nil)
	w := httptest.NewRecorder()

	handler.Forward(w, req, route, nil)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed, got %d", w.Code)
	}
}

func TestUpstreamTimeout(t *testing.T) {
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		// Wait until context is canceled by proxy timeout
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:     "rou_timeout",
		UpstreamURL: "http://upstream.internal:8000",
		Status:      "active",
		TimeoutMS:   50, // 50ms timeout
	}

	req := httptest.NewRequest("GET", "/v1/slow", nil)
	w := httptest.NewRecorder()

	handler.Forward(w, req, route, nil)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 Gateway Timeout, got %d, body: %s", w.Code, w.Body.String())
	}

	var errResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["error"] != "UPSTREAM_TIMEOUT" {
		t.Errorf("expected error code UPSTREAM_TIMEOUT, got %v", errResp["error"])
	}
}

func TestUpstreamBadGateway(t *testing.T) {
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection refused")
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:     "rou_down",
		UpstreamURL: "http://upstream.internal:8000",
		Status:      "active",
	}

	req := httptest.NewRequest("GET", "/v1/down", nil)
	w := httptest.NewRecorder()

	handler.Forward(w, req, route, nil)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway, got %d, body: %s", w.Code, w.Body.String())
	}

	var errResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["error"] != "BAD_GATEWAY" {
		t.Errorf("expected error code BAD_GATEWAY, got %v", errResp["error"])
	}
}

func TestUpstreamQueryPreservedAndMerged(t *testing.T) {
	var receivedRawQuery string
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		receivedRawQuery = r.URL.RawQuery
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:     "rou_weather",
		PathPattern: "/api/v1/simple-weather",
		UpstreamURL: "https://apis.juhe.cn/simpleWeather/query?key=b9b6fd5f9ac9cc4d30229f34f304be4a",
		Status:      "active",
	}

	req := httptest.NewRequest("GET", "/api/v1/simple-weather?city=%E5%8C%97%E4%BA%AC&future_num=", nil)
	w := httptest.NewRecorder()

	handler.Forward(w, req, route, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify both key and client city parameters exist in upstream query
	if !strings.Contains(receivedRawQuery, "key=b9b6fd5f9ac9cc4d30229f34f304be4a") {
		t.Errorf("expected target key preserved in query, got: %s", receivedRawQuery)
	}
	if !strings.Contains(receivedRawQuery, "city=") {
		t.Errorf("expected client query params preserved in query, got: %s", receivedRawQuery)
	}
}

func TestUpstreamAuthQueryInjection(t *testing.T) {
	var receivedRawQuery string
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		receivedRawQuery = r.URL.RawQuery
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	handler := NewHandler(metrics.New(), mockTransport)
	route := &rcu.RouteConfig{
		RouteID:     "rou_weather_auth",
		PathPattern: "/api/v1/simple-weather",
		UpstreamURL: "https://apis.juhe.cn/simpleWeather/query",
		Status:      "active",
		UpstreamSecrets: map[string]string{
			"value": "b9b6fd5f9ac9cc4d30229f34f304be4a",
		},
		UpstreamAuth: &upstreamauth.UpstreamAuthConfig{
			Type: "api_key",
			Params: map[string]interface{}{
				"in":   "query",
				"name": "key",
			},
		},
	}

	req := httptest.NewRequest("GET", "/api/v1/simple-weather?city=%E5%8C%97%E4%BA%AC", nil)
	w := httptest.NewRecorder()

	handler.Forward(w, req, route, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !strings.Contains(receivedRawQuery, "key=b9b6fd5f9ac9cc4d30229f34f304be4a") {
		t.Errorf("expected auth key injected in query, got: %s", receivedRawQuery)
	}
	if !strings.Contains(receivedRawQuery, "city=") {
		t.Errorf("expected client query params preserved, got: %s", receivedRawQuery)
	}
}

