package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/open-agentpay/agentpay-facilitator-go/internal/models"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	errCode := "INTERNAL_SERVER_ERROR"
	var details map[string]interface{}

	switch {
	case errors.Is(err, service.ErrUnauthorized):
		status = http.StatusUnauthorized
		errCode = "UNAUTHORIZED"
	case errors.Is(err, service.ErrPayeeNotAllowed):
		status = http.StatusForbidden
		errCode = "PAYEE_NOT_ALLOWED"
	case errors.Is(err, service.ErrInsufficientTokenBalance):
		status = http.StatusPaymentRequired
		errCode = "INSUFFICIENT_TOKEN_BALANCE"
		var balErr *service.InsufficientTokenBalanceError
		if errors.As(err, &balErr) {
			details = map[string]interface{}{
				"requiredAmount":   balErr.RequiredAmount,
				"availableBalance": balErr.AvailableBalance,
				"inFlightAmount":   balErr.InFlightAmount,
				"network":          balErr.Network,
				"asset":            balErr.Asset,
			}
		}
	case errors.Is(err, service.ErrInsufficientGasBalance):
		status = http.StatusServiceUnavailable
		errCode = "INSUFFICIENT_FACILITATOR_GAS"
	case errors.Is(err, service.ErrRPCUnavailable):
		status = http.StatusServiceUnavailable
		errCode = "FACILITATOR_UNAVAILABLE"
	case errors.Is(err, service.ErrNonceAlreadyUsed):
		status = http.StatusConflict
		errCode = "NONCE_ALREADY_USED"
	case errors.Is(err, service.ErrChallengeExpired):
		status = http.StatusGone
		errCode = "CHALLENGE_EXPIRED"
	case errors.Is(err, service.ErrInvalidSignature):
		status = http.StatusUnauthorized
		errCode = "INVALID_SIGNATURE"
	case errors.Is(err, service.ErrInvalidRequest):
		status = http.StatusBadRequest
		errCode = "INVALID_REQUEST"
	case errors.Is(err, service.ErrSettlementNotFound):
		status = http.StatusNotFound
		errCode = "SETTLEMENT_NOT_FOUND"
	}

	writeJSON(w, status, &models.ErrorResponse{
		Valid:     false,
		Error:     errCode,
		ErrorCode: errCode,
		Message:   err.Error(),
		Details:   details,
	})
}

// handleVerify serves POST /v1/verify
func handleVerify(svc *service.FacilitatorService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, service.ErrInvalidRequest)
			return
		}

		resp, err := svc.Verify(r.Context(), &req)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// handleSettle serves POST /v1/settle
func handleSettle(svc *service.FacilitatorService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.SettleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, service.ErrInvalidRequest)
			return
		}

		resp, err := svc.Settle(r.Context(), &req)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusAccepted, resp)
	}
}

// handleGetSettlement serves GET /v1/settle/{id}
func handleGetSettlement(svc *service.FacilitatorService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/v1/settle/")
		if id == "" {
			writeError(w, service.ErrInvalidRequest)
			return
		}

		status, err := svc.GetSettlement(id)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, status)
	}
}

// handleSettlementLookup serves GET /v1/settle/lookup?network=...&asset=...&payer=...&nonce=...
func handleSettlementLookup(svc *service.FacilitatorService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query()
		network := strings.TrimSpace(q.Get("network"))
		asset := strings.TrimSpace(q.Get("asset"))
		payer := strings.TrimSpace(q.Get("payer"))
		nonce := strings.TrimSpace(q.Get("nonce"))

		if network == "" || asset == "" || payer == "" || nonce == "" {
			writeError(w, service.ErrInvalidRequest)
			return
		}

		status, err := svc.GetSettlementByNonce(network, asset, payer, nonce)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, status)
	}
}

// handleNonceCheck serves GET /v1/nonce
func handleNonceCheck(svc *service.FacilitatorService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query()
		network := q.Get("network")
		asset := q.Get("asset")
		payer := q.Get("payer")
		nonce := q.Get("nonce")

		used := svc.IsNonceUsed(network, asset, payer, nonce)
		writeJSON(w, http.StatusOK, map[string]bool{"isUsed": used})
	}
}

// handleHealth serves GET /health and GET /v1/health
func handleHealth(svc *service.FacilitatorService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":             "ok",
			"time":               svc.Now(),
			"service":            "agentpay-facilitator-go",
			"facilitatorAddress": svc.FacilitatorAddress().Hex(),
		})
	}
}

// handleGasEstimate serves GET /v1/gas/estimate
func handleGasEstimate(svc *service.FacilitatorService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query()
		network := strings.TrimSpace(q.Get("network"))
		asset := strings.TrimSpace(q.Get("asset"))
		slippageStr := strings.TrimSpace(q.Get("slippage"))

		var slippage float64 = 10.0
		if slippageStr != "" {
			if s, err := strconv.ParseFloat(slippageStr, 64); err == nil && s >= 0 {
				slippage = s
			}
		}

		resp, err := svc.EstimateGasSurcharge(r.Context(), network, asset, slippage)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// handleBatchSettle serves POST /v1/settle/batch
func handleBatchSettle(svc *service.FacilitatorService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.BatchSettleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, service.ErrInvalidRequest)
			return
		}

		resp, err := svc.BatchSettle(r.Context(), &req)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusAccepted, resp)
	}
}
