package x402

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/agent"
	"github.com/open-agentpay/agentpay-gateway-go/internal/idempotency"
	"github.com/open-agentpay/agentpay-gateway-go/internal/metrics"
	"github.com/open-agentpay/agentpay-gateway-go/internal/payment"
	"github.com/open-agentpay/agentpay-gateway-go/internal/proxy"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/recovery"
	"github.com/open-agentpay/agentpay-gateway-go/internal/usage"
)

// WalletAccount represents the state of a virtual wallet.
type WalletAccount struct {
	WalletID         string
	TenantID         string
	APIKeyID         string
	ProjectID        string
	AgentID          string
	Status           string // active, frozen, frozen_arrears, closed
	AvailableBalance int64
	HeldBalance      int64
}

// WalletService coordinates balance checks, holds, and accounting.
type WalletService interface {
	GetWalletByAPIKey(ctx context.Context, apiKey string) (*WalletAccount, error)
	HoldFunds(ctx context.Context, walletID string, amount int64, intentID string) (string, error)
	CaptureFunds(ctx context.Context, holdID string) error
	ReleaseFunds(ctx context.Context, holdID string) error
}

// markWalletFundsRecovery keeps a payment operation recoverable when the ledger
// capture or release fails. The outcome can be ambiguous, so callers must not
// emit a terminal capture/release event or success metric.
func (p *PaymentProcessor) markWalletFundsRecovery(
	ctx context.Context,
	tenantID, idempotencyKey string,
	statusCode int,
	reason string,
) {
	if idempotencyKey == "" {
		return
	}
	if err := p.idempotencyStore.MarkRecoveryRequired(ctx, tenantID, idempotencyKey, statusCode, reason); err != nil {
		slog.Error("failed to mark wallet funds recovery required",
			slog.String("tenant_id", tenantID),
			slog.String("idempotency_key", idempotencyKey),
			slog.Any("error", err),
		)
	}
}

// MemoryWalletService is an in-memory implementation for standalone/testing operation.
type MemoryWalletService struct {
	mu       sync.Mutex
	wallets  map[string]*WalletAccount
	apiKeys  map[string]string // apiKey -> walletID
	holds    map[string]int64  // holdID -> amount
	holdWIDs map[string]string // holdID -> walletID
}

// NewMemoryWalletService creates a thread-safe in-memory wallet service.
func NewMemoryWalletService() *MemoryWalletService {
	return &MemoryWalletService{
		wallets:  make(map[string]*WalletAccount),
		apiKeys:  make(map[string]string),
		holds:    make(map[string]int64),
		holdWIDs: make(map[string]string),
	}
}

// RegisterWallet registers an account and associates an API Key.
func (m *MemoryWalletService) RegisterWallet(apiKey string, w *WalletAccount) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wallets[w.WalletID] = w
	m.apiKeys[apiKey] = w.WalletID
}

func (m *MemoryWalletService) GetWalletByAPIKey(ctx context.Context, apiKey string) (*WalletAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	wID, ok := m.apiKeys[apiKey]
	if !ok {
		return nil, fmt.Errorf("invalid API key")
	}
	w, ok := m.wallets[wID]
	if !ok {
		return nil, fmt.Errorf("wallet not found")
	}
	return w, nil
}

func (m *MemoryWalletService) HoldFunds(ctx context.Context, walletID string, amount int64, intentID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.wallets[walletID]
	if !ok {
		return "", fmt.Errorf("wallet not found")
	}
	if w.Status != "active" {
		return "", fmt.Errorf("wallet is %s", w.Status)
	}
	if w.AvailableBalance < amount {
		return "", fmt.Errorf("insufficient available balance")
	}

	w.AvailableBalance -= amount
	w.HeldBalance += amount
	holdID := fmt.Sprintf("hld_%d", time.Now().UnixNano())
	m.holds[holdID] = amount
	m.holdWIDs[holdID] = walletID
	return holdID, nil
}

func (m *MemoryWalletService) CaptureFunds(ctx context.Context, holdID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	amount, ok := m.holds[holdID]
	if !ok {
		return fmt.Errorf("hold not found")
	}
	wID := m.holdWIDs[holdID]
	w := m.wallets[wID]
	if w != nil {
		w.HeldBalance -= amount
	}
	delete(m.holds, holdID)
	delete(m.holdWIDs, holdID)
	return nil
}

func (m *MemoryWalletService) ReleaseFunds(ctx context.Context, holdID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	amount, ok := m.holds[holdID]
	if !ok {
		return fmt.Errorf("hold not found")
	}
	wID := m.holdWIDs[holdID]
	w := m.wallets[wID]
	if w != nil {
		w.HeldBalance -= amount
		w.AvailableBalance += amount
	}
	delete(m.holds, holdID)
	delete(m.holdWIDs, holdID)
	return nil
}

// PaymentProcessor orchestrates dual-path routing, challenge generation, payment validation,
// idempotency enforcement, and anti-rollback failure recovery.
type PaymentProcessor struct {
	verifier           PaymentVerifier
	nonceStore         NonceStore
	walletService      WalletService
	idempotencyStore   idempotency.Store
	recoveryCoord      *recovery.RecoveryCoordinator
	proxyHandler       *proxy.Handler
	metrics            *metrics.Metrics
	agentRegistry      *agent.Registry
	agentEnforcer      *agent.Enforcer
	auditEmitter       agent.AuditEmitter
	usageService       usage.Service
	paymentService     payment.Service
	facilitatorURL     string
	settlementDefaults SettlementDefaults
}

// NewPaymentProcessor creates an initialized PaymentProcessor with full safety defaults.
func NewPaymentProcessor(
	verifier PaymentVerifier,
	nonceStore NonceStore,
	walletService WalletService,
	proxyHandler *proxy.Handler,
	m *metrics.Metrics,
) *PaymentProcessor {
	if m == nil {
		m = metrics.New()
	}
	if nonceStore == nil {
		nonceStore = NewMemoryNonceStore()
	}
	if verifier == nil {
		verifier = &MockPaymentVerifier{ShouldPass: true}
	}
	if walletService == nil {
		walletService = NewMemoryWalletService()
	}
	return &PaymentProcessor{
		verifier:         verifier,
		nonceStore:       nonceStore,
		walletService:    walletService,
		idempotencyStore: idempotency.NewMemoryStore(24 * time.Hour),
		recoveryCoord:    recovery.NewRecoveryCoordinator(),
		proxyHandler:     proxyHandler,
		metrics:          m,
		agentRegistry:    agent.NewRegistry(),
		agentEnforcer:    agent.NewEnforcer(),
		auditEmitter:     agent.NewMemoryAuditEmitter(),
		usageService:     usage.NewMemoryService(),
		paymentService:   payment.NewMemoryService(),
	}
}

// SetPaymentService configures a custom payment correlation service.
func (p *PaymentProcessor) SetPaymentService(svc payment.Service) {
	if svc != nil {
		p.paymentService = svc
	}
}

// GetPaymentService returns the active payment correlation service.
func (p *PaymentProcessor) GetPaymentService() payment.Service {
	return p.paymentService
}

// SetFacilitatorURL configures the Facilitator clearing endpoint URL.
func (p *PaymentProcessor) SetFacilitatorURL(url string) {
	p.facilitatorURL = url
}

// SetVerifier configures a custom payment verifier.
func (p *PaymentProcessor) SetVerifier(verifier PaymentVerifier) {
	if verifier != nil {
		p.verifier = verifier
	}
}

// GetVerifier returns the active payment verifier.
func (p *PaymentProcessor) GetVerifier() PaymentVerifier {
	return p.verifier
}

// GetFacilitatorURL returns the configured Facilitator clearing endpoint URL.
func (p *PaymentProcessor) GetFacilitatorURL() string {
	return p.facilitatorURL
}

// dispatchAsyncSettlement dispatches POST /v1/settle to Facilitator in background to broadcast transaction on-chain.
func (p *PaymentProcessor) dispatchAsyncSettlement(settlementID string, vToken string, auth *PaymentAuthorization, idemKey string) {
	if p.facilitatorURL == "" || vToken == "" || auth == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var fv *FacilitatorVerifier
		if v, ok := p.verifier.(*FacilitatorVerifier); ok {
			fv = v
		} else {
			fv = NewFacilitatorVerifier(p.facilitatorURL, 15*time.Second)
		}

		res, err := fv.SettlePayment(ctx, vToken, auth, idemKey)
		if err != nil {
			slog.Error("async on-chain settlement broadcast failed",
				slog.String("settlement_id", settlementID),
				slog.Any("error", err),
			)
			if p.paymentService != nil {
				_ = p.paymentService.UpdateSettlementStatus(context.Background(), settlementID, payment.SettlementStatusUnknown, "", nil, err.Error())
			}
			return
		}

		slog.Info("on-chain settlement broadcast succeeded",
			slog.String("settlement_id", settlementID),
			slog.String("tx_hash", res.TxHash),
			slog.String("status", res.Status),
		)

		if p.paymentService != nil {
			_ = p.paymentService.UpdateSettlementStatus(context.Background(), settlementID, payment.SettlementStatusConfirmed, res.TxHash, nil, "")
		}
	}()
}

// SetSettlementDefaults configures default blockchain settlement parameters across routes.
func (p *PaymentProcessor) SetSettlementDefaults(defaults SettlementDefaults) {
	p.settlementDefaults = defaults
}

// GetSettlementDefaults returns configured default blockchain settlement parameters.
func (p *PaymentProcessor) GetSettlementDefaults() SettlementDefaults {
	return p.settlementDefaults
}

// SetUsageService configures a custom usage service.
func (p *PaymentProcessor) SetUsageService(svc usage.Service) {
	if svc != nil {
		p.usageService = svc
	}
}

// GetUsageService returns the active usage service.
func (p *PaymentProcessor) GetUsageService() usage.Service {
	return p.usageService
}

// SetAgentRegistry configures a custom agent registry.
func (p *PaymentProcessor) SetAgentRegistry(reg *agent.Registry) {
	if reg != nil {
		p.agentRegistry = reg
	}
}

// GetAgentRegistry returns the active agent registry.
func (p *PaymentProcessor) GetAgentRegistry() *agent.Registry {
	return p.agentRegistry
}

// SetAgentEnforcer configures a custom agent policy enforcer.
func (p *PaymentProcessor) SetAgentEnforcer(enf *agent.Enforcer) {
	if enf != nil {
		p.agentEnforcer = enf
	}
}

// GetAgentEnforcer returns the active agent policy enforcer.
func (p *PaymentProcessor) GetAgentEnforcer() *agent.Enforcer {
	return p.agentEnforcer
}

// SetAuditEmitter configures a custom audit emitter.
func (p *PaymentProcessor) SetAuditEmitter(emitter agent.AuditEmitter) {
	p.auditEmitter = emitter
}

// GetAuditEmitter returns the active audit emitter.
func (p *PaymentProcessor) GetAuditEmitter() agent.AuditEmitter {
	return p.auditEmitter
}

// SetIdempotencyStore sets a custom idempotency storage provider (e.g. PostgreSQL or distributed Redis).
func (p *PaymentProcessor) SetIdempotencyStore(store idempotency.Store) {
	if store != nil {
		p.idempotencyStore = store
	}
}

// SetRecoveryCoordinator sets a custom recovery coordinator.
func (p *PaymentProcessor) SetRecoveryCoordinator(coord *recovery.RecoveryCoordinator) {
	if coord != nil {
		p.recoveryCoord = coord
	}
}

// IdempotencyStore exposes the current store.
func (p *PaymentProcessor) IdempotencyStore() idempotency.Store {
	return p.idempotencyStore
}

// RecoveryCoordinator exposes the current coordinator.
func (p *PaymentProcessor) RecoveryCoordinator() *recovery.RecoveryCoordinator {
	return p.recoveryCoord
}

// WriteCachedResponse streams a previously recorded successful response to the downstream caller.
func WriteCachedResponse(w http.ResponseWriter, rec *idempotency.Record) {
	for k, v := range rec.Headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("X-Cache", "HIT")
	w.WriteHeader(rec.StatusCode)
	_, _ = w.Write(rec.ResponseBody)
}

// HandleRequest processes an incoming request against route payment requirements without snapshot tracking.
func (p *PaymentProcessor) HandleRequest(w http.ResponseWriter, r *http.Request, route *rcu.RouteConfig, requestID string) {
	p.HandleRequestWithSnapshot(w, r, route, nil, nil, requestID)
}

// HandleRequestWithSnapshot processes an incoming request against route pricing, policy rules, and tracks active config version.
func (p *PaymentProcessor) HandleRequestWithSnapshot(
	w http.ResponseWriter,
	r *http.Request,
	route *rcu.RouteConfig,
	policy *rcu.PolicyConfig,
	snap *rcu.ConfigSnapshot,
	requestID string,
) {
	sc := newReqScope(p, r, route, requestID, snap)
	configVersion, configHash := sc.configVersion, sc.configHash
	if snap != nil {
		w.Header().Set("X-Config-Version", configVersion)
	}
	startTime, clientIP := sc.startTime, sc.clientIP

	recordReq := sc.recordReq
	recordUsage := sc.recordUsage
	initIntent := sc.initIntent
	initSettlement := sc.initSettlement
	initFulfillment := sc.initFulfillment
	updateCorrelationOutcome := sc.updateCorrelationOutcome

	// 1. Enforce route-specific or global Policy checks (T2.5)
	if policy != nil && policy.IsActive {
		if policy.MaxAmountPerRequest > 0 && route.PriceBaseUnits > policy.MaxAmountPerRequest {
			p.metrics.IncPaymentsFailed()
			recordReq(route.TenantID, "prj_default", "", "", "", http.StatusForbidden, ErrCodePolicyMaxPerTx, false)
			WriteProtocolError(w, ErrCodePolicyMaxPerTx, requestID, map[string]interface{}{
				"route_id":               route.RouteID,
				"price_base_units":       route.PriceBaseUnits,
				"max_amount_per_request": policy.MaxAmountPerRequest,
				"policy_id":              policy.PolicyID,
			})
			return
		}
	}

	// Free route bypasses payments
	if route.PriceBaseUnits == 0 {
		pCtx := &proxy.PaymentContext{
			RequestID:     requestID,
			ConfigVersion: configVersion,
			ConfigHash:    configHash,
		}
		resp, err := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
		if err != nil {
			slog.Error("forwarding free route request failed", slog.String("request_id", requestID), slog.String("route_id", route.RouteID), slog.Any("error", err))
		}
		statusCode := http.StatusBadGateway
		if resp != nil {
			statusCode = resp.StatusCode
		}
		recordReq(route.TenantID, "prj_default", "", "", "", statusCode, "", false)
		return
	}

	// Buffer request body for deterministic hash computation & replay
	var bodyBytes []byte
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			slog.Error("failed to read request body", slog.String("request_id", requestID), slog.Any("error", err))
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":      "ERR_INVALID_REQUEST_BODY",
					"message":   "Failed to read request body",
					"status":    http.StatusBadRequest,
					"requestId": requestID,
				},
			})
			recordReq("", "", "", "", "", http.StatusBadRequest, "ERR_INVALID_REQUEST_BODY", false)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	requestHash := idempotency.ComputeRequestHash(r.Method, r.URL.Path, r.URL.RawQuery, bodyBytes)
	idemKey := r.Header.Get(HeaderIdempotencyKey)

	var intentID string
	authHeader := r.Header.Get(HeaderPaymentSignature)
	bearerHeader := r.Header.Get("Authorization")
	apiKeyHeader := r.Header.Get("X-API-Key")
	agentKeyHeader := r.Header.Get("X-Agent-Key")

	// Determine client-provided credentials
	var apiKey string
	if strings.HasPrefix(strings.ToLower(bearerHeader), "bearer ") {
		apiKey = strings.TrimSpace(bearerHeader[7:])
	} else if apiKeyHeader != "" {
		apiKey = apiKeyHeader
	} else if agentKeyHeader != "" {
		apiKey = agentKeyHeader
	}

	// ------------------------------------------------------------------------
	// PATH 1: Virtual Wallet Payment (API Key Presented)
	// ------------------------------------------------------------------------
	if apiKey != "" {
		allowed := route.AllowedPaymentMethods
		if allowed == "onchain" {
			recordReq(route.TenantID, "prj_default", "", "", "", http.StatusMethodNotAllowed, ErrCodePaymentMethodNotAllowed, false)
			WriteProtocolError(w, ErrCodePaymentMethodNotAllowed, requestID, map[string]interface{}{
				"route_id":                route.RouteID,
				"allowed_payment_methods": allowed,
			})
			return
		}

		wallet, err := p.walletService.GetWalletByAPIKey(r.Context(), apiKey)
		if err != nil {
			recordReq(route.TenantID, "prj_default", "", "", "", http.StatusUnauthorized, ErrCodeAPIKeyInvalid, false)
			WriteProtocolError(w, ErrCodeAPIKeyInvalid, requestID, nil)
			return
		}

		if wallet.Status == "frozen_arrears" || wallet.Status == "frozen" {
			recordReq(wallet.TenantID, "prj_default", wallet.APIKeyID, "", "", http.StatusPaymentRequired, ErrCodeWalletFrozen, false)
			WriteProtocolError(w, ErrCodeWalletFrozen, requestID, map[string]interface{}{
				"wallet_id": wallet.WalletID,
				"status":    wallet.Status,
			})
			return
		}

		// API-key wallet requests first receive a price challenge. Only the SDK's
		// approved wallet authorization may claim it and proceed to a Hold.
		// SECURITY-REVIEW: challenge claim is durable and one-time before any money-moving operation.
		var walletChallenge *payment.WalletChallenge
		if authHeader == "" {
			if route.AllowedPaymentMethods == "onchain" {
				WriteProtocolError(w, ErrCodePaymentMethodNotAllowed, requestID, map[string]interface{}{"allowed_payment_methods": route.AllowedPaymentMethods})
				return
			}
			challengeID := generateChallengeID()
			intentID = "pi_" + challengeID
			now := time.Now()
			validBefore := now.Add(DefaultChallengeTTL)
			requestBinding := ComputeRequestBinding(r.Method, r.URL, bodyBytes)
			keyDigest := sha256.Sum256([]byte(apiKey))
			resourceURI := r.URL.EscapedPath()
			if resourceURI == "" {
				resourceURI = "/"
			}
			walletAsset := "USD"
			switch strings.ToUpper(strings.TrimSpace(route.Asset)) {
			case "CNY", "USD", "EUR":
				walletAsset = strings.ToUpper(strings.TrimSpace(route.Asset))
			}
			walletChallenge = &payment.WalletChallenge{
				ChallengeID: challengeID, TenantID: wallet.TenantID, WalletID: wallet.WalletID,
				APIKeyHash: hex.EncodeToString(keyDigest[:]), RouteID: route.RouteID, IntentID: intentID,
				IdempotencyKey: idemKey,
				RequestBinding: requestBinding, Amount: route.PriceBaseUnits, Asset: walletAsset, ResourceURI: resourceURI,
				ValidAfter: now, ValidBefore: validBefore, Status: "issued",
			}
			projectID := wallet.ProjectID
			if projectID == "" {
				projectID = "prj_default"
			}
			// The retry's Idempotency-Key is separately bound to the durable challenge; keeping
			// it off the provisional intent avoids colliding with an already completed request.
			initIntent(wallet.TenantID, projectID, wallet.AgentID, intentID, "", "wallet", payment.IntentStatusChallenged)
			if err := p.paymentService.CreateWalletChallenge(r.Context(), walletChallenge); err != nil {
				WriteProtocolError(w, ErrCodeUpstreamBadGateway, requestID, map[string]interface{}{"reason": "failed to persist wallet challenge"})
				return
			}
			ch := &PaymentChallenge{
				Version: DefaultVersion, ChallengeID: challengeID, Network: "agentpay:internal-ledger",
				Asset: walletAsset, AssetSymbol: walletAsset, Decimals: 6, Amount: fmt.Sprintf("%d", route.PriceBaseUnits),
				Payee: "agentpay:route:" + route.RouteID, Scheme: WalletScheme,
				ValidAfter: now.Unix(), ValidBefore: validBefore.Unix(), ResourceURI: resourceURI,
				Extensions: map[string]interface{}{"paymentMethod": "wallet", "routeId": route.RouteID, "requestBinding": requestBinding},
			}
			encoded, err := EncodeChallenge(ch, true)
			if err != nil {
				WriteProtocolError(w, ErrCodeUpstreamBadGateway, requestID, nil)
				return
			}
			w.Header().Set(HeaderPaymentRequired, encoded)
			WriteProtocolError(w, ErrCodePaymentRequired, requestID, map[string]interface{}{"challenge": ch})
			return
		}

		auth, err := DecodeWalletAuthorization(authHeader)
		if err != nil {
			WriteProtocolError(w, ErrCodeWalletAuthorizationInvalid, requestID, nil)
			return
		}
		keyDigest := sha256.Sum256([]byte(apiKey))
		requestBinding := ComputeRequestBinding(r.Method, r.URL, bodyBytes)
		if auth.Amount != fmt.Sprintf("%d", route.PriceBaseUnits) || auth.ResourceURI != r.URL.EscapedPath() || auth.RequestBinding != requestBinding {
			WriteProtocolError(w, ErrCodeWalletAuthorizationInvalid, requestID, nil)
			return
		}
		walletChallenge, err = p.paymentService.ConsumeWalletChallenge(r.Context(), auth.ChallengeID, wallet.TenantID, wallet.WalletID,
			hex.EncodeToString(keyDigest[:]), route.RouteID, requestBinding, auth.Nonce, idemKey, route.PriceBaseUnits, time.Now())
		if err != nil || walletChallenge == nil || auth.ValidAfter != walletChallenge.ValidAfter.Unix() || auth.ValidBefore != walletChallenge.ValidBefore.Unix() {
			if errors.Is(err, payment.ErrWalletChallengeReplay) {
				WriteProtocolError(w, ErrCodeNonceReplay, requestID, nil)
				return
			}
			WriteProtocolError(w, ErrCodeWalletAuthorizationInvalid, requestID, nil)
			return
		}
		intentID = walletChallenge.IntentID
		rejectChallenge := func() {
			if walletChallenge != nil {
				if err := p.paymentService.RejectWalletChallenge(r.Context(), walletChallenge.ChallengeID); err != nil {
					slog.Error("failed to mark wallet challenge rejected", slog.String("challenge_id", walletChallenge.ChallengeID), slog.Any("error", err))
				}
			}
		}

		// 1. Agent resolution and context sanitization (T2.9 & T1.8)
		declared := agent.ExtractDeclaredContext(r)
		agent.SanitizeAndStripSpoofedAgentHeaders(r.Header)
		userAgent := r.UserAgent()

		resolvedAgent, agentPolicy, _ := p.agentRegistry.ResolveAPIKey(apiKey)
		if resolvedAgent == nil {
			agentID := wallet.AgentID
			projectID := wallet.ProjectID
			if projectID == "" {
				projectID = "prj_default"
			}
			resolvedAgent = &agent.AgentIdentity{
				AgentID:   agentID,
				TenantID:  wallet.TenantID,
				ProjectID: projectID,
				Name:      "DefaultAgent",
				Status:    agent.AgentStatusActive,
			}
		}

		if walletChallenge == nil {
			intentID = fmt.Sprintf("pi_%s", requestID)
		}
		correlation := agent.CorrelationContext{
			RequestID:      requestID,
			IdempotencyKey: idemKey,
			IntentID:       intentID,
		}

		// 2. Synchronous In-Path Agent Policy Enforcement (T2.9)
		decision := p.agentEnforcer.Evaluate(r.Context(), resolvedAgent, agentPolicy, route.RouteID, r.URL.Path, route.PriceBaseUnits)
		if p.auditEmitter != nil && resolvedAgent.AgentID != "" {
			p.auditEmitter.Emit(agent.BuildPolicyDecisionEvent(
				resolvedAgent,
				decision,
				route.RouteID,
				correlation,
				clientIP,
				userAgent,
				&declared,
			))
		}
		if !decision.Allowed {
			rejectChallenge()
			p.metrics.IncPaymentsFailed()
			initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusRejected)
			recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, wallet.APIKeyID, resolvedAgent.AgentID, intentID, http.StatusForbidden, decision.ReasonCode, false)
			WriteProtocolError(w, decision.ReasonCode, requestID, map[string]interface{}{
				"agent_id":    resolvedAgent.AgentID,
				"reason":      decision.Reason,
				"reason_code": decision.ReasonCode,
				"rule_id":     decision.RuleID,
			})
			return
		}

		// Helper closure to emit agent.request audit event upon completion
		emitRequestAudit := func(statusCode int, isReplay bool, errCode string) {
			durationMs := time.Since(startTime).Milliseconds()
			if p.auditEmitter != nil && resolvedAgent.AgentID != "" {
				p.auditEmitter.Emit(agent.BuildRequestEvent(
					resolvedAgent,
					correlation,
					clientIP,
					userAgent,
					route.RouteID,
					statusCode,
					durationMs,
					requestHash,
					&declared,
				))
			}
			recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, wallet.APIKeyID, resolvedAgent.AgentID, intentID, statusCode, errCode, isReplay)
			if !isReplay && correlation.HoldID != "" {
				settlementID := fmt.Sprintf("stl_%s", correlation.HoldID)
				fulfillmentID := fmt.Sprintf("ful_%s", requestID)
				updateCorrelationOutcome(intentID, settlementID, fulfillmentID, statusCode)
			}
		}

		// Helper closure to emit agent.spend audit event
		emitSpendAudit := func(holdID string, action string) {
			correlation.HoldID = holdID
			if p.auditEmitter != nil && resolvedAgent.AgentID != "" {
				corr := correlation
				corr.HoldID = holdID
				p.auditEmitter.Emit(agent.BuildSpendEvent(
					resolvedAgent,
					corr,
					clientIP,
					userAgent,
					route.PriceBaseUnits,
					route.Asset,
					"wallet",
					action,
					&declared,
				))
			}
			settlementID := fmt.Sprintf("stl_%s", holdID)
			if action == "hold" {
				initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusAuthorized)
				initSettlement(settlementID, intentID, "wallet", payment.SettlementStatusHeld, wallet.WalletID, route.PayeeAddress, holdID, holdID)
				fulfillmentID := fmt.Sprintf("ful_%s", requestID)
				initFulfillment(fulfillmentID, intentID)
				if p.paymentService != nil {
					_ = p.paymentService.LinkWalletAssociation(r.Context(), intentID, &payment.WalletHoldAssociation{
						HoldID:    holdID,
						WalletID:  wallet.WalletID,
						Amount:    route.PriceBaseUnits,
						Status:    "held",
						ExpiresAt: time.Now().Add(120 * time.Second),
					}, nil)
				}
			} else if action == "capture" {
				recordUsage(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, "wallet", true)
				if p.paymentService != nil {
					_ = p.paymentService.LinkWalletAssociation(r.Context(), intentID, &payment.WalletHoldAssociation{
						HoldID:    holdID,
						WalletID:  wallet.WalletID,
						Amount:    route.PriceBaseUnits,
						Status:    "captured",
						ExpiresAt: time.Now().Add(120 * time.Second),
					}, &payment.LedgerAssociation{
						EntryID:   "led_" + holdID,
						WalletID:  wallet.WalletID,
						Type:      "capture",
						Amount:    -route.PriceBaseUnits,
						CreatedAt: time.Now(),
					})
					_ = p.paymentService.UpdateSettlementStatus(r.Context(), settlementID, payment.SettlementStatusConfirmed, "", nil, "")
				}
			} else if action == "release" {
				recordUsage(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, "wallet", false)
				if p.paymentService != nil {
					_ = p.paymentService.LinkWalletAssociation(r.Context(), intentID, &payment.WalletHoldAssociation{
						HoldID:    holdID,
						WalletID:  wallet.WalletID,
						Amount:    route.PriceBaseUnits,
						Status:    "released",
						ExpiresAt: time.Now().Add(120 * time.Second),
					}, &payment.LedgerAssociation{
						EntryID:   "led_" + holdID,
						WalletID:  wallet.WalletID,
						Type:      "hold_release",
						Amount:    route.PriceBaseUnits,
						CreatedAt: time.Now(),
					})
					_ = p.paymentService.UpdateSettlementStatus(r.Context(), settlementID, payment.SettlementStatusReleased, "", nil, "")
				}
			}
		}

		// 1.1 Idempotent Request Path
		if idemKey != "" {
			tenantID := wallet.TenantID
			if tenantID == "" {
				tenantID = wallet.WalletID
			}

			rec, checkRes, err := p.idempotencyStore.CheckOrLock(r.Context(), tenantID, idemKey, requestHash, intentID, "wallet")
			if err != nil {
				recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, wallet.APIKeyID, resolvedAgent.AgentID, intentID, http.StatusBadGateway, ErrCodeUpstreamBadGateway, false)
				WriteProtocolError(w, ErrCodeUpstreamBadGateway, requestID, map[string]interface{}{"error": err.Error()})
				return
			}

			// Case A: Conflict (in-flight or payload mismatch)
			if checkRes == idempotency.ResultConflict {
				rejectChallenge()
				recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, wallet.APIKeyID, resolvedAgent.AgentID, intentID, http.StatusConflict, ErrCodeIdempotencyConflict, false)
				WriteProtocolError(w, ErrCodeIdempotencyConflict, requestID, map[string]interface{}{
					"idempotency_key": idemKey,
					"reason":          "Idempotency conflict: request in-progress or payload altered",
				})
				return
			}

			// Case B: Cache Hit (already succeeded previously)
			if checkRes == idempotency.ResultCacheHit {
				rejectChallenge()
				WriteCachedResponse(w, rec)
				emitRequestAudit(rec.StatusCode, true, "")
				return
			}

			// Case C: Free Replay (previous attempt failed upstream with 5xx or timeout)
			if checkRes == idempotency.ResultFreeReplay {
				rejectChallenge()
				// Anti-Double-Charge Invariant: Reuse existing HoldID without creating a second hold
				pCtx := &proxy.PaymentContext{
					PayerAddress:        wallet.WalletID,
					AmountBaseUnits:     route.PriceBaseUnits,
					Asset:               route.Asset,
					Network:             "platform:internal_ledger",
					TxHash:              rec.HoldID,
					IdempotencyKey:      idemKey,
					RequestID:           requestID,
					ConfigVersion:       configVersion,
					ConfigHash:          configHash,
					AgentID:             resolvedAgent.AgentID,
					TenantID:            resolvedAgent.TenantID,
					ProjectID:           resolvedAgent.ProjectID,
					DeclaredSessionID:   declared.SessionID,
					DeclaredTaskID:      declared.TaskID,
					DeclaredToolName:    declared.ToolName,
					DeclaredParentReqID: declared.ParentRequestID,
				}

				resp, err := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
				if err != nil {
					slog.Error("failed to forward request with capture (wallet idempotent free replay)",
						slog.String("request_id", requestID),
						slog.String("route_id", route.RouteID),
						slog.Any("error", err),
					)
				}
				if resp == nil {
					resp = &proxy.ProxyResponse{
						StatusCode: http.StatusBadGateway,
						Headers:    make(map[string]string),
					}
				}
				emitRequestAudit(resp.StatusCode, true, "")

				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					if err := p.walletService.CaptureFunds(r.Context(), rec.HoldID); err != nil {
						slog.Error("failed to capture wallet funds", slog.String("hold_id", rec.HoldID), slog.Any("error", err))
						// Preserve the existing hold and leave the idempotency record
						// recoverable; do not emit a capture event or success metric.
						p.markWalletFundsRecovery(r.Context(), tenantID, idemKey, resp.StatusCode, "wallet capture failed")
						return
					}
					p.agentEnforcer.RecordSpend(resolvedAgent.AgentID, route.PriceBaseUnits)
					emitSpendAudit(rec.HoldID, "capture")
					p.metrics.IncHoldsCaptured()
					p.metrics.IncPaymentsVerified()
					if err := p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body); err != nil {
						slog.Error("failed to mark idempotency completed", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
					}
				} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
					if err := p.walletService.ReleaseFunds(r.Context(), rec.HoldID); err != nil {
						slog.Error("failed to release wallet funds", slog.String("hold_id", rec.HoldID), slog.Any("error", err))
						p.markWalletFundsRecovery(r.Context(), tenantID, idemKey, resp.StatusCode, "wallet release failed")
						return
					}
					emitSpendAudit(rec.HoldID, "release")
					p.metrics.IncHoldsReleased()
					if err := p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body); err != nil {
						slog.Error("failed to mark idempotency completed", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
					}
				} else {
					// 5xx or Timeout: KEEP HELD, Mark Recovery Required (strictly no auto-release or double capture)
					if err := p.idempotencyStore.MarkRecoveryRequired(r.Context(), tenantID, idemKey, resp.StatusCode, "upstream failure"); err != nil {
						slog.Error("failed to mark recovery required", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
					}
				}
				return
			}

			// Case D: ResultNew (First time receiving this idempotent request)
			if wallet.AvailableBalance < route.PriceBaseUnits {
				rejectChallenge()
				_ = p.idempotencyStore.ReleaseLock(r.Context(), tenantID, idemKey)
				p.metrics.IncPaymentsFailed()
				initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusFailed)
				initSettlement(fmt.Sprintf("stl_%s", requestID), intentID, "wallet", payment.SettlementStatusFailed, wallet.WalletID, route.PayeeAddress, "", "")
				recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, wallet.APIKeyID, resolvedAgent.AgentID, intentID, http.StatusPaymentRequired, ErrCodeWalletInsufficientBalance, false)
				WriteProtocolError(w, ErrCodeWalletInsufficientBalance, requestID, map[string]interface{}{
					"wallet_id":         wallet.WalletID,
					"required_amount":   route.PriceBaseUnits,
					"available_balance": wallet.AvailableBalance,
					"auto_downgrade":    false,
				})
				return
			}

			holdID, err := p.walletService.HoldFunds(r.Context(), wallet.WalletID, route.PriceBaseUnits, intentID)
			if err != nil {
				rejectChallenge()
				_ = p.idempotencyStore.ReleaseLock(r.Context(), tenantID, idemKey)
				p.metrics.IncPaymentsFailed()
				initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusFailed)
				initSettlement(fmt.Sprintf("stl_%s", requestID), intentID, "wallet", payment.SettlementStatusFailed, wallet.WalletID, route.PayeeAddress, "", "")
				recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, wallet.APIKeyID, resolvedAgent.AgentID, intentID, http.StatusPaymentRequired, ErrCodeWalletInsufficientBalance, false)
				WriteProtocolError(w, ErrCodeWalletInsufficientBalance, requestID, nil)
				return
			}
			// SECURITY-REVIEW: tie the durable one-time authorization to the created hold before upstream dispatch.
			if walletChallenge != nil {
				if err := p.paymentService.SetWalletChallengeHold(r.Context(), walletChallenge.ChallengeID, holdID); err != nil {
					_ = p.walletService.ReleaseFunds(r.Context(), holdID)
					rejectChallenge()
					WriteProtocolError(w, ErrCodeUpstreamBadGateway, requestID, map[string]interface{}{"reason": "failed to link wallet challenge to hold"})
					return
				}
			}
			p.metrics.IncHoldsCreated()
			emitSpendAudit(holdID, "hold")
			_ = p.idempotencyStore.AttachPaymentDetails(r.Context(), tenantID, idemKey, holdID, "")

			pCtx := &proxy.PaymentContext{
				PayerAddress:        wallet.WalletID,
				AmountBaseUnits:     route.PriceBaseUnits,
				Asset:               route.Asset,
				Network:             "platform:internal_ledger",
				TxHash:              holdID,
				IdempotencyKey:      idemKey,
				RequestID:           requestID,
				ConfigVersion:       configVersion,
				ConfigHash:          configHash,
				AgentID:             resolvedAgent.AgentID,
				TenantID:            resolvedAgent.TenantID,
				ProjectID:           resolvedAgent.ProjectID,
				DeclaredSessionID:   declared.SessionID,
				DeclaredTaskID:      declared.TaskID,
				DeclaredToolName:    declared.ToolName,
				DeclaredParentReqID: declared.ParentRequestID,
			}

			resp, err := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
			if err != nil {
				slog.Error("failed to forward request with capture (wallet idempotent fresh)",
					slog.String("request_id", requestID),
					slog.String("route_id", route.RouteID),
					slog.Any("error", err),
				)
			}
			if resp == nil {
				resp = &proxy.ProxyResponse{
					StatusCode: http.StatusBadGateway,
					Headers:    make(map[string]string),
				}
			}
			emitRequestAudit(resp.StatusCode, false, "")

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				// Capture only after a successful upstream response. A capture failure must
				// remain visible to recovery; logging alone does not reconcile this hold.
				if err := p.walletService.CaptureFunds(r.Context(), holdID); err != nil {
					slog.Error("failed to capture wallet funds", slog.String("hold_id", holdID), slog.Any("error", err))
					// The service response is already sent, but payment completion is
					// uncertain. Keep the hold and make the idempotency record recoverable.
					p.markWalletFundsRecovery(r.Context(), tenantID, idemKey, resp.StatusCode, "wallet capture failed")
					return
				}
				p.agentEnforcer.RecordSpend(resolvedAgent.AgentID, route.PriceBaseUnits)
				emitSpendAudit(holdID, "capture")
				p.metrics.IncHoldsCaptured()
				p.metrics.IncPaymentsVerified()
				if err := p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body); err != nil {
					slog.Error("failed to mark idempotency completed", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
				}
			} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				if err := p.walletService.ReleaseFunds(r.Context(), holdID); err != nil {
					slog.Error("failed to release wallet funds", slog.String("hold_id", holdID), slog.Any("error", err))
					p.markWalletFundsRecovery(r.Context(), tenantID, idemKey, resp.StatusCode, "wallet release failed")
					return
				}
				emitSpendAudit(holdID, "release")
				p.metrics.IncHoldsReleased()
				if err := p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body); err != nil {
					slog.Error("failed to mark idempotency completed", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
				}
			} else {
				// A 5xx or transport failure leaves the upstream outcome ambiguous. Keep
				// the hold and idempotency record recoverable; releasing here could grant
				// the service while also refunding the buyer.
				if err := p.idempotencyStore.MarkRecoveryRequired(r.Context(), tenantID, idemKey, resp.StatusCode, "upstream failure"); err != nil {
					slog.Error("failed to mark recovery required", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
				}
			}
			return
		}

		// 1.2 Non-Idempotent Request Path
		if wallet.AvailableBalance < route.PriceBaseUnits {
			rejectChallenge()
			p.metrics.IncPaymentsFailed()
			initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusFailed)
			initSettlement(fmt.Sprintf("stl_%s", requestID), intentID, "wallet", payment.SettlementStatusFailed, wallet.WalletID, route.PayeeAddress, "", "")
			recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, wallet.APIKeyID, resolvedAgent.AgentID, intentID, http.StatusPaymentRequired, ErrCodeWalletInsufficientBalance, false)
			WriteProtocolError(w, ErrCodeWalletInsufficientBalance, requestID, map[string]interface{}{
				"wallet_id":         wallet.WalletID,
				"required_amount":   route.PriceBaseUnits,
				"available_balance": wallet.AvailableBalance,
				"auto_downgrade":    false,
			})
			return
		}

		holdID, err := p.walletService.HoldFunds(r.Context(), wallet.WalletID, route.PriceBaseUnits, intentID)
		if err != nil {
			rejectChallenge()
			p.metrics.IncPaymentsFailed()
			initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusFailed)
			initSettlement(fmt.Sprintf("stl_%s", requestID), intentID, "wallet", payment.SettlementStatusFailed, wallet.WalletID, route.PayeeAddress, "", "")
			recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, wallet.APIKeyID, resolvedAgent.AgentID, intentID, http.StatusPaymentRequired, ErrCodeWalletInsufficientBalance, false)
			WriteProtocolError(w, ErrCodeWalletInsufficientBalance, requestID, nil)
			return
		}
		// SECURITY-REVIEW: tie the durable one-time authorization to the created hold before upstream dispatch.
		if walletChallenge != nil {
			if err := p.paymentService.SetWalletChallengeHold(r.Context(), walletChallenge.ChallengeID, holdID); err != nil {
				_ = p.walletService.ReleaseFunds(r.Context(), holdID)
				rejectChallenge()
				WriteProtocolError(w, ErrCodeUpstreamBadGateway, requestID, map[string]interface{}{"reason": "failed to link wallet challenge to hold"})
				return
			}
		}
		p.metrics.IncHoldsCreated()
		emitSpendAudit(holdID, "hold")
		settlementID := fmt.Sprintf("stl_%s", requestID)
		initSettlement(settlementID, intentID, "wallet", payment.SettlementStatusHeld, wallet.WalletID, route.PayeeAddress, holdID, holdID)

		pCtx := &proxy.PaymentContext{
			PayerAddress:        wallet.WalletID,
			AmountBaseUnits:     route.PriceBaseUnits,
			Asset:               route.Asset,
			Network:             "platform:internal_ledger",
			TxHash:              holdID,
			RequestID:           requestID,
			ConfigVersion:       configVersion,
			ConfigHash:          configHash,
			AgentID:             resolvedAgent.AgentID,
			TenantID:            resolvedAgent.TenantID,
			ProjectID:           resolvedAgent.ProjectID,
			DeclaredSessionID:   declared.SessionID,
			DeclaredTaskID:      declared.TaskID,
			DeclaredToolName:    declared.ToolName,
			DeclaredParentReqID: declared.ParentRequestID,
		}

		resp, err := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
		if err != nil {
			slog.Error("failed to forward request with capture (wallet non-idempotent)",
				slog.String("request_id", requestID),
				slog.String("route_id", route.RouteID),
				slog.Any("error", err),
			)
		}
		if resp == nil {
			resp = &proxy.ProxyResponse{
				StatusCode: http.StatusBadGateway,
				Headers:    make(map[string]string),
			}
		}
		emitRequestAudit(resp.StatusCode, false, "")

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// This path has no idempotency record to drive automated recovery, so a
			// capture error needs a durable settlement/reconciliation signal.
			if err := p.walletService.CaptureFunds(r.Context(), holdID); err != nil {
				slog.Error("failed to capture wallet funds", slog.String("hold_id", holdID), slog.Any("error", err))
				// There is no idempotency record on this path, so persist the
				// settlement as unknown for the existing payment reconciliation view.
				if p.paymentService != nil {
					if statusErr := p.paymentService.UpdateSettlementStatus(r.Context(), settlementID, payment.SettlementStatusUnknown, "", nil, "wallet capture failed"); statusErr != nil {
						slog.Error("failed to mark wallet settlement unknown", slog.String("settlement_id", settlementID), slog.Any("error", statusErr))
					}
				}
				return
			}
			p.agentEnforcer.RecordSpend(resolvedAgent.AgentID, route.PriceBaseUnits)
			emitSpendAudit(holdID, "capture")
			p.metrics.IncHoldsCaptured()
			p.metrics.IncPaymentsVerified()
			if p.paymentService != nil {
				if statusErr := p.paymentService.UpdateSettlementStatus(r.Context(), settlementID, payment.SettlementStatusConfirmed, "", nil, ""); statusErr != nil {
					slog.Error("failed to mark wallet settlement confirmed", slog.String("settlement_id", settlementID), slog.Any("error", statusErr))
				}
			}
		} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			if err := p.walletService.ReleaseFunds(r.Context(), holdID); err != nil {
				slog.Error("failed to release wallet funds", slog.String("hold_id", holdID), slog.Any("error", err))
				if p.paymentService != nil {
					if statusErr := p.paymentService.UpdateSettlementStatus(r.Context(), settlementID, payment.SettlementStatusUnknown, "", nil, "wallet release failed"); statusErr != nil {
						slog.Error("failed to mark wallet settlement unknown", slog.String("settlement_id", settlementID), slog.Any("error", statusErr))
					}
				}
				return
			}
			emitSpendAudit(holdID, "release")
			p.metrics.IncHoldsReleased()
			if p.paymentService != nil {
				if statusErr := p.paymentService.UpdateSettlementStatus(r.Context(), settlementID, payment.SettlementStatusReleased, "", nil, ""); statusErr != nil {
					slog.Error("failed to mark wallet settlement released", slog.String("settlement_id", settlementID), slog.Any("error", statusErr))
				}
			}
		}
		// In non-idempotent 5xx, hold remains preserved for manual reconciliation/review.
		// Keep this conservative rule aligned with the idempotent recovery path above.
		return
	}

	// ------------------------------------------------------------------------
	// PATH 2: On-Chain x402 Payment (PAYMENT-SIGNATURE Presented)
	// ------------------------------------------------------------------------
	if authHeader != "" {
		allowed := route.AllowedPaymentMethods
		if allowed == "wallet" {
			recordReq(route.TenantID, "prj_default", "", "", "", http.StatusMethodNotAllowed, ErrCodePaymentMethodNotAllowed, false)
			WriteProtocolError(w, ErrCodePaymentMethodNotAllowed, requestID, map[string]interface{}{
				"route_id":                route.RouteID,
				"allowed_payment_methods": allowed,
			})
			return
		}
		slog.Info("x402 authorization received", slog.String("request_id", requestID))
		auth, err := DecodeAuthorization(authHeader)
		if err != nil {
			p.metrics.IncPaymentsFailed()
			intentID := fmt.Sprintf("pi_%s", requestID)
			initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusFailed)
			recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusUnauthorized, ErrCodeSignatureInvalid, false)
			WriteProtocolError(w, ErrCodeSignatureInvalid, requestID, map[string]interface{}{
				"reason": err.Error(),
			})
			return
		}

		oc := p.resolveOnchain(route)
		network, onchainPrice, onchainAsset := oc.Network, oc.Price, oc.Asset
		contractAddress := oc.Contract
		intentID := fmt.Sprintf("pi_%s", requestID)
		if auth.ChallengeID != "" {
			intentID = "pi_" + auth.ChallengeID
		}
		settlementID := fmt.Sprintf("stl_%s", requestID)

		// 2.1 Idempotent On-Chain Request
		if idemKey != "" {
			tenantID := auth.Payer
			rec, checkRes, err := p.idempotencyStore.CheckOrLock(r.Context(), tenantID, idemKey, requestHash, intentID, "onchain")
			if err != nil {
				recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusBadGateway, ErrCodeUpstreamBadGateway, false)
				WriteProtocolError(w, ErrCodeUpstreamBadGateway, requestID, map[string]interface{}{"error": err.Error()})
				return
			}

			if checkRes == idempotency.ResultConflict {
				recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusConflict, ErrCodeIdempotencyConflict, false)
				WriteProtocolError(w, ErrCodeIdempotencyConflict, requestID, map[string]interface{}{
					"idempotency_key": idemKey,
					"reason":          "Idempotency conflict: request in-progress or payload altered",
				})
				return
			}

			if checkRes == idempotency.ResultCacheHit {
				recordReq(route.TenantID, "prj_default", "", "", intentID, rec.StatusCode, "", true)
				WriteCachedResponse(w, rec)
				return
			}

			if checkRes == idempotency.ResultFreeReplay {
				// Free Replay: Do not re-verify signature or double-charge
				pCtx := &proxy.PaymentContext{
					PayerAddress:    auth.Payer,
					AmountBaseUnits: route.PriceBaseUnits,
					Asset:           contractAddress,
					Network:         network,
					TxHash:          auth.Nonce,
					IdempotencyKey:  idemKey,
					RequestID:       requestID,
					ConfigVersion:   configVersion,
					ConfigHash:      configHash,
				}
				resp, err := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
				if err != nil {
					slog.Error("failed to forward request with capture (on-chain idempotent retry)",
						slog.String("request_id", requestID),
						slog.String("route_id", route.RouteID),
						slog.Any("error", err),
					)
				}
				if resp == nil {
					resp = &proxy.ProxyResponse{
						StatusCode: http.StatusBadGateway,
						Headers:    make(map[string]string),
					}
				}
				recordReq(route.TenantID, "prj_default", "", "", intentID, resp.StatusCode, "", true)
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					if err := p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body); err != nil {
						slog.Error("failed to mark idempotency completed", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
					}
				} else {
					if err := p.idempotencyStore.MarkRecoveryRequired(r.Context(), tenantID, idemKey, resp.StatusCode, "upstream failure"); err != nil {
						slog.Error("failed to mark recovery required", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
					}
				}
				return
			}

			// ResultNew: Validate Nonce & Signature
			validNonce, err := p.nonceStore.CheckAndRecordNonce(network, contractAddress, auth.Payer, auth.Nonce)
			if err != nil || !validNonce {
				_ = p.idempotencyStore.ReleaseLock(r.Context(), tenantID, idemKey)
				p.metrics.IncPaymentsFailed()
				initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusFailed)
				initSettlement(settlementID, intentID, "onchain", payment.SettlementStatusFailed, auth.Payer, route.PayeeAddress, auth.Nonce, "")
				recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusPaymentRequired, ErrCodeNonceAlreadyUsed, false)
				WriteProtocolError(w, ErrCodeNonceAlreadyUsed, requestID, map[string]interface{}{
					"nonce": auth.Nonce,
					"payer": auth.Payer,
				})
				return
			}

			ch := oc.verifyChallenge(auth, p.facilitatorURL)

			result, err := p.verifier.VerifyPayment(r.Context(), ch, auth)
			if err != nil || result == nil || !result.Valid {
				_ = p.idempotencyStore.ReleaseLock(r.Context(), tenantID, idemKey)
				p.metrics.IncPaymentsFailed()
				errCode := ErrCodeSignatureInvalid
				reason := "signature verification failed"
				if err != nil {
					reason = err.Error()
				} else if result != nil {
					if result.ErrorCode != "" {
						errCode = result.ErrorCode
					}
					if result.Message != "" {
						reason = result.Message
					}
				}
				initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusFailed)
				initSettlement(settlementID, intentID, "onchain", payment.SettlementStatusFailed, auth.Payer, route.PayeeAddress, auth.Nonce, "")
				recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusPaymentRequired, errCode, false)
				WriteProtocolError(w, errCode, requestID, map[string]interface{}{
					"reason": reason,
				})
				return
			}

			initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusAuthorized)
			initSettlement(settlementID, intentID, "onchain", payment.SettlementStatusSubmitted, auth.Payer, route.PayeeAddress, auth.Nonce, "")
			fulfillmentID := fmt.Sprintf("ful_%s", requestID)
			initFulfillment(fulfillmentID, intentID)

			// Register settlement in recovery coordinator
			_ = p.recoveryCoord.RegisterSettlement(&recovery.SettlementRecord{
				SettlementID:    settlementID,
				PaymentIntentID: intentID,
				PaymentMethod:   "onchain",
				Payer:           auth.Payer,
				Amount:          onchainPrice,
				Asset:           onchainAsset,
				Network:         network,
				TxHashOrNonce:   auth.Nonce,
				Status:          recovery.SettlementSubmitted,
			})
			_ = p.idempotencyStore.AttachPaymentDetails(r.Context(), tenantID, idemKey, "", settlementID)
			p.metrics.IncPaymentsVerified()

			pCtx := &proxy.PaymentContext{
				PayerAddress:    result.Payer,
				AmountBaseUnits: onchainPrice,
				Asset:           onchainAsset,
				Network:         network,
				TxHash:          auth.Nonce,
				IdempotencyKey:  idemKey,
				RequestID:       requestID,
				ConfigVersion:   configVersion,
				ConfigHash:      configHash,
			}

			resp, err := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
			if err != nil {
				slog.Error("failed to forward request with capture (on-chain idempotent)",
					slog.String("request_id", requestID),
					slog.String("route_id", route.RouteID),
					slog.Any("error", err),
				)
			}
			if resp == nil {
				resp = &proxy.ProxyResponse{
					StatusCode: http.StatusBadGateway,
					Headers:    make(map[string]string),
				}
			}
			updateCorrelationOutcome(intentID, settlementID, fulfillmentID, resp.StatusCode)
			recordReq(route.TenantID, "prj_default", "", "", intentID, resp.StatusCode, "", false)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if err := p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body); err != nil {
					slog.Error("failed to mark idempotency completed", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
				}
				recordUsage(route.TenantID, "prj_default", "", intentID, "onchain", true)
				if result != nil && result.VerificationToken != "" {
					p.dispatchAsyncSettlement(settlementID, result.VerificationToken, auth, idemKey)
				}
			} else {
				// Anti-Rollback: Record remains SUBMITTED, Mark recovery required
				_ = p.recoveryCoord.MarkUnknown(settlementID, "upstream error")
				if err := p.idempotencyStore.MarkRecoveryRequired(r.Context(), tenantID, idemKey, resp.StatusCode, "upstream failure"); err != nil {
					slog.Error("failed to mark recovery required", slog.String("tenant_id", tenantID), slog.String("idempotency_key", idemKey), slog.Any("error", err))
				}
				recordUsage(route.TenantID, "prj_default", "", intentID, "onchain", false)
			}
			return
		}

		// 2.2 Non-Idempotent On-Chain Request
		validNonce, err := p.nonceStore.CheckAndRecordNonce(network, contractAddress, auth.Payer, auth.Nonce)
		if err != nil || !validNonce {
			p.metrics.IncPaymentsFailed()
			initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusFailed)
			initSettlement(settlementID, intentID, "onchain", payment.SettlementStatusFailed, auth.Payer, route.PayeeAddress, auth.Nonce, "")
			recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusPaymentRequired, ErrCodeNonceAlreadyUsed, false)
			WriteProtocolError(w, ErrCodeNonceAlreadyUsed, requestID, map[string]interface{}{
				"nonce": auth.Nonce,
				"payer": auth.Payer,
			})
			return
		}

		ch := oc.verifyChallenge(auth, p.facilitatorURL)

		result, err := p.verifier.VerifyPayment(r.Context(), ch, auth)
		if err != nil || result == nil || !result.Valid {
			p.metrics.IncPaymentsFailed()
			errCode := ErrCodeSignatureInvalid
			reason := "signature verification failed"
			if err != nil {
				reason = err.Error()
			} else if result != nil {
				if result.ErrorCode != "" {
					errCode = result.ErrorCode
				}
				if result.Message != "" {
					reason = result.Message
				}
			}
			initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusFailed)
			initSettlement(settlementID, intentID, "onchain", payment.SettlementStatusFailed, auth.Payer, route.PayeeAddress, auth.Nonce, "")
			recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusPaymentRequired, errCode, false)
			WriteProtocolError(w, errCode, requestID, map[string]interface{}{
				"reason": reason,
			})
			return
		}

		initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusAuthorized)
		initSettlement(settlementID, intentID, "onchain", payment.SettlementStatusSubmitted, auth.Payer, route.PayeeAddress, auth.Nonce, "")
		fulfillmentID := fmt.Sprintf("ful_%s", requestID)
		initFulfillment(fulfillmentID, intentID)

		p.metrics.IncPaymentsVerified()
		pCtx := &proxy.PaymentContext{
			PayerAddress:    result.Payer,
			AmountBaseUnits: onchainPrice,
			Asset:           onchainAsset,
			Network:         network,
			TxHash:          auth.Nonce,
			RequestID:       requestID,
			ConfigVersion:   configVersion,
			ConfigHash:      configHash,
		}

		resp, err := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
		if err != nil {
			slog.Error("failed to forward request with capture (on-chain non-idempotent)",
				slog.String("request_id", requestID),
				slog.String("route_id", route.RouteID),
				slog.Any("error", err),
			)
		}
		if resp == nil {
			resp = &proxy.ProxyResponse{
				StatusCode: http.StatusBadGateway,
				Headers:    make(map[string]string),
			}
		}
		updateCorrelationOutcome(intentID, settlementID, fulfillmentID, resp.StatusCode)
		recordReq(route.TenantID, "prj_default", "", "", intentID, resp.StatusCode, "", false)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			recordUsage(route.TenantID, "prj_default", "", intentID, "onchain", true)
			if result != nil && result.VerificationToken != "" {
				p.dispatchAsyncSettlement(settlementID, result.VerificationToken, auth, idemKey)
			}
		} else {
			recordUsage(route.TenantID, "prj_default", "", intentID, "onchain", false)
		}
		return
	}

	// ------------------------------------------------------------------------
	// PATH 3: Unauthenticated -> Issue HTTP 402 Payment Required Challenge
	// ------------------------------------------------------------------------
	if route.AllowedPaymentMethods == "wallet" {
		WriteProtocolError(w, ErrCodeWalletAuthRequired, requestID, nil)
		return
	}
	oc := p.resolveOnchain(route)
	facURL := p.facilitatorURL
	if facURL == "" {
		facURL = "http://127.0.0.1:8001"
	}
	ch := oc.newChallenge(facURL, r.URL.Path, 300*time.Second)
	intentID = "pi_" + ch.ChallengeID
	// Keep Idempotency-Key off the provisional challenged intent so the retry can bind it cleanly.
	initIntent(route.TenantID, "prj_default", "", intentID, "", "onchain", payment.IntentStatusChallenged)
	recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusPaymentRequired, ErrCodePaymentRequired, false)
	p.metrics.IncChallenges()
	encodedChallenge, err := EncodeChallenge(ch, true)
	if err != nil {
		WriteProtocolError(w, ErrCodeUpstreamBadGateway, requestID, nil)
		return
	}

	w.Header().Set(HeaderPaymentRequired, encodedChallenge)
	WriteProtocolError(w, ErrCodePaymentRequired, requestID, map[string]interface{}{
		"challenge": ch,
	})
}
