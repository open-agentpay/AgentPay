package x402

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	verifier         PaymentVerifier
	nonceStore       NonceStore
	walletService    WalletService
	idempotencyStore idempotency.Store
	recoveryCoord    *recovery.RecoveryCoordinator
	proxyHandler     *proxy.Handler
	metrics          *metrics.Metrics
	agentRegistry    *agent.Registry
	agentEnforcer    *agent.Enforcer
	auditEmitter     agent.AuditEmitter
	usageService     usage.Service
	paymentService   payment.Service
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
	var configVersion, configHash string
	if snap != nil {
		configVersion = snap.VersionID
		configHash = snap.SnapshotHash
		w.Header().Set("X-Config-Version", configVersion)
	}

	startTime := time.Now()
	clientIP := agent.ExtractClientIP(r)

	recordReq := func(tenantID, projectID, apiKeyID, agentID, intentID string, statusCode int, errorCode string, isReplay bool) {
		if p.usageService != nil {
			tID := tenantID
			if tID == "" && route != nil {
				tID = route.TenantID
			}
			pID := projectID
			if pID == "" {
				pID = "prj_default"
			}
			_ = p.usageService.RecordRequest(r.Context(), &usage.RequestRecord{
				RequestID:     requestID,
				TenantID:      tID,
				ProjectID:     pID,
				APIKeyID:      apiKeyID,
				AgentID:       agentID,
				RouteID:       route.RouteID,
				ConfigVersion: configVersion,
				ClientIP:      clientIP,
				HTTPMethod:    r.Method,
				RequestPath:   r.URL.Path,
				StatusCode:    statusCode,
				DurationMs:    time.Since(startTime).Milliseconds(),
				ErrorCode:     errorCode,
				IsReplay:      isReplay,
				IntentID:      intentID,
				CreatedAt:     time.Now(),
			})
		}
	}

	recordUsage := func(tenantID, projectID, agentID, intentID, paymentMethod string, isSuccess bool) {
		if p.usageService != nil {
			tID := tenantID
			if tID == "" && route != nil {
				tID = route.TenantID
			}
			pID := projectID
			if pID == "" {
				pID = "prj_default"
			}
			_ = p.usageService.RecordUsage(r.Context(), &usage.UsageEvent{
				EventID:         usage.GenerateUsageEventID(),
				TenantID:        tID,
				ProjectID:       pID,
				AgentID:         agentID,
				RouteID:         route.RouteID,
				IntentID:        intentID,
				AmountBaseUnits: route.PriceBaseUnits,
				Asset:           route.Asset,
				PaymentMethod:   paymentMethod,
				IsSuccess:       isSuccess,
				CreatedAt:       time.Now(),
			})
		}
	}

	initIntent := func(tenantID, projectID, agentID, intentID, idemKey, paymentMethod, status string) {
		if p.paymentService != nil {
			tID := tenantID
			if tID == "" && route != nil {
				tID = route.TenantID
			}
			pID := projectID
			if pID == "" {
				pID = "prj_default"
			}
			_ = p.paymentService.CreateIntent(r.Context(), &payment.PaymentIntent{
				IntentID:        intentID,
				TenantID:        tID,
				ProjectID:       pID,
				AgentID:         agentID,
				RouteID:         route.RouteID,
				IdempotencyKey:  idemKey,
				RequestID:       requestID,
				PaymentMethod:   paymentMethod,
				AmountBaseUnits: route.PriceBaseUnits,
				Asset:           route.Asset,
				Status:          status,
				ConfigVersionID: configVersion,
				CreatedAt:       startTime,
			})
		}
	}

	initSettlement := func(settlementID, intentID, paymentMethod, status, payer, payee, nonce, txHash string) {
		if p.paymentService != nil {
			_ = p.paymentService.RecordSettlement(r.Context(), &payment.SettlementRecord{
				SettlementID:    settlementID,
				IntentID:        intentID,
				PaymentMethod:   paymentMethod,
				NetworkID:       route.Network,
				AssetAddress:    route.Asset,
				PayerAddress:    payer,
				PayeeAddress:    payee,
				AmountBaseUnits: route.PriceBaseUnits,
				Nonce:           nonce,
				TxHash:          txHash,
				Status:          status,
				CreatedAt:       startTime,
			})
		}
	}

	initFulfillment := func(fulfillmentID, intentID string) {
		if p.paymentService != nil {
			_ = p.paymentService.RecordFulfillment(r.Context(), &payment.FulfillmentRecord{
				FulfillmentID:  fulfillmentID,
				IntentID:       intentID,
				UpstreamURL:    route.UpstreamURL,
				Status:         payment.FulfillmentStatusInProgress,
				CreatedAt:      startTime,
			})
		}
	}

	updateCorrelationOutcome := func(intentID, settlementID, fulfillmentID string, statusCode int) {
		if p.paymentService != nil {
			durationMs := time.Since(startTime).Milliseconds()
			if statusCode >= 200 && statusCode < 300 {
				_ = p.paymentService.UpdateSettlementStatus(r.Context(), settlementID, payment.SettlementStatusConfirmed, "", nil, "")
				_ = p.paymentService.UpdateFulfillment(r.Context(), fulfillmentID, payment.FulfillmentStatusSucceeded, statusCode, durationMs, "")
				_ = p.paymentService.UpdateIntentStatus(r.Context(), intentID, payment.IntentStatusCompleted)
			} else if statusCode >= 400 && statusCode < 500 {
				_ = p.paymentService.UpdateFulfillment(r.Context(), fulfillmentID, payment.FulfillmentStatusFailed, statusCode, durationMs, "")
				_ = p.paymentService.UpdateIntentStatus(r.Context(), intentID, payment.IntentStatusFailed)
			} else {
				_ = p.paymentService.UpdateFulfillment(r.Context(), fulfillmentID, payment.FulfillmentStatusUnknown, statusCode, durationMs, "")
				_ = p.paymentService.UpdateIntentStatus(r.Context(), intentID, payment.IntentStatusNeedsReview)
			}
		}
	}

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
		resp, _ := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
		recordReq(route.TenantID, "prj_default", "", "", "", resp.StatusCode, "", false)
		return
	}

	// Buffer request body for deterministic hash computation & replay
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	requestHash := idempotency.ComputeRequestHash(r.Method, r.URL.Path, r.URL.RawQuery, bodyBytes)
	idemKey := r.Header.Get(HeaderIdempotencyKey)

	authHeader := r.Header.Get(HeaderPaymentSignature)
	bearerHeader := r.Header.Get("Authorization")
	apiKeyHeader := r.Header.Get("X-API-Key")

	// Determine client-provided credentials
	var apiKey string
	if strings.HasPrefix(strings.ToLower(bearerHeader), "bearer ") {
		apiKey = strings.TrimSpace(bearerHeader[7:])
	} else if apiKeyHeader != "" {
		apiKey = apiKeyHeader
	}

	// ------------------------------------------------------------------------
	// PATH 1: Virtual Wallet Payment (API Key Presented)
	// ------------------------------------------------------------------------
	if apiKey != "" {
		allowed := route.AllowedPaymentMethods
		if allowed == "onchain" {
			recordReq(route.TenantID, "prj_default", apiKey, "", "", http.StatusMethodNotAllowed, ErrCodePaymentMethodNotAllowed, false)
			WriteProtocolError(w, ErrCodePaymentMethodNotAllowed, requestID, map[string]interface{}{
				"route_id":                route.RouteID,
				"allowed_payment_methods": allowed,
			})
			return
		}

		wallet, err := p.walletService.GetWalletByAPIKey(r.Context(), apiKey)
		if err != nil {
			recordReq(route.TenantID, "prj_default", apiKey, "", "", http.StatusUnauthorized, ErrCodeAPIKeyInvalid, false)
			WriteProtocolError(w, ErrCodeAPIKeyInvalid, requestID, nil)
			return
		}

		if wallet.Status == "frozen_arrears" || wallet.Status == "frozen" {
			recordReq(wallet.TenantID, "prj_default", apiKey, "", "", http.StatusPaymentRequired, ErrCodeWalletFrozen, false)
			WriteProtocolError(w, ErrCodeWalletFrozen, requestID, map[string]interface{}{
				"wallet_id": wallet.WalletID,
				"status":    wallet.Status,
			})
			return
		}

		// 1. Agent resolution and context sanitization (T2.9 & T1.8)
		declared := agent.ExtractDeclaredContext(r)
		agent.SanitizeAndStripSpoofedAgentHeaders(r.Header)
		userAgent := r.UserAgent()

		resolvedAgent, agentPolicy, _ := p.agentRegistry.ResolveAPIKey(apiKey)
		if resolvedAgent == nil {
			resolvedAgent = &agent.AgentIdentity{
				AgentID:   "agt_wallet_" + wallet.WalletID,
				TenantID:  wallet.TenantID,
				ProjectID: "prj_default",
				Name:      "DefaultAgent",
				Status:    agent.AgentStatusActive,
			}
		}

		intentID := fmt.Sprintf("pi_%s", requestID)
		correlation := agent.CorrelationContext{
			RequestID:      requestID,
			IdempotencyKey: idemKey,
			IntentID:       intentID,
		}

		// 2. Synchronous In-Path Agent Policy Enforcement (T2.9)
		decision := p.agentEnforcer.Evaluate(r.Context(), resolvedAgent, agentPolicy, route.RouteID, r.URL.Path, route.PriceBaseUnits)
		if p.auditEmitter != nil {
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
			p.metrics.IncPaymentsFailed()
			initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusRejected)
			recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, apiKey, resolvedAgent.AgentID, intentID, http.StatusForbidden, decision.ReasonCode, false)
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
			if p.auditEmitter != nil {
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
			recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, apiKey, resolvedAgent.AgentID, intentID, statusCode, errCode, isReplay)
			if !isReplay && correlation.HoldID != "" {
				settlementID := fmt.Sprintf("stl_%s", correlation.HoldID)
				fulfillmentID := fmt.Sprintf("ful_%s", requestID)
				updateCorrelationOutcome(intentID, settlementID, fulfillmentID, statusCode)
			}
		}

		// Helper closure to emit agent.spend audit event
		emitSpendAudit := func(holdID string, action string) {
			correlation.HoldID = holdID
			if p.auditEmitter != nil {
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
				initSettlement(settlementID, intentID, "wallet", payment.SettlementStatusHeld, wallet.WalletID, route.PayeeAddress, "", holdID)
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
				recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, apiKey, resolvedAgent.AgentID, intentID, http.StatusBadGateway, ErrCodeUpstreamBadGateway, false)
				WriteProtocolError(w, ErrCodeUpstreamBadGateway, requestID, map[string]interface{}{"error": err.Error()})
				return
			}

			// Case A: Conflict (in-flight or payload mismatch)
			if checkRes == idempotency.ResultConflict {
				recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, apiKey, resolvedAgent.AgentID, intentID, http.StatusConflict, ErrCodeIdempotencyConflict, false)
				WriteProtocolError(w, ErrCodeIdempotencyConflict, requestID, map[string]interface{}{
					"idempotency_key": idemKey,
					"reason":          "Idempotency conflict: request in-progress or payload altered",
				})
				return
			}

			// Case B: Cache Hit (already succeeded previously)
			if checkRes == idempotency.ResultCacheHit {
				WriteCachedResponse(w, rec)
				emitRequestAudit(rec.StatusCode, true, "")
				return
			}

			// Case C: Free Replay (previous attempt failed upstream with 5xx or timeout)
			if checkRes == idempotency.ResultFreeReplay {
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

				resp, _ := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
				emitRequestAudit(resp.StatusCode, true, "")

				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					_ = p.walletService.CaptureFunds(r.Context(), rec.HoldID)
					p.agentEnforcer.RecordSpend(resolvedAgent.AgentID, route.PriceBaseUnits)
					emitSpendAudit(rec.HoldID, "capture")
					p.metrics.IncHoldsCaptured()
					p.metrics.IncPaymentsVerified()
					_ = p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body)
				} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
					_ = p.walletService.ReleaseFunds(r.Context(), rec.HoldID)
					emitSpendAudit(rec.HoldID, "release")
					p.metrics.IncHoldsReleased()
					_ = p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body)
				} else {
					// 5xx or Timeout: KEEP HELD, Mark Recovery Required (strictly no auto-release or double capture)
					_ = p.idempotencyStore.MarkRecoveryRequired(r.Context(), tenantID, idemKey, resp.StatusCode, "upstream failure")
				}
				return
			}

			// Case D: ResultNew (First time receiving this idempotent request)
			if wallet.AvailableBalance < route.PriceBaseUnits {
				_ = p.idempotencyStore.ReleaseLock(r.Context(), tenantID, idemKey)
				p.metrics.IncPaymentsFailed()
				initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusFailed)
				initSettlement(fmt.Sprintf("stl_%s", requestID), intentID, "wallet", payment.SettlementStatusFailed, wallet.WalletID, route.PayeeAddress, "", "")
				recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, apiKey, resolvedAgent.AgentID, intentID, http.StatusPaymentRequired, ErrCodeWalletInsufficientBalance, false)
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
				_ = p.idempotencyStore.ReleaseLock(r.Context(), tenantID, idemKey)
				p.metrics.IncPaymentsFailed()
				initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusFailed)
				initSettlement(fmt.Sprintf("stl_%s", requestID), intentID, "wallet", payment.SettlementStatusFailed, wallet.WalletID, route.PayeeAddress, "", "")
				recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, apiKey, resolvedAgent.AgentID, intentID, http.StatusPaymentRequired, ErrCodeWalletInsufficientBalance, false)
				WriteProtocolError(w, ErrCodeWalletInsufficientBalance, requestID, nil)
				return
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

			resp, _ := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
			emitRequestAudit(resp.StatusCode, false, "")

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				_ = p.walletService.CaptureFunds(r.Context(), holdID)
				p.agentEnforcer.RecordSpend(resolvedAgent.AgentID, route.PriceBaseUnits)
				emitSpendAudit(holdID, "capture")
				p.metrics.IncHoldsCaptured()
				p.metrics.IncPaymentsVerified()
				_ = p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body)
			} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				_ = p.walletService.ReleaseFunds(r.Context(), holdID)
				emitSpendAudit(holdID, "release")
				p.metrics.IncHoldsReleased()
				_ = p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body)
			} else {
				// 5xx / Timeout: KEEP HELD, Mark Recovery Required
				_ = p.idempotencyStore.MarkRecoveryRequired(r.Context(), tenantID, idemKey, resp.StatusCode, "upstream failure")
			}
			return
		}

		// 1.2 Non-Idempotent Request Path
		if wallet.AvailableBalance < route.PriceBaseUnits {
			p.metrics.IncPaymentsFailed()
			initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusFailed)
			initSettlement(fmt.Sprintf("stl_%s", requestID), intentID, "wallet", payment.SettlementStatusFailed, wallet.WalletID, route.PayeeAddress, "", "")
			recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, apiKey, resolvedAgent.AgentID, intentID, http.StatusPaymentRequired, ErrCodeWalletInsufficientBalance, false)
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
			p.metrics.IncPaymentsFailed()
			initIntent(resolvedAgent.TenantID, resolvedAgent.ProjectID, resolvedAgent.AgentID, intentID, idemKey, "wallet", payment.IntentStatusFailed)
			initSettlement(fmt.Sprintf("stl_%s", requestID), intentID, "wallet", payment.SettlementStatusFailed, wallet.WalletID, route.PayeeAddress, "", "")
			recordReq(resolvedAgent.TenantID, resolvedAgent.ProjectID, apiKey, resolvedAgent.AgentID, intentID, http.StatusPaymentRequired, ErrCodeWalletInsufficientBalance, false)
			WriteProtocolError(w, ErrCodeWalletInsufficientBalance, requestID, nil)
			return
		}
		p.metrics.IncHoldsCreated()
		emitSpendAudit(holdID, "hold")

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

		resp, _ := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
		emitRequestAudit(resp.StatusCode, false, "")

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			_ = p.walletService.CaptureFunds(r.Context(), holdID)
			p.agentEnforcer.RecordSpend(resolvedAgent.AgentID, route.PriceBaseUnits)
			emitSpendAudit(holdID, "capture")
			p.metrics.IncHoldsCaptured()
			p.metrics.IncPaymentsVerified()
		} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			_ = p.walletService.ReleaseFunds(r.Context(), holdID)
			emitSpendAudit(holdID, "release")
			p.metrics.IncHoldsReleased()
		}
		// In non-idempotent 5xx, hold remains preserved for manual reconciliation/review
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

		network := "eip155:84532"
		intentID := fmt.Sprintf("pi_%s", requestID)
		settlementID := fmt.Sprintf("stl_%s", auth.Nonce)

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
					Asset:           route.Asset,
					Network:         network,
					TxHash:          auth.Nonce,
					IdempotencyKey:  idemKey,
					RequestID:       requestID,
					ConfigVersion:   configVersion,
					ConfigHash:      configHash,
				}
				resp, _ := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
				recordReq(route.TenantID, "prj_default", "", "", intentID, resp.StatusCode, "", true)
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					_ = p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body)
				} else {
					_ = p.idempotencyStore.MarkRecoveryRequired(r.Context(), tenantID, idemKey, resp.StatusCode, "upstream failure")
				}
				return
			}

			// ResultNew: Validate Nonce & Signature
			validNonce, err := p.nonceStore.CheckAndRecordNonce(network, route.Asset, auth.Payer, auth.Nonce)
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

			ch := &PaymentChallenge{
				Version:     auth.Version,
				ChallengeID: auth.ChallengeID,
				Network:     network,
				Asset:       route.Asset,
				Amount:      fmt.Sprintf("%d", route.PriceBaseUnits),
				Payee:       route.PayeeAddress,
				ValidAfter:  auth.ValidAfter,
				ValidBefore: auth.ValidBefore,
			}

			result, err := p.verifier.VerifyPayment(r.Context(), ch, auth)
			if err != nil || !result.Valid {
				_ = p.idempotencyStore.ReleaseLock(r.Context(), tenantID, idemKey)
				p.metrics.IncPaymentsFailed()
				errCode := ErrCodeSignatureInvalid
				if result != nil && result.ErrorCode != "" {
					errCode = result.ErrorCode
				}
				initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusFailed)
				initSettlement(settlementID, intentID, "onchain", payment.SettlementStatusFailed, auth.Payer, route.PayeeAddress, auth.Nonce, "")
				recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusPaymentRequired, errCode, false)
				WriteProtocolError(w, errCode, requestID, map[string]interface{}{
					"reason": result.Message,
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
				Amount:          route.PriceBaseUnits,
				Asset:           route.Asset,
				Network:         network,
				TxHashOrNonce:   auth.Nonce,
				Status:          recovery.SettlementSubmitted,
			})
			_ = p.idempotencyStore.AttachPaymentDetails(r.Context(), tenantID, idemKey, "", settlementID)
			p.metrics.IncPaymentsVerified()

			pCtx := &proxy.PaymentContext{
				PayerAddress:    result.Payer,
				AmountBaseUnits: route.PriceBaseUnits,
				Asset:           route.Asset,
				Network:         network,
				TxHash:          auth.Nonce,
				IdempotencyKey:  idemKey,
				RequestID:       requestID,
				ConfigVersion:   configVersion,
				ConfigHash:      configHash,
			}

			resp, _ := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
			updateCorrelationOutcome(intentID, settlementID, fulfillmentID, resp.StatusCode)
			recordReq(route.TenantID, "prj_default", "", "", intentID, resp.StatusCode, "", false)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				_ = p.idempotencyStore.MarkCompleted(r.Context(), tenantID, idemKey, resp.StatusCode, resp.Headers, resp.Body)
				recordUsage(route.TenantID, "prj_default", "", intentID, "onchain", true)
			} else {
				// Anti-Rollback: Record remains SUBMITTED, Mark recovery required
				_ = p.recoveryCoord.MarkUnknown(settlementID, "upstream error")
				_ = p.idempotencyStore.MarkRecoveryRequired(r.Context(), tenantID, idemKey, resp.StatusCode, "upstream failure")
				recordUsage(route.TenantID, "prj_default", "", intentID, "onchain", false)
			}
			return
		}

		// 2.2 Non-Idempotent On-Chain Request
		validNonce, err := p.nonceStore.CheckAndRecordNonce(network, route.Asset, auth.Payer, auth.Nonce)
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

		ch := &PaymentChallenge{
			Version:     auth.Version,
			ChallengeID: auth.ChallengeID,
			Network:     network,
			Asset:       route.Asset,
			Amount:      fmt.Sprintf("%d", route.PriceBaseUnits),
			Payee:       route.PayeeAddress,
			ValidAfter:  auth.ValidAfter,
			ValidBefore: auth.ValidBefore,
		}

		result, err := p.verifier.VerifyPayment(r.Context(), ch, auth)
		if err != nil || !result.Valid {
			p.metrics.IncPaymentsFailed()
			errCode := ErrCodeSignatureInvalid
			if result != nil && result.ErrorCode != "" {
				errCode = result.ErrorCode
			}
			initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusFailed)
			initSettlement(settlementID, intentID, "onchain", payment.SettlementStatusFailed, auth.Payer, route.PayeeAddress, auth.Nonce, "")
			recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusPaymentRequired, errCode, false)
			WriteProtocolError(w, errCode, requestID, map[string]interface{}{
				"reason": result.Message,
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
			AmountBaseUnits: route.PriceBaseUnits,
			Asset:           route.Asset,
			Network:         network,
			TxHash:          auth.Nonce,
			RequestID:       requestID,
			ConfigVersion:   configVersion,
			ConfigHash:      configHash,
		}

		resp, _ := p.proxyHandler.ForwardWithCapture(w, r, route, pCtx)
		updateCorrelationOutcome(intentID, settlementID, fulfillmentID, resp.StatusCode)
		recordReq(route.TenantID, "prj_default", "", "", intentID, resp.StatusCode, "", false)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			recordUsage(route.TenantID, "prj_default", "", intentID, "onchain", true)
		} else {
			recordUsage(route.TenantID, "prj_default", "", intentID, "onchain", false)
		}
		return
	}

	// ------------------------------------------------------------------------
	// PATH 3: Unauthenticated -> Issue HTTP 402 Payment Required Challenge
	// ------------------------------------------------------------------------
	intentID := fmt.Sprintf("pi_%s", requestID)
	initIntent(route.TenantID, "prj_default", "", intentID, idemKey, "onchain", payment.IntentStatusChallenged)
	recordReq(route.TenantID, "prj_default", "", "", intentID, http.StatusPaymentRequired, ErrCodePaymentRequired, false)
	p.metrics.IncChallenges()
	network := "eip155:84532"
	if route.Network != "" {
		network = route.Network
	}
	ch := NewChallenge(
		network,
		route.Asset,
		"USDC",
		6,
		fmt.Sprintf("%d", route.PriceBaseUnits),
		route.PayeeAddress,
		"http://127.0.0.1:8001",
		r.URL.Path,
		300*time.Second,
	)

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
