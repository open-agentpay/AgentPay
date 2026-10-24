package server

import (
	"encoding/json"
	"errors"
	"net/http"
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

	switch {
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
