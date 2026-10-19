package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/agent"
	"github.com/open-agentpay/agentpay-gateway-go/internal/auth"
	"github.com/open-agentpay/agentpay-gateway-go/internal/payment"
	"github.com/open-agentpay/agentpay-gateway-go/internal/rcu"
	"github.com/open-agentpay/agentpay-gateway-go/internal/timeline"
	"github.com/open-agentpay/agentpay-gateway-go/internal/upstreamauth"
	"github.com/open-agentpay/agentpay-gateway-go/internal/usage"
	"github.com/open-agentpay/agentpay-gateway-go/internal/wallet"
)

// LargeAdjustmentThreshold defines amount beyond which dual control / audit alert is needed.
const LargeAdjustmentThreshold int64 = 100_000_000 // 100 USDC

// Service defines the Control Plane management operations.
type Service interface {
	// Auth
	Register(ctx context.Context, tenantName, email, password string) (*Tenant, *Member, error)
	Login(ctx context.Context, email, password string) (*Member, *Tenant, error)
	GetMemberByEmail(ctx context.Context, email string) (*Member, *Tenant, error)
	SetMemberPassword(ctx context.Context, tenantID, memberID, password string) error

	// Tenant
	CreateTenant(ctx context.Context, name string, tenantID string, ownerEmail string) (*Tenant, *Member, error)
	GetTenant(ctx context.Context, tenantID string) (*Tenant, error)
	ListTenants(ctx context.Context) ([]*Tenant, error)
	UpdateTenant(ctx context.Context, tenantID string, name string, actorMemberID string) (*Tenant, error)
	UpdateTenantStatus(ctx context.Context, tenantID string, status TenantStatus, actorMemberID string) (*Tenant, error)

	// Project
	CreateProject(ctx context.Context, tenantID, name string, budgetCap int64, projectID string, actorMemberID string) (*Project, error)
	GetProject(ctx context.Context, tenantID, projectID string) (*Project, error)
	ListProjects(ctx context.Context, tenantID string) ([]*Project, error)
	UpdateProject(ctx context.Context, tenantID, projectID string, name string, budgetCap *int64, status *ProjectStatus, actorMemberID string) (*Project, error)
	DeleteProject(ctx context.Context, tenantID, projectID string, actorMemberID string) error

	// Member
	AddMember(ctx context.Context, tenantID, email string, role MemberRole, actorMemberID string) (*Member, error)
	GetMember(ctx context.Context, tenantID, memberID string) (*Member, error)
	ListMembers(ctx context.Context, tenantID string) ([]*Member, error)
	UpdateMember(ctx context.Context, tenantID, memberID string, role *MemberRole, status *MemberStatus, actorMemberID string) (*Member, error)
	RemoveMember(ctx context.Context, tenantID, memberID string, actorMemberID string) error

	// Agent & Kill Switch
	CreateAgent(ctx context.Context, tenantID, projectID, name string, policyCfg map[string]interface{}, actorMemberID string) (*Agent, error)
	GetAgent(ctx context.Context, tenantID, agentID string) (*Agent, error)
	ListAgents(ctx context.Context, tenantID, projectID string, status *AgentStatus) ([]*Agent, error)
	UpdateAgent(ctx context.Context, tenantID, agentID string, name string, policyCfg map[string]interface{}, actorMemberID string) (*Agent, error)
	UpdateAgentStatus(ctx context.Context, tenantID, agentID string, status AgentStatus, actorMemberID string) (*Agent, error)
	KillAgent(ctx context.Context, tenantID, agentID, reason string, actorMemberID string, revoke bool, clientIP string) (*Agent, []string, error)
	DeleteAgent(ctx context.Context, tenantID, agentID, actorMemberID string) error

	// API Keys
	CreateAPIKey(ctx context.Context, tenantID, projectID, agentID, name string, scopes []string, expiresAt *int64, actorMemberID string) (*CreateAPIKeyResult, error)
	GetAPIKey(ctx context.Context, tenantID, keyID string) (*APIKey, error)
	ListAPIKeys(ctx context.Context, tenantID, agentID string) ([]*APIKey, error)
	RevokeAPIKey(ctx context.Context, tenantID, keyID, actorMemberID string) (*APIKey, error)
	RotateAPIKey(ctx context.Context, tenantID, keyID, actorMemberID string) (*CreateAPIKeyResult, error)

	// Routes
	CreateRoute(ctx context.Context, route *Route, actorMemberID string) (*Route, error)
	GetRoute(ctx context.Context, tenantID, routeID string) (*Route, error)
	ListRoutes(ctx context.Context, tenantID string) ([]*Route, error)
	UpdateRoute(ctx context.Context, input *UpdateRouteInput, actorMemberID string) (*Route, error)
	DeleteRoute(ctx context.Context, tenantID, routeID, actorMemberID string) error

	// Policies
	CreatePolicy(ctx context.Context, pol *Policy, actorMemberID string) (*Policy, error)
	GetPolicy(ctx context.Context, tenantID, policyID string) (*Policy, error)
	ListPolicies(ctx context.Context, tenantID, projectID, agentID string) ([]*Policy, error)
	UpdatePolicy(ctx context.Context, pol *Policy, actorMemberID string) (*Policy, error)
	DeletePolicy(ctx context.Context, tenantID, policyID, actorMemberID string) error

	// Config Draft & Validation (T4.3)
	ValidateConfig(ctx context.Context, tenantID string, actorMemberID string) (*ValidationResult, error)
	GetDraftSummary(ctx context.Context, tenantID string) (*ConfigDraftSummary, error)
	GetDraftDiff(ctx context.Context, tenantID string) (*ConfigDraftDiff, error)

	// Config Publishing, Versions & Rollback (T4.4)
	PublishConfig(ctx context.Context, tenantID string, changeSummary string, actorMemberID string) (*ConfigVersion, error)
	ListConfigVersions(ctx context.Context, tenantID string) ([]*ConfigVersion, error)
	GetConfigVersion(ctx context.Context, tenantID, versionID string) (*ConfigVersion, error)
	RollbackConfig(ctx context.Context, tenantID, targetVersionID, reason string, actorMemberID string) (*ConfigVersion, error)
	GetConfigVersionDiff(ctx context.Context, tenantID, baseVersionID, targetVersionID string) (*ConfigDraftDiff, error)

	// Upstream Auth & Connectivity (T4.2 Enhancement)
	GetAuthTypes() []upstreamauth.AuthTypeSchema
	TestUpstream(ctx context.Context, tenantID string, input *TestUpstreamInput, actorMemberID string) (*TestUpstreamResult, error)

	// Wallet Admin
	AdminGrantWallet(ctx context.Context, tenantID string, amount int64, reason string, idempotencyKey string, actorMemberID string, clientIP string) error
	AdjustWallet(ctx context.Context, walletID string, amount int64, reason string, idempotencyKey string, actorMemberID string, clientIP string) error
	FreezeWallet(ctx context.Context, walletID string, reason string, actorMemberID string, clientIP string) error
	UnfreezeWallet(ctx context.Context, walletID string, reason string, actorMemberID string, clientIP string) error
	RecalculateWallet(ctx context.Context, walletID string, actorMemberID string) (map[string]interface{}, error)
	GetWallet(ctx context.Context, tenantID string) (*wallet.Wallet, error)
	GetWalletByID(ctx context.Context, walletID string) (*wallet.Wallet, error)
	GetLedgerEntries(ctx context.Context, walletID string) ([]*wallet.LedgerEntry, error)
	GetTopUpOrders(ctx context.Context, walletID string) ([]*wallet.TopUpOrder, error)

	// Audit
	ListAuditEvents(ctx context.Context, tenantID, targetType, targetID string, limit int) ([]*AuditEvent, error)

	// Usage & Request Tracing (T5.1)
	SetUsageService(svc usage.Service)
	GetUsageService() usage.Service
	ListRequestRecords(ctx context.Context, tenantID string, filter usage.RequestFilter, actorMemberID string) ([]*usage.RequestRecord, int64, error)
	GetRequestRecord(ctx context.Context, tenantID, requestID string, actorMemberID string) (*usage.RequestRecord, error)
	ListUsageEvents(ctx context.Context, tenantID string, filter usage.UsageFilter, actorMemberID string) ([]*usage.UsageEvent, int64, error)
	GetUsageEvent(ctx context.Context, tenantID, eventID string, actorMemberID string) (*usage.UsageEvent, error)

	// Payment, Settlement & Fulfillment Correlation (T5.2)
	SetPaymentService(svc payment.Service)
	GetPaymentService() payment.Service
	GetPaymentAudit(ctx context.Context, tenantID, intentID string, actorMemberID string) (*payment.PaymentAuditRecord, error)
	GetPaymentAuditByRequestID(ctx context.Context, tenantID, requestID string, actorMemberID string) (*payment.PaymentAuditRecord, error)
	ListPaymentAudits(ctx context.Context, tenantID string, filter payment.PaymentAuditFilter, actorMemberID string) ([]*payment.PaymentAuditRecord, int64, error)

	// Agent Behavior Timeline (T5.8)
	SetTimelineService(svc timeline.Service)
	GetTimelineService() timeline.Service
	GetAgentTimeline(ctx context.Context, tenantID, agentID string, filter timeline.TimelineFilter, actorMemberID string) ([]*timeline.TimelineEvent, int64, []string, error)

	// RCU Data Plane Synchronization
	SetRCUManager(mgr *rcu.Manager)
	SyncRCU(ctx context.Context) error
}

// MemoryService provides an in-memory thread-safe implementation of Service.
type MemoryService struct {
	mu                   sync.RWMutex
	tenants              map[string]*Tenant
	projects             map[string]*Project
	members              map[string]*Member
	agents               map[string]*Agent
	apiKeys              map[string]*APIKey
	routes               map[string]*Route
	policies             map[string]*Policy
	auditEvents          []*AuditEvent
	agentRegistry        *agent.Registry
	walletSvc            wallet.Service
	usageSvc             usage.Service
	paymentSvc           payment.Service
	timelineSvc          timeline.Service
	lastValidation       map[string]*ValidationResult
	configVersions       map[string]*ConfigVersion
	configVersionHistory map[string][]*ConfigVersion
	rcuMgr               *rcu.Manager
}

// NewMemoryService constructs an in-memory control plane service.
func NewMemoryService(agentRegistry *agent.Registry, walletSvc wallet.Service) *MemoryService {
	if agentRegistry == nil {
		agentRegistry = agent.NewRegistry()
	}
	if walletSvc == nil {
		walletSvc = wallet.NewMemoryService()
	}
	return &MemoryService{
		tenants:              make(map[string]*Tenant),
		projects:             make(map[string]*Project),
		members:              make(map[string]*Member),
		agents:               make(map[string]*Agent),
		apiKeys:              make(map[string]*APIKey),
		routes:               make(map[string]*Route),
		policies:             make(map[string]*Policy),
		auditEvents:          make([]*AuditEvent, 0),
		agentRegistry:        agentRegistry,
		walletSvc:            walletSvc,
		timelineSvc:          timeline.NewMemoryService(),
		lastValidation:       make(map[string]*ValidationResult),
		configVersions:       make(map[string]*ConfigVersion),
		configVersionHistory: make(map[string][]*ConfigVersion),
	}
}

func (s *MemoryService) recordAudit(tenantID, operatorID, action, targetType, targetID string, before, after map[string]interface{}, clientIP string) {
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}
	ev := &AuditEvent{
		AuditID:     GenerateAuditID(),
		TenantID:    tenantID,
		OperatorID:  operatorID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		BeforeState: before,
		AfterState:  after,
		ClientIP:    clientIP,
		CreatedAt:   time.Now().Unix(),
	}
	s.auditEvents = append(s.auditEvents, ev)
}

// ----------------------------------------------------------------------------
// Auth
// ----------------------------------------------------------------------------

func (s *MemoryService) Register(ctx context.Context, tenantName, email, password string) (*Tenant, *Member, error) {
	cleanName := strings.TrimSpace(tenantName)
	cleanEmail := strings.TrimSpace(email)
	if cleanName == "" {
		return nil, nil, fmt.Errorf("tenant name cannot be empty")
	}
	if cleanEmail == "" {
		return nil, nil, fmt.Errorf("email cannot be empty")
	}
	if len(password) < 8 {
		return nil, nil, ErrPasswordTooShort
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.tenants {
		if t.Name == cleanName {
			return nil, nil, ErrTenantAlreadyExists
		}
	}
	for _, m := range s.members {
		if strings.EqualFold(m.Email, cleanEmail) {
			return nil, nil, ErrEmailAlreadyRegistered
		}
	}

	tid := GenerateTenantID()
	now := time.Now().Unix()
	t := &Tenant{
		TenantID:  tid,
		Name:      cleanName,
		Status:    TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.tenants[tid] = t

	ownerMember := &Member{
		MemberID:     GenerateMemberID(),
		TenantID:     tid,
		Email:        cleanEmail,
		PasswordHash: hash,
		Role:         RoleOwner,
		Status:       MemberStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.members[ownerMember.MemberID] = ownerMember

	// Initialize master wallet
	if s.walletSvc != nil {
		memWlt, ok := s.walletSvc.(*wallet.MemoryService)
		if ok {
			memWlt.RegisterWallet(&wallet.Wallet{
				WalletID:       fmt.Sprintf("wlt_%s", tid),
				TenantID:       tid,
				Asset:          "USDC",
				Status:         wallet.StatusActive,
				Version:        1,
				AvailableCache: 0,
				HeldCache:      0,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			})
		}
	}

	// Initialize default project for new tenant
	defaultPrjID := GenerateProjectID()
	defaultProject := &Project{
		ProjectID:          defaultPrjID,
		TenantID:           tid,
		Name:               "默认项目 (Default)",
		BudgetCapBaseUnits: 1000000000, // 1000 USDC default budget cap
		Status:             ProjectStatusActive,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.projects[defaultPrjID] = defaultProject

	s.recordAudit(tid, ownerMember.MemberID, "auth.register", "Tenant", tid, nil, map[string]interface{}{"email": cleanEmail, "tenant": cleanName, "project_id": defaultPrjID}, "127.0.0.1")
	return t, ownerMember, nil
}

func (s *MemoryService) Login(ctx context.Context, email, password string) (*Member, *Tenant, error) {
	cleanEmail := strings.TrimSpace(email)
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matchedMember *Member
	for _, m := range s.members {
		if strings.EqualFold(m.Email, cleanEmail) {
			matchedMember = m
			break
		}
	}
	if matchedMember == nil {
		return nil, nil, ErrInvalidCredentials
	}

	if matchedMember.Status != MemberStatusActive {
		return nil, nil, ErrMemberDisabled
	}

	tenant, ok := s.tenants[matchedMember.TenantID]
	if !ok || tenant.Status != TenantStatusActive {
		return nil, nil, ErrTenantSuspended
	}

	if matchedMember.PasswordHash == "" || !auth.VerifyPassword(password, matchedMember.PasswordHash) {
		return nil, nil, ErrInvalidCredentials
	}

	return matchedMember, tenant, nil
}

func (s *MemoryService) GetMemberByEmail(ctx context.Context, email string) (*Member, *Tenant, error) {
	cleanEmail := strings.TrimSpace(email)
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.members {
		if strings.EqualFold(m.Email, cleanEmail) {
			t := s.tenants[m.TenantID]
			return m, t, nil
		}
	}
	return nil, nil, ErrMemberNotFound
}

func (s *MemoryService) SetMemberPassword(ctx context.Context, tenantID, memberID, password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.members[memberID]
	if !ok || m.TenantID != tenantID {
		return ErrMemberNotFound
	}
	m.PasswordHash = hash
	m.UpdatedAt = time.Now().Unix()
	return nil
}

// ----------------------------------------------------------------------------
// Tenants
// ----------------------------------------------------------------------------

func (s *MemoryService) CreateTenant(ctx context.Context, name string, tenantID string, ownerEmail string) (*Tenant, *Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, nil, fmt.Errorf("tenant name cannot be empty")
	}

	for _, t := range s.tenants {
		if t.Name == cleanName {
			return nil, nil, ErrTenantAlreadyExists
		}
	}

	tid := tenantID
	if tid == "" {
		tid = GenerateTenantID()
	}

	now := time.Now().Unix()
	t := &Tenant{
		TenantID:  tid,
		Name:      cleanName,
		Status:    TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.tenants[tid] = t

	var ownerMember *Member
	if ownerEmail != "" {
		ownerMember = &Member{
			MemberID:  GenerateMemberID(),
			TenantID:  tid,
			Email:     strings.TrimSpace(ownerEmail),
			Role:      RoleOwner,
			Status:    MemberStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		s.members[ownerMember.MemberID] = ownerMember
	}

	// Initialize master wallet
	if s.walletSvc != nil {
		memWlt, ok := s.walletSvc.(*wallet.MemoryService)
		if ok {
			memWlt.RegisterWallet(&wallet.Wallet{
				WalletID:       fmt.Sprintf("wlt_%s", tid),
				TenantID:       tid,
				Asset:          "USDC",
				Status:         wallet.StatusActive,
				Version:        1,
				AvailableCache: 0,
				HeldCache:      0,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			})
		}
	}

	s.recordAudit(tid, "system", "tenant.created", "Tenant", tid, nil, map[string]interface{}{"name": cleanName}, "127.0.0.1")
	return t, ownerMember, nil
}

func (s *MemoryService) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}
	return t, nil
}

func (s *MemoryService) ListTenants(ctx context.Context) ([]*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		res = append(res, t)
	}
	return res, nil
}

func (s *MemoryService) UpdateTenant(ctx context.Context, tenantID string, name string, actorMemberID string) (*Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionTenantUpdate); err != nil {
			return nil, err
		}
	}

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	before := map[string]interface{}{"name": t.Name}
	t.Name = cleanName
	t.UpdatedAt = time.Now().Unix()

	s.recordAudit(tenantID, actorMemberID, "tenant.updated", "Tenant", tenantID, before, map[string]interface{}{"name": cleanName}, "127.0.0.1")
	return t, nil
}

func (s *MemoryService) UpdateTenantStatus(ctx context.Context, tenantID string, status TenantStatus, actorMemberID string) (*Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}

	if t.Status == TenantStatusTerminated && status != TenantStatusTerminated {
		return nil, ErrInvalidStatusTransition
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID || actor.Role != RoleOwner {
			return nil, ErrPermissionDenied
		}
	}

	before := map[string]interface{}{"status": string(t.Status)}
	t.Status = status
	t.UpdatedAt = time.Now().Unix()

	s.recordAudit(tenantID, actorMemberID, "tenant.status.changed", "Tenant", tenantID, before, map[string]interface{}{"status": string(status)}, "127.0.0.1")
	return t, nil
}

// ----------------------------------------------------------------------------
// Projects
// ----------------------------------------------------------------------------

func (s *MemoryService) CreateProject(ctx context.Context, tenantID, name string, budgetCap int64, projectID string, actorMemberID string) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant, ok := s.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}
	if tenant.Status == TenantStatusSuspended {
		return nil, ErrTenantSuspended
	}
	if tenant.Status == TenantStatusTerminated {
		return nil, ErrTenantTerminated
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionProjectCreate); err != nil {
			return nil, err
		}
	}

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	for _, p := range s.projects {
		if p.TenantID == tenantID && p.Name == cleanName {
			return nil, ErrProjectAlreadyExists
		}
	}

	pid := projectID
	if pid == "" {
		pid = GenerateProjectID()
	}

	now := time.Now().Unix()
	p := &Project{
		ProjectID:          pid,
		TenantID:           tenantID,
		Name:               cleanName,
		BudgetCapBaseUnits: budgetCap,
		Status:             ProjectStatusActive,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.projects[pid] = p

	s.recordAudit(tenantID, actorMemberID, "project.created", "Project", pid, nil, map[string]interface{}{"name": cleanName}, "127.0.0.1")
	return p, nil
}

func (s *MemoryService) GetProject(ctx context.Context, tenantID, projectID string) (*Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.projects[projectID]
	if !ok {
		return nil, ErrProjectNotFound
	}
	if tenantID != "" && p.TenantID != tenantID {
		return nil, ErrProjectTenantMismatch
	}
	return p, nil
}

func (s *MemoryService) ListProjects(ctx context.Context, tenantID string) ([]*Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Project, 0)
	for _, p := range s.projects {
		if tenantID == "" || p.TenantID == tenantID {
			res = append(res, p)
		}
	}
	return res, nil
}

func (s *MemoryService) UpdateProject(ctx context.Context, tenantID, projectID string, name string, budgetCap *int64, status *ProjectStatus, actorMemberID string) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.projects[projectID]
	if !ok {
		return nil, ErrProjectNotFound
	}
	if p.TenantID != tenantID {
		return nil, ErrProjectTenantMismatch
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionProjectUpdate); err != nil {
			return nil, err
		}
	}

	before := map[string]interface{}{"name": p.Name, "budget_cap": p.BudgetCapBaseUnits, "status": string(p.Status)}

	if name != "" {
		cleanName := strings.TrimSpace(name)
		for _, other := range s.projects {
			if other.ProjectID != projectID && other.TenantID == tenantID && other.Name == cleanName {
				return nil, ErrProjectAlreadyExists
			}
		}
		p.Name = cleanName
	}
	if budgetCap != nil {
		p.BudgetCapBaseUnits = *budgetCap
	}
	if status != nil {
		p.Status = *status
	}
	p.UpdatedAt = time.Now().Unix()

	s.recordAudit(tenantID, actorMemberID, "project.updated", "Project", projectID, before, map[string]interface{}{"name": p.Name, "status": string(p.Status)}, "127.0.0.1")
	return p, nil
}

func (s *MemoryService) DeleteProject(ctx context.Context, tenantID, projectID string, actorMemberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.projects[projectID]
	if !ok {
		return ErrProjectNotFound
	}
	if p.TenantID != tenantID {
		return ErrProjectTenantMismatch
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionProjectDelete); err != nil {
			return err
		}
	}

	delete(s.projects, projectID)
	s.recordAudit(tenantID, actorMemberID, "project.deleted", "Project", projectID, map[string]interface{}{"name": p.Name}, nil, "127.0.0.1")
	return nil
}

// ----------------------------------------------------------------------------
// Members & RBAC
// ----------------------------------------------------------------------------

func (s *MemoryService) AddMember(ctx context.Context, tenantID, email string, role MemberRole, actorMemberID string) (*Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanEmail := strings.TrimSpace(strings.ToLower(email))
	if cleanEmail == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionMemberCreate); err != nil {
			return nil, err
		}
	}

	for _, m := range s.members {
		if m.TenantID == tenantID && m.Email == cleanEmail {
			return nil, ErrMemberAlreadyExists
		}
	}

	now := time.Now().Unix()
	m := &Member{
		MemberID:  GenerateMemberID(),
		TenantID:  tenantID,
		Email:     cleanEmail,
		Role:      role,
		Status:    MemberStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.members[m.MemberID] = m

	s.recordAudit(tenantID, actorMemberID, "member.created", "Member", m.MemberID, nil, map[string]interface{}{"email": cleanEmail, "role": string(role)}, "127.0.0.1")
	return m, nil
}

func (s *MemoryService) GetMember(ctx context.Context, tenantID, memberID string) (*Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.members[memberID]
	if !ok || m.TenantID != tenantID {
		return nil, ErrMemberNotFound
	}
	return m, nil
}

func (s *MemoryService) ListMembers(ctx context.Context, tenantID string) ([]*Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Member, 0)
	for _, m := range s.members {
		if m.TenantID == tenantID {
			res = append(res, m)
		}
	}
	return res, nil
}

func (s *MemoryService) UpdateMember(ctx context.Context, tenantID, memberID string, role *MemberRole, status *MemberStatus, actorMemberID string) (*Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.members[memberID]
	if !ok || m.TenantID != tenantID {
		return nil, ErrMemberNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionMemberUpdate); err != nil {
			return nil, err
		}
	}

	// Last owner protection
	if m.Role == RoleOwner && (role != nil && *role != RoleOwner || status != nil && *status == MemberStatusDisabled) {
		activeOwners := 0
		for _, other := range s.members {
			if other.TenantID == tenantID && other.Role == RoleOwner && other.Status == MemberStatusActive {
				activeOwners++
			}
		}
		if activeOwners <= 1 {
			return nil, ErrCannotDemoteLastOwner
		}
	}

	before := map[string]interface{}{"role": string(m.Role), "status": string(m.Status)}
	if role != nil {
		m.Role = *role
	}
	if status != nil {
		m.Status = *status
	}
	m.UpdatedAt = time.Now().Unix()

	s.recordAudit(tenantID, actorMemberID, "member.updated", "Member", memberID, before, map[string]interface{}{"role": string(m.Role), "status": string(m.Status)}, "127.0.0.1")
	return m, nil
}

func (s *MemoryService) RemoveMember(ctx context.Context, tenantID, memberID string, actorMemberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.members[memberID]
	if !ok || m.TenantID != tenantID {
		return ErrMemberNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionMemberDelete); err != nil {
			return err
		}
	}

	if m.Role == RoleOwner {
		activeOwners := 0
		for _, other := range s.members {
			if other.TenantID == tenantID && other.Role == RoleOwner && other.Status == MemberStatusActive {
				activeOwners++
			}
		}
		if activeOwners <= 1 {
			return ErrCannotRemoveLastOwner
		}
	}

	delete(s.members, memberID)
	s.recordAudit(tenantID, actorMemberID, "member.deleted", "Member", memberID, map[string]interface{}{"email": m.Email}, nil, "127.0.0.1")
	return nil
}

// ----------------------------------------------------------------------------
// Agents & Kill Switch
// ----------------------------------------------------------------------------

func (s *MemoryService) CreateAgent(ctx context.Context, tenantID, projectID, name string, policyCfg map[string]interface{}, actorMemberID string) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proj, ok := s.projects[projectID]
	if !ok || proj.TenantID != tenantID {
		return nil, ErrProjectNotFound
	}
	if proj.Status == ProjectStatusArchived {
		return nil, ErrProjectArchived
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionAgentCreate); err != nil {
			return nil, err
		}
	}

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	for _, a := range s.agents {
		if a.ProjectID == projectID && a.Name == cleanName {
			return nil, ErrAgentAlreadyExists
		}
	}

	now := time.Now().Unix()
	aid := GenerateAgentID()
	if policyCfg == nil {
		policyCfg = make(map[string]interface{})
	}

	ag := &Agent{
		AgentID:      aid,
		TenantID:     tenantID,
		ProjectID:    projectID,
		Name:         cleanName,
		Status:       AgentStatusActive,
		PolicyConfig: policyCfg,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.agents[aid] = ag

	// Register in data plane registry
	if s.agentRegistry != nil {
		s.agentRegistry.RegisterAgent(&agent.AgentIdentity{
			AgentID:   aid,
			TenantID:  tenantID,
			ProjectID: projectID,
			Name:      cleanName,
			Status:    agent.AgentStatusActive,
		}, nil)
	}

	s.recordAudit(tenantID, actorMemberID, "agent.created", "Agent", aid, nil, map[string]interface{}{"name": cleanName, "project_id": projectID}, "127.0.0.1")
	return ag, nil
}

func (s *MemoryService) GetAgent(ctx context.Context, tenantID, agentID string) (*Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ag, ok := s.agents[agentID]
	if !ok {
		return nil, ErrAgentNotFound
	}
	if tenantID != "" && ag.TenantID != tenantID {
		return nil, ErrAgentTenantMismatch
	}
	return ag, nil
}

func (s *MemoryService) ListAgents(ctx context.Context, tenantID, projectID string, status *AgentStatus) ([]*Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Agent, 0)
	for _, a := range s.agents {
		if tenantID != "" && a.TenantID != tenantID {
			continue
		}
		if projectID != "" && a.ProjectID != projectID {
			continue
		}
		if status != nil && a.Status != *status {
			continue
		}
		res = append(res, a)
	}
	return res, nil
}

func (s *MemoryService) UpdateAgent(ctx context.Context, tenantID, agentID string, name string, policyCfg map[string]interface{}, actorMemberID string) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ag, ok := s.agents[agentID]
	if !ok || ag.TenantID != tenantID {
		return nil, ErrAgentNotFound
	}
	if ag.Status == AgentStatusRevoked {
		return nil, ErrAgentRevoked
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionAgentUpdate); err != nil {
			return nil, err
		}
	}

	before := map[string]interface{}{"name": ag.Name, "policy_config": ag.PolicyConfig}
	if name != "" {
		cleanName := strings.TrimSpace(name)
		for _, other := range s.agents {
			if other.AgentID != agentID && other.ProjectID == ag.ProjectID && other.Name == cleanName {
				return nil, ErrAgentAlreadyExists
			}
		}
		ag.Name = cleanName
	}
	if policyCfg != nil {
		ag.PolicyConfig = policyCfg
	}
	ag.UpdatedAt = time.Now().Unix()

	s.recordAudit(tenantID, actorMemberID, "agent.updated", "Agent", agentID, before, map[string]interface{}{"name": ag.Name, "policy_config": ag.PolicyConfig}, "127.0.0.1")
	return ag, nil
}

func (s *MemoryService) UpdateAgentStatus(ctx context.Context, tenantID, agentID string, status AgentStatus, actorMemberID string) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ag, ok := s.agents[agentID]
	if !ok || ag.TenantID != tenantID {
		return nil, ErrAgentNotFound
	}

	if ag.Status == AgentStatusRevoked && status != AgentStatusRevoked {
		return nil, ErrInvalidStatusTransition
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if status == AgentStatusRevoked || status == AgentStatusSuspended {
			if err := EnforcePermission(actor, PermissionAgentKill); err != nil {
				return nil, err
			}
		} else {
			if err := EnforcePermission(actor, PermissionAgentUpdate); err != nil {
				return nil, err
			}
		}
	}

	before := map[string]interface{}{"status": string(ag.Status)}
	ag.Status = status
	ag.UpdatedAt = time.Now().Unix()

	// Invalidate keys if revoked
	if status == AgentStatusRevoked {
		for _, k := range s.apiKeys {
			if k.AgentID == agentID && k.Status == APIKeyStatusActive {
				k.Status = APIKeyStatusRevoked
				k.UpdatedAt = ag.UpdatedAt
			}
		}
	}

	// Update data plane registry
	if s.agentRegistry != nil {
		s.agentRegistry.SetAgentStatus(agentID, agent.AgentStatus(status))
	}

	s.recordAudit(tenantID, actorMemberID, "agent.status.changed", "Agent", agentID, before, map[string]interface{}{"status": string(status)}, "127.0.0.1")
	return ag, nil
}

func (s *MemoryService) KillAgent(ctx context.Context, tenantID, agentID, reason string, actorMemberID string, revoke bool, clientIP string) (*Agent, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return nil, nil, ErrMissingReason
	}

	ag, ok := s.agents[agentID]
	if !ok || ag.TenantID != tenantID {
		return nil, nil, ErrAgentNotFound
	}
	if ag.Status == AgentStatusRevoked {
		return nil, nil, ErrInvalidStatusTransition
	}

	if actorMemberID == "" {
		return nil, nil, ErrPermissionDenied
	}
	actor, ok := s.members[actorMemberID]
	if !ok || actor.TenantID != tenantID {
		return nil, nil, ErrPermissionDenied
	}
	if err := EnforcePermission(actor, PermissionAgentKill); err != nil {
		return nil, nil, err
	}

	targetStatus := AgentStatusSuspended
	if revoke {
		targetStatus = AgentStatusRevoked
	}

	now := time.Now().Unix()
	before := map[string]interface{}{"status": string(ag.Status)}
	ag.Status = targetStatus
	ag.UpdatedAt = now

	// Cascade revocation of active keys
	revokedKeys := make([]string, 0)
	for _, k := range s.apiKeys {
		if k.AgentID == agentID && k.Status == APIKeyStatusActive {
			k.Status = APIKeyStatusRevoked
			k.UpdatedAt = now
			revokedKeys = append(revokedKeys, k.KeyID)
		}
	}

	// Direct fast-path in Go Registry (nanosecond level!)
	if s.agentRegistry != nil {
		s.agentRegistry.SetAgentStatus(agentID, agent.AgentStatus(targetStatus))
	}

	after := map[string]interface{}{
		"status":       string(targetStatus),
		"reason":       cleanReason,
		"revoked_keys": revokedKeys,
		"revoke":       revoke,
	}
	s.recordAudit(tenantID, actorMemberID, "agent.control", "Agent", agentID, before, after, clientIP)

	return ag, revokedKeys, nil
}

func (s *MemoryService) DeleteAgent(ctx context.Context, tenantID, agentID, actorMemberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ag, ok := s.agents[agentID]
	if !ok || ag.TenantID != tenantID {
		return ErrAgentNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionAgentKill); err != nil {
			return err
		}
	}

	// Invalidate keys
	for _, k := range s.apiKeys {
		if k.AgentID == agentID {
			k.Status = APIKeyStatusRevoked
		}
	}
	delete(s.agents, agentID)

	s.recordAudit(tenantID, actorMemberID, "agent.deleted", "Agent", agentID, map[string]interface{}{"name": ag.Name}, nil, "127.0.0.1")
	return nil
}

// ----------------------------------------------------------------------------
// API Keys
// ----------------------------------------------------------------------------

func (s *MemoryService) CreateAPIKey(ctx context.Context, tenantID, projectID, agentID, name string, scopes []string, expiresAt *int64, actorMemberID string) (*CreateAPIKeyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ag, ok := s.agents[agentID]
	if !ok || ag.TenantID != tenantID || ag.ProjectID != projectID {
		return nil, ErrAgentNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionAPIKeyCreate); err != nil {
			return nil, err
		}
	}

	rawKey, keyHash, keyPrefix := GenerateRawAPIKey()
	now := time.Now().Unix()
	if scopes == nil {
		scopes = []string{"gateway:call"}
	}

	k := &APIKey{
		KeyID:      GenerateAPIKeyID(),
		TenantID:   tenantID,
		ProjectID:  projectID,
		AgentID:    agentID,
		Name:       strings.TrimSpace(name),
		KeyHash:    keyHash,
		KeyPrefix:  keyPrefix,
		Scopes:     scopes,
		Status:     APIKeyStatusActive,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.apiKeys[k.KeyID] = k

	// Bind in registry and memory wallet
	if s.agentRegistry != nil {
		s.agentRegistry.BindAPIKeyHash(keyHash, agentID)
	}
	if s.walletSvc != nil {
		memWlt, ok := s.walletSvc.(*wallet.MemoryService)
		if ok {
			memWlt.RegisterAPIKey(rawKey, tenantID, projectID)
		}
	}

	s.recordAudit(tenantID, actorMemberID, "apikey.created", "ApiKey", k.KeyID, nil, map[string]interface{}{"name": k.Name, "agent_id": agentID}, "127.0.0.1")
	return &CreateAPIKeyResult{APIKey: k, RawKey: rawKey}, nil
}

func (s *MemoryService) GetAPIKey(ctx context.Context, tenantID, keyID string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k, ok := s.apiKeys[keyID]
	if !ok || (tenantID != "" && k.TenantID != tenantID) {
		return nil, ErrAPIKeyNotFound
	}
	return k, nil
}

func (s *MemoryService) ListAPIKeys(ctx context.Context, tenantID, agentID string) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*APIKey, 0)
	for _, k := range s.apiKeys {
		if tenantID != "" && k.TenantID != tenantID {
			continue
		}
		if agentID != "" && k.AgentID != agentID {
			continue
		}
		res = append(res, k)
	}
	return res, nil
}

func (s *MemoryService) RevokeAPIKey(ctx context.Context, tenantID, keyID, actorMemberID string) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k, ok := s.apiKeys[keyID]
	if !ok || k.TenantID != tenantID {
		return nil, ErrAPIKeyNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionAPIKeyRevoke); err != nil {
			return nil, err
		}
	}

	k.Status = APIKeyStatusRevoked
	k.UpdatedAt = time.Now().Unix()

	s.recordAudit(tenantID, actorMemberID, "apikey.revoked", "ApiKey", keyID, nil, map[string]interface{}{"status": "revoked"}, "127.0.0.1")
	return k, nil
}

func (s *MemoryService) RotateAPIKey(ctx context.Context, tenantID, keyID, actorMemberID string) (*CreateAPIKeyResult, error) {
	oldKey, err := s.RevokeAPIKey(ctx, tenantID, keyID, actorMemberID)
	if err != nil {
		return nil, err
	}
	return s.CreateAPIKey(ctx, tenantID, oldKey.ProjectID, oldKey.AgentID, oldKey.Name, oldKey.Scopes, oldKey.ExpiresAt, actorMemberID)
}

// ----------------------------------------------------------------------------
// Routes & Policies
// ----------------------------------------------------------------------------

func (s *MemoryService) CreateRoute(ctx context.Context, r *Route, actorMemberID string) (*Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != r.TenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionRouteWrite); err != nil {
			return nil, err
		}
	}

	cleanPath := strings.TrimSpace(r.PathPattern)
	if cleanPath == "" {
		return nil, fmt.Errorf("path_pattern cannot be empty")
	}
	if !strings.HasPrefix(cleanPath, "/") {
		return nil, fmt.Errorf("path_pattern must begin with /")
	}
	cleanUpstream := strings.TrimSpace(r.UpstreamURL)
	if cleanUpstream == "" {
		return nil, fmt.Errorf("upstream_url cannot be empty")
	}

	if r.ProjectID != "" {
		p, ok := s.projects[r.ProjectID]
		if !ok || p.TenantID != r.TenantID {
			return nil, ErrProjectNotFound
		}
	}

	for _, existing := range s.routes {
		if existing.TenantID == r.TenantID && existing.PathPattern == cleanPath {
			return nil, ErrRouteAlreadyExists
		}
	}

	if r.RouteID == "" {
		r.RouteID = GenerateRouteID()
	}
	r.PathPattern = cleanPath
	r.UpstreamURL = cleanUpstream
	if r.TimeoutMs <= 0 {
		r.TimeoutMs = 30000
	}
	if r.AllowedPaymentMethods == "" {
		r.AllowedPaymentMethods = "both"
	}
	if r.RateLimitRPM <= 0 {
		r.RateLimitRPM = 600
	}
	if r.Asset == "" {
		r.Asset = "USDC"
	}
	if r.Status == "" {
		r.Status = RouteStatusActive
	}
	r.PayeeAddress = strings.ToLower(strings.TrimSpace(r.PayeeAddress))

	now := time.Now().Unix()
	r.CreatedAt = now
	r.UpdatedAt = now

	if r.UpstreamAuth != nil {
		if r.UpstreamAuth.Type == "" {
			r.UpstreamAuth.Type = "none"
		}
		if auth, ok := upstreamauth.Get(r.UpstreamAuth.Type); ok {
			if err := auth.Validate(r.UpstreamAuth.Params, r.UpstreamAuth.Secrets); err != nil {
				return nil, fmt.Errorf("upstream_auth validation error: %w", err)
			}
		}
		if len(r.UpstreamAuth.Secrets) > 0 {
			enc, err := upstreamauth.EncryptSecrets(r.UpstreamAuth.Secrets, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt upstream secrets: %w", err)
			}
			r.UpstreamAuth.SecretEnc = enc
			r.UpstreamAuth.HasSecret = true
			r.UpstreamAuth.KeyVersion = 1
			r.UpstreamAuth.Secrets = nil
		}
		r.UpstreamAuth.UpdatedAt = now
	}

	s.routes[r.RouteID] = r

	s.recordAudit(r.TenantID, actorMemberID, "route.created", "Route", r.RouteID, nil, map[string]interface{}{
		"path":         r.PathPattern,
		"upstream_url": r.UpstreamURL,
		"price":        r.PriceBaseUnits,
		"status":       string(r.Status),
	}, "127.0.0.1")
	_ = s.syncRCULocked()
	return r, nil
}

func (s *MemoryService) GetRoute(ctx context.Context, tenantID, routeID string) (*Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.routes[routeID]
	if !ok || (tenantID != "" && r.TenantID != tenantID) {
		return nil, ErrRouteNotFound
	}
	return r, nil
}

func (s *MemoryService) ListRoutes(ctx context.Context, tenantID string) ([]*Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Route, 0)
	for _, r := range s.routes {
		if tenantID == "" || r.TenantID == tenantID {
			res = append(res, r)
		}
	}
	return res, nil
}

func (s *MemoryService) UpdateRoute(ctx context.Context, input *UpdateRouteInput, actorMemberID string) (*Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.routes[input.RouteID]
	if !ok || existing.TenantID != input.TenantID {
		return nil, ErrRouteNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != input.TenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionRouteWrite); err != nil {
			return nil, err
		}
	}

	before := map[string]interface{}{
		"project_id":              existing.ProjectID,
		"path_pattern":            existing.PathPattern,
		"upstream_url":            existing.UpstreamURL,
		"price_base_units":        existing.PriceBaseUnits,
		"asset":                   existing.Asset,
		"payee_address":           existing.PayeeAddress,
		"allowed_payment_methods": existing.AllowedPaymentMethods,
		"rate_limit_rpm":          existing.RateLimitRPM,
		"timeout_ms":              existing.TimeoutMs,
		"status":                  string(existing.Status),
	}

	if input.PathPattern != nil {
		cleanPath := strings.TrimSpace(*input.PathPattern)
		if cleanPath == "" {
			return nil, fmt.Errorf("path_pattern cannot be empty")
		}
		if !strings.HasPrefix(cleanPath, "/") {
			return nil, fmt.Errorf("path_pattern must begin with /")
		}
		for _, other := range s.routes {
			if other.RouteID != input.RouteID && other.TenantID == input.TenantID && other.PathPattern == cleanPath {
				return nil, ErrRouteAlreadyExists
			}
		}
		existing.PathPattern = cleanPath
	}

	if input.UpstreamURL != nil {
		cleanUpstream := strings.TrimSpace(*input.UpstreamURL)
		if cleanUpstream == "" {
			return nil, fmt.Errorf("upstream_url cannot be empty")
		}
		existing.UpstreamURL = cleanUpstream
	}

	if input.ProjectID != nil {
		pID := strings.TrimSpace(*input.ProjectID)
		if pID != "" {
			p, ok := s.projects[pID]
			if !ok || p.TenantID != input.TenantID {
				return nil, ErrProjectNotFound
			}
		}
		existing.ProjectID = pID
	}

	if input.PriceBaseUnits != nil {
		if *input.PriceBaseUnits < 0 {
			return nil, fmt.Errorf("price_base_units cannot be negative")
		}
		existing.PriceBaseUnits = *input.PriceBaseUnits
	}

	if input.Asset != nil {
		existing.Asset = strings.TrimSpace(*input.Asset)
	}

	if input.PayeeAddress != nil {
		existing.PayeeAddress = strings.ToLower(strings.TrimSpace(*input.PayeeAddress))
	}

	if input.AllowedPaymentMethods != nil {
		method := strings.TrimSpace(*input.AllowedPaymentMethods)
		if method == "wallet" || method == "onchain" || method == "both" {
			existing.AllowedPaymentMethods = method
		}
	}

	if input.RateLimitRPM != nil && *input.RateLimitRPM > 0 {
		existing.RateLimitRPM = *input.RateLimitRPM
	}

	if input.TimeoutMs != nil && *input.TimeoutMs > 0 {
		existing.TimeoutMs = *input.TimeoutMs
	}

	if input.Status != nil {
		existing.Status = *input.Status
	}

	if input.UpstreamAuth != nil {
		authCfg := input.UpstreamAuth
		if authCfg.Type == "" {
			authCfg.Type = "none"
		}
		if auth, ok := upstreamauth.Get(authCfg.Type); ok {
			if err := auth.Validate(authCfg.Params, authCfg.Secrets); err != nil {
				return nil, fmt.Errorf("upstream_auth validation error: %w", err)
			}
		}
		if len(authCfg.Secrets) > 0 {
			enc, err := upstreamauth.EncryptSecrets(authCfg.Secrets, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt upstream secrets: %w", err)
			}
			authCfg.SecretEnc = enc
			authCfg.HasSecret = true
			authCfg.KeyVersion = 1
			if existing.UpstreamAuth != nil {
				authCfg.KeyVersion = existing.UpstreamAuth.KeyVersion + 1
			}
			authCfg.Secrets = nil
		} else if existing.UpstreamAuth != nil && existing.UpstreamAuth.Type == authCfg.Type {
			authCfg.SecretEnc = existing.UpstreamAuth.SecretEnc
			authCfg.HasSecret = existing.UpstreamAuth.HasSecret
			authCfg.KeyVersion = existing.UpstreamAuth.KeyVersion
		}
		authCfg.UpdatedAt = time.Now().Unix()
		existing.UpstreamAuth = authCfg
	}

	existing.UpdatedAt = time.Now().Unix()

	s.recordAudit(input.TenantID, actorMemberID, "route.updated", "Route", input.RouteID, before, map[string]interface{}{
		"path_pattern":            existing.PathPattern,
		"upstream_url":            existing.UpstreamURL,
		"price_base_units":        existing.PriceBaseUnits,
		"payee_address":           existing.PayeeAddress,
		"allowed_payment_methods": existing.AllowedPaymentMethods,
		"timeout_ms":              existing.TimeoutMs,
		"status":                  string(existing.Status),
	}, "127.0.0.1")

	_ = s.syncRCULocked()
	return existing, nil
}

func (s *MemoryService) DeleteRoute(ctx context.Context, tenantID, routeID, actorMemberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.routes[routeID]
	if !ok || existing.TenantID != tenantID {
		return ErrRouteNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionRouteWrite); err != nil {
			return err
		}
	}

	delete(s.routes, routeID)

	s.recordAudit(tenantID, actorMemberID, "route.deleted", "Route", routeID, map[string]interface{}{"path": existing.PathPattern}, nil, "127.0.0.1")
	_ = s.syncRCULocked()
	return nil
}

func (s *MemoryService) CreatePolicy(ctx context.Context, pol *Policy, actorMemberID string) (*Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pol.PolicyID == "" {
		pol.PolicyID = GeneratePolicyID()
	}
	now := time.Now().Unix()
	pol.CreatedAt = now
	pol.UpdatedAt = now
	pol.IsActive = true
	s.policies[pol.PolicyID] = pol

	s.recordAudit(pol.TenantID, actorMemberID, "policy.created", "Policy", pol.PolicyID, nil, nil, "127.0.0.1")
	_ = s.syncRCULocked()
	return pol, nil
}

func (s *MemoryService) GetPolicy(ctx context.Context, tenantID, policyID string) (*Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pol, ok := s.policies[policyID]
	if !ok || (tenantID != "" && pol.TenantID != tenantID) {
		return nil, ErrPolicyNotFound
	}
	return pol, nil
}

func (s *MemoryService) ListPolicies(ctx context.Context, tenantID, projectID, agentID string) ([]*Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Policy, 0)
	for _, p := range s.policies {
		if tenantID != "" && p.TenantID != tenantID {
			continue
		}
		if projectID != "" && p.ProjectID != projectID {
			continue
		}
		if agentID != "" && p.AgentID != agentID {
			continue
		}
		res = append(res, p)
	}
	return res, nil
}

func (s *MemoryService) UpdatePolicy(ctx context.Context, pol *Policy, actorMemberID string) (*Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.policies[pol.PolicyID]
	if !ok || existing.TenantID != pol.TenantID {
		return nil, ErrPolicyNotFound
	}
	pol.UpdatedAt = time.Now().Unix()
	s.policies[pol.PolicyID] = pol

	s.recordAudit(pol.TenantID, actorMemberID, "policy.updated", "Policy", pol.PolicyID, nil, nil, "127.0.0.1")
	_ = s.syncRCULocked()
	return pol, nil
}

func (s *MemoryService) DeletePolicy(ctx context.Context, tenantID, policyID, actorMemberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.policies[policyID]
	if !ok || existing.TenantID != tenantID {
		return ErrPolicyNotFound
	}
	delete(s.policies, policyID)

	s.recordAudit(tenantID, actorMemberID, "policy.deleted", "Policy", policyID, nil, nil, "127.0.0.1")
	_ = s.syncRCULocked()
	return nil
}

// ----------------------------------------------------------------------------
// RCU Data Plane Synchronization
// ----------------------------------------------------------------------------

func (s *MemoryService) SetRCUManager(mgr *rcu.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rcuMgr = mgr
}

func (s *MemoryService) SyncRCU(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncRCULocked()
}

func (s *MemoryService) syncRCULocked() error {
	if s.rcuMgr == nil {
		return nil
	}

	routes := make([]rcu.RouteConfig, 0, len(s.routes))
	for _, r := range s.routes {
		if r.Status == RouteStatusActive {
			routes = append(routes, rcu.RouteConfig{
				RouteID:               r.RouteID,
				TenantID:              r.TenantID,
				ProjectID:             r.ProjectID,
				PathPattern:           r.PathPattern,
				UpstreamURL:           r.UpstreamURL,
				PriceBaseUnits:        r.PriceBaseUnits,
				Asset:                 r.Asset,
				PayeeAddress:          r.PayeeAddress,
				AllowedPaymentMethods: r.AllowedPaymentMethods,
				TimeoutMS:             r.TimeoutMs,
				RateLimitRPM:          r.RateLimitRPM,
				Status:                string(r.Status),
				UpstreamAuth:          r.UpstreamAuth,
			})
		}
	}

	policies := make([]rcu.PolicyConfig, 0, len(s.policies))
	for _, p := range s.policies {
		if p.IsActive {
			var agID, rteID *string
			if p.AgentID != "" {
				ag := p.AgentID
				agID = &ag
			}
			if p.RouteID != "" {
				rt := p.RouteID
				rteID = &rt
			}
			policies = append(policies, rcu.PolicyConfig{
				PolicyID:             p.PolicyID,
				TenantID:             p.TenantID,
				ProjectID:            p.ProjectID,
				AgentID:              agID,
				RouteID:              rteID,
				MaxAmountPerRequest:  p.MaxAmountPerRequest,
				Rolling24hBudgetCap:  p.Rolling24hBudgetCap,
				CircuitBreakerMaxRPM: p.CircuitBreakerMaxRPM,
				WhitelistedNetworks:  p.WhitelistedNetworks,
				WhitelistedRoutes:    p.WhitelistedRoutes,
				IsActive:             p.IsActive,
			})
		}
	}

	curr := s.rcuMgr.Current()
	if curr != nil {
		if len(routes) == 0 && len(policies) == 0 {
			return nil
		}
		existingRoutes := make(map[string]bool)
		for _, r := range routes {
			existingRoutes[r.PathPattern] = true
		}
		for _, br := range curr.Routes {
			if !existingRoutes[br.PathPattern] && br.Status == "active" {
				routes = append(routes, br)
			}
		}

		existingPolicies := make(map[string]bool)
		for _, p := range policies {
			existingPolicies[p.PolicyID] = true
		}
		for _, bp := range curr.Policies {
			if !existingPolicies[bp.PolicyID] && bp.IsActive {
				policies = append(policies, bp)
			}
		}
	} else if len(routes) == 0 && len(policies) == 0 {
		return nil
	}

	newSnap := &rcu.ConfigSnapshot{
		SchemaVersion: "2.0.0",
		TenantID:      "global",
		VersionID:     fmt.Sprintf("ver_sync_%d", time.Now().UnixNano()),
		VersionNumber: time.Now().Unix(),
		PublishedAt:   time.Now().Unix(),
		PublishedBy:   "admin_memory_sync",
		Routes:        routes,
		Policies:      policies,
	}

	return s.rcuMgr.Update(newSnap)
}

// ----------------------------------------------------------------------------
// Wallets
// ----------------------------------------------------------------------------

func (s *MemoryService) AdminGrantWallet(ctx context.Context, tenantID string, amount int64, reason string, idempotencyKey string, actorMemberID string, clientIP string) error {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return ErrMissingReason
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}

	if actorMemberID != "" {
		s.mu.RLock()
		actor, ok := s.members[actorMemberID]
		s.mu.RUnlock()
		if !ok || actor.TenantID != tenantID {
			return ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionWalletAdjust); err != nil {
			return err
		}
	}

	w, err := s.walletSvc.GetWalletByTenant(ctx, tenantID)
	if err != nil {
		return ErrWalletNotFound
	}

	order := &wallet.TopUpOrder{
		TopUpID:     fmt.Sprintf("ord_%s_%d", tenantID, time.Now().UnixNano()),
		WalletID:    w.WalletID,
		TenantID:    tenantID,
		Channel:     wallet.ChannelAdminGrant,
		Amount:      amount,
		Asset:       w.Asset,
		ExternalRef: idempotencyKey,
		Reason:      cleanReason,
		Status:      wallet.TopUpStatusConfirmed,
		CreatedAt:   time.Now(),
		ConfirmedAt: time.Now(),
	}

	if err := s.walletSvc.CreditTopUp(ctx, order); err != nil {
		return err
	}

	s.recordAudit(tenantID, actorMemberID, "wallet.admin_grant", "Wallet", w.WalletID, nil, map[string]interface{}{"amount": amount, "reason": cleanReason}, clientIP)
	return nil
}

func (s *MemoryService) AdjustWallet(ctx context.Context, walletID string, amount int64, reason string, idempotencyKey string, actorMemberID string, clientIP string) error {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return ErrMissingReason
	}

	w, err := s.walletSvc.GetWallet(ctx, walletID)
	if err != nil {
		return ErrWalletNotFound
	}

	if actorMemberID != "" {
		s.mu.RLock()
		actor, ok := s.members[actorMemberID]
		s.mu.RUnlock()
		if !ok || actor.TenantID != w.TenantID {
			return ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionWalletAdjust); err != nil {
			return err
		}
	}

	if err := s.walletSvc.AdjustBalance(ctx, walletID, amount, cleanReason, idempotencyKey, actorMemberID); err != nil {
		return err
	}

	s.recordAudit(w.TenantID, actorMemberID, "wallet.adjust", "Wallet", walletID, nil, map[string]interface{}{"amount": amount, "reason": cleanReason}, clientIP)
	return nil
}

func (s *MemoryService) FreezeWallet(ctx context.Context, walletID string, reason string, actorMemberID string, clientIP string) error {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return ErrMissingReason
	}

	w, err := s.walletSvc.GetWallet(ctx, walletID)
	if err != nil {
		return ErrWalletNotFound
	}

	if actorMemberID != "" {
		s.mu.RLock()
		actor, ok := s.members[actorMemberID]
		s.mu.RUnlock()
		if !ok || actor.TenantID != w.TenantID {
			return ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionWalletAdjust); err != nil {
			return err
		}
	}

	if err := s.walletSvc.SetWalletStatus(ctx, walletID, wallet.StatusFrozen); err != nil {
		return err
	}

	s.recordAudit(w.TenantID, actorMemberID, "wallet.frozen", "Wallet", walletID, nil, map[string]interface{}{"reason": cleanReason}, clientIP)
	return nil
}

func (s *MemoryService) UnfreezeWallet(ctx context.Context, walletID string, reason string, actorMemberID string, clientIP string) error {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return ErrMissingReason
	}

	w, err := s.walletSvc.GetWallet(ctx, walletID)
	if err != nil {
		return ErrWalletNotFound
	}

	if actorMemberID != "" {
		s.mu.RLock()
		actor, ok := s.members[actorMemberID]
		s.mu.RUnlock()
		if !ok || actor.TenantID != w.TenantID {
			return ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionWalletAdjust); err != nil {
			return err
		}
	}

	if w.Status == wallet.StatusFrozenArrears && w.AvailableCache < 0 {
		return fmt.Errorf("cannot unfreeze wallet in arrears")
	}

	if err := s.walletSvc.SetWalletStatus(ctx, walletID, wallet.StatusActive); err != nil {
		return err
	}

	s.recordAudit(w.TenantID, actorMemberID, "wallet.unfrozen", "Wallet", walletID, nil, map[string]interface{}{"reason": cleanReason}, clientIP)
	return nil
}

func (s *MemoryService) RecalculateWallet(ctx context.Context, walletID string, actorMemberID string) (map[string]interface{}, error) {
	rep, err := s.walletSvc.ReconcileBalance(ctx, walletID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"wallet_id":   walletID,
		"cached":      rep.AvailableCache,
		"calculated":  rep.SumLedgerEntries,
		"consistent":  rep.IsBalanced,
		"is_balanced": rep.IsBalanced,
	}, nil
}

func (s *MemoryService) GetWallet(ctx context.Context, tenantID string) (*wallet.Wallet, error) {
	return s.walletSvc.GetWalletByTenant(ctx, tenantID)
}

func (s *MemoryService) GetWalletByID(ctx context.Context, walletID string) (*wallet.Wallet, error) {
	return s.walletSvc.GetWallet(ctx, walletID)
}

func (s *MemoryService) GetLedgerEntries(ctx context.Context, walletID string) ([]*wallet.LedgerEntry, error) {
	return s.walletSvc.GetLedgerEntries(ctx, walletID)
}

func (s *MemoryService) GetTopUpOrders(ctx context.Context, walletID string) ([]*wallet.TopUpOrder, error) {
	return s.walletSvc.GetTopUpOrders(ctx, walletID)
}

// ----------------------------------------------------------------------------
// Audit Events
// ----------------------------------------------------------------------------

func (s *MemoryService) ListAuditEvents(ctx context.Context, tenantID, targetType, targetID string, limit int) ([]*AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*AuditEvent, 0)
	for i := len(s.auditEvents) - 1; i >= 0; i-- {
		ev := s.auditEvents[i]
		if tenantID != "" && ev.TenantID != tenantID {
			continue
		}
		if targetType != "" && ev.TargetType != targetType {
			continue
		}
		if targetID != "" && ev.TargetID != targetID {
			continue
		}
		res = append(res, ev)
		if limit > 0 && len(res) >= limit {
			break
		}
	}
	return res, nil
}

// ----------------------------------------------------------------------------
// Config Draft & Validation (T4.3)
// ----------------------------------------------------------------------------

var ethAddressValidationRegex = regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)
var caip2ValidationRegex = regexp.MustCompile(`^[-a-z0-9]{3,8}:[-_a-zA-Z0-9]{1,32}$`)

func (s *MemoryService) ValidateConfig(ctx context.Context, tenantID string, actorMemberID string) (*ValidationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}

	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionRouteRead); err != nil {
			return nil, err
		}
	}

	errorsList := make([]ValidationError, 0)

	// Collect tenant routes and policies
	routes := make([]*Route, 0)
	for _, r := range s.routes {
		if r.TenantID == tenantID {
			routes = append(routes, r)
		}
	}

	policies := make([]*Policy, 0)
	for _, p := range s.policies {
		if p.TenantID == tenantID {
			policies = append(policies, p)
		}
	}

	// 1. Route validation
	seenPatterns := make(map[string]string)
	for _, r := range routes {
		// 1.1 URI scheme
		if !strings.HasPrefix(r.UpstreamURL, "http://") && !strings.HasPrefix(r.UpstreamURL, "https://") {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "upstream_url",
				Code:       "INVALID_UPSTREAM_URL",
				Message:    fmt.Sprintf("上游地址 '%s' 非法，必须以 http:// 或 https:// 开头", r.UpstreamURL),
			})
		}

		// 1.2 Path pattern
		cleanPath := strings.TrimSpace(r.PathPattern)
		if cleanPath == "" || !strings.HasPrefix(cleanPath, "/") {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "path_pattern",
				Code:       "INVALID_PATH_PATTERN",
				Message:    fmt.Sprintf("路由暴露路径 '%s' 非法，必须以 / 开头", r.PathPattern),
			})
		} else {
			if existingID, seen := seenPatterns[cleanPath]; seen {
				errorsList = append(errorsList, ValidationError{
					TargetType: "Route",
					TargetID:   r.RouteID,
					Field:      "path_pattern",
					Code:       "DUPLICATE_PATH_PATTERN",
					Message:    fmt.Sprintf("路由路径 '%s' 与路由 %s 重复冲突", cleanPath, existingID),
				})
			} else {
				seenPatterns[cleanPath] = r.RouteID
			}
		}

		// 1.3 Payee address for onchain/both
		if r.AllowedPaymentMethods != "wallet" {
			if r.PayeeAddress == "" {
				errorsList = append(errorsList, ValidationError{
					TargetType: "Route",
					TargetID:   r.RouteID,
					Field:      "payee_address",
					Code:       "MISSING_PAYEE_ADDRESS",
					Message:    "支持链上结算时必须指定收款地址",
				})
			} else if !ethAddressValidationRegex.MatchString(r.PayeeAddress) {
				errorsList = append(errorsList, ValidationError{
					TargetType: "Route",
					TargetID:   r.RouteID,
					Field:      "payee_address",
					Code:       "INVALID_PAYEE_ADDRESS",
					Message:    fmt.Sprintf("收款地址 '%s' 格式错误，必须为 42 位 0x 开头的有效 EVM 地址", r.PayeeAddress),
				})
			}
		}

		// 1.4 Pricing non-negative
		if r.PriceBaseUnits < 0 {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "price_base_units",
				Code:       "NEGATIVE_PRICE_UNITS",
				Message:    "单次调用单价不能为负数",
			})
		}

		// 1.5 Payment methods option
		if r.AllowedPaymentMethods != "wallet" && r.AllowedPaymentMethods != "onchain" && r.AllowedPaymentMethods != "both" {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "allowed_payment_methods",
				Code:       "INVALID_PAYMENT_METHOD_OPTION",
				Message:    fmt.Sprintf("支付通道选项 '%s' 无效，必须为 wallet、onchain 或 both", r.AllowedPaymentMethods),
			})
		}
	}

	// 2. Policy validation
	for _, p := range policies {
		if p.Rolling24hBudgetCap > 0 && p.MaxAmountPerRequest > p.Rolling24hBudgetCap {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Policy",
				TargetID:   p.PolicyID,
				Field:      "max_amount_per_request",
				Code:       "MAX_AMOUNT_EXCEEDS_ROLLING_BUDGET",
				Message:    fmt.Sprintf("单笔限额 (%d) 超出了 24 小时滚动预算上限 (%d)", p.MaxAmountPerRequest, p.Rolling24hBudgetCap),
			})
		}

		if p.CircuitBreakerMaxRPM < 0 {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Policy",
				TargetID:   p.PolicyID,
				Field:      "circuit_breaker_max_rpm",
				Code:       "INVALID_CIRCUIT_BREAKER_RPM",
				Message:    "熔断阈值 RPM 不能小于 0",
			})
		}

		for _, net := range p.WhitelistedNetworks {
			if !caip2ValidationRegex.MatchString(net) {
				errorsList = append(errorsList, ValidationError{
					TargetType: "Policy",
					TargetID:   p.PolicyID,
					Field:      "whitelisted_networks",
					Code:       "INVALID_NETWORK_IDENTIFIER",
					Message:    fmt.Sprintf("网络标识符 '%s' 不符合 CAIP-2 规范 (如 eip155:1 或 eip155:42161)", net),
				})
			}
		}
	}

	now := time.Now().Unix()
	res := &ValidationResult{
		Valid:  len(errorsList) == 0,
		Errors: errorsList,
		Summary: ValidationSummary{
			TotalRoutes:   len(routes),
			TotalPolicies: len(policies),
			ErrorCount:    len(errorsList),
			WarningCount:  0,
		},
		ValidatedAt: now,
	}

	if s.lastValidation == nil {
		s.lastValidation = make(map[string]*ValidationResult)
	}
	s.lastValidation[tenantID] = res

	s.recordAudit(tenantID, actorMemberID, "config.validated", "Tenant", tenantID, nil, map[string]interface{}{
		"valid":       res.Valid,
		"error_count": len(errorsList),
	}, "127.0.0.1")

	return res, nil
}

func (s *MemoryService) GetDraftSummary(ctx context.Context, tenantID string) (*ConfigDraftSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routesCount := 0
	for _, r := range s.routes {
		if r.TenantID == tenantID {
			routesCount++
		}
	}

	policiesCount := 0
	for _, p := range s.policies {
		if p.TenantID == tenantID {
			policiesCount++
		}
	}

	var lastRes *ValidationResult
	var lastTime *int64
	isValidated := false
	if s.lastValidation != nil {
		if res, ok := s.lastValidation[tenantID]; ok {
			lastRes = res
			lastTime = &res.ValidatedAt
			isValidated = res.Valid
		}
	}

	return &ConfigDraftSummary{
		TenantID:        tenantID,
		PendingChanges:  routesCount + policiesCount,
		RoutesCount:     routesCount,
		PoliciesCount:   policiesCount,
		LastValidatedAt: lastTime,
		IsValidated:     isValidated,
		LastResult:      lastRes,
	}, nil
}

func (s *MemoryService) GetDraftDiff(ctx context.Context, tenantID string) (*ConfigDraftDiff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routeDiffs := make([]RouteDiffItem, 0)
	for _, r := range s.routes {
		if r.TenantID == tenantID {
			routeDiffs = append(routeDiffs, RouteDiffItem{
				RouteID:     r.RouteID,
				Action:      "added",
				PathPattern: r.PathPattern,
				After: map[string]interface{}{
					"path_pattern":            r.PathPattern,
					"upstream_url":            r.UpstreamURL,
					"price_base_units":        r.PriceBaseUnits,
					"allowed_payment_methods": r.AllowedPaymentMethods,
					"status":                  string(r.Status),
				},
			})
		}
	}

	policyDiffs := make([]PolicyDiffItem, 0)
	for _, p := range s.policies {
		if p.TenantID == tenantID {
			policyDiffs = append(policyDiffs, PolicyDiffItem{
				PolicyID: p.PolicyID,
				Action:   "added",
				After: map[string]interface{}{
					"max_amount_per_request": p.MaxAmountPerRequest,
					"rolling_24h_budget_cap": p.Rolling24hBudgetCap,
					"is_active":              p.IsActive,
				},
			})
		}
	}

	return &ConfigDraftDiff{
		TenantID:    tenantID,
		HasChanges:  len(routeDiffs) > 0 || len(policyDiffs) > 0,
		RouteDiffs:  routeDiffs,
		PolicyDiffs: policyDiffs,
	}, nil
}

// ----------------------------------------------------------------------------
// Config Publishing, Versions & Rollback (T4.4)
// ----------------------------------------------------------------------------

func (s *MemoryService) PublishConfig(ctx context.Context, tenantID string, changeSummary string, actorMemberID string) (*ConfigVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}

	var publisherEmail string
	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionRouteWrite); err != nil {
			return nil, err
		}
		publisherEmail = actor.Email
	} else {
		publisherEmail = "admin@agentpay.io"
	}

	// 1. Enforce static validation matrix (T4.3 Gate)
	valErrors := make([]ValidationError, 0)
	routes := make([]*Route, 0)
	for _, r := range s.routes {
		if r.TenantID == tenantID {
			routes = append(routes, r)
		}
	}
	policies := make([]*Policy, 0)
	for _, p := range s.policies {
		if p.TenantID == tenantID {
			policies = append(policies, p)
		}
	}

	seenPatterns := make(map[string]string)
	for _, r := range routes {
		if !strings.HasPrefix(r.UpstreamURL, "http://") && !strings.HasPrefix(r.UpstreamURL, "https://") {
			valErrors = append(valErrors, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "upstream_url",
				Code:       "INVALID_UPSTREAM_URL",
				Message:    fmt.Sprintf("上游地址 '%s' 非法，必须以 http:// 或 https:// 开头", r.UpstreamURL),
			})
		}
		cleanPath := strings.TrimSpace(r.PathPattern)
		if cleanPath == "" || !strings.HasPrefix(cleanPath, "/") {
			valErrors = append(valErrors, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "path_pattern",
				Code:       "INVALID_PATH_PATTERN",
				Message:    fmt.Sprintf("路由暴露路径 '%s' 非法，必须以 / 开头", r.PathPattern),
			})
		} else {
			if existingID, seen := seenPatterns[cleanPath]; seen {
				valErrors = append(valErrors, ValidationError{
					TargetType: "Route",
					TargetID:   r.RouteID,
					Field:      "path_pattern",
					Code:       "DUPLICATE_PATH_PATTERN",
					Message:    fmt.Sprintf("路由路径 '%s' 与路由 %s 重复冲突", cleanPath, existingID),
				})
			} else {
				seenPatterns[cleanPath] = r.RouteID
			}
		}

		if r.AllowedPaymentMethods != "wallet" {
			if r.PayeeAddress == "" {
				valErrors = append(valErrors, ValidationError{
					TargetType: "Route",
					TargetID:   r.RouteID,
					Field:      "payee_address",
					Code:       "MISSING_PAYEE_ADDRESS",
					Message:    "支持链上结算时必须指定收款地址",
				})
			} else if !ethAddressValidationRegex.MatchString(r.PayeeAddress) {
				valErrors = append(valErrors, ValidationError{
					TargetType: "Route",
					TargetID:   r.RouteID,
					Field:      "payee_address",
					Code:       "INVALID_PAYEE_ADDRESS",
					Message:    fmt.Sprintf("收款地址 '%s' 格式错误，必须为 42 位 0x 开头的有效 EVM 地址", r.PayeeAddress),
				})
			}
		}

		if r.PriceBaseUnits < 0 {
			valErrors = append(valErrors, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "price_base_units",
				Code:       "NEGATIVE_PRICE_UNITS",
				Message:    "单次调用单价不能为负数",
			})
		}

		if r.AllowedPaymentMethods != "wallet" && r.AllowedPaymentMethods != "onchain" && r.AllowedPaymentMethods != "both" {
			valErrors = append(valErrors, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "allowed_payment_methods",
				Code:       "INVALID_PAYMENT_METHOD_OPTION",
				Message:    fmt.Sprintf("支付通道选项 '%s' 无效，必须为 wallet、onchain 或 both", r.AllowedPaymentMethods),
			})
		}
	}

	for _, p := range policies {
		if p.Rolling24hBudgetCap > 0 && p.MaxAmountPerRequest > p.Rolling24hBudgetCap {
			valErrors = append(valErrors, ValidationError{
				TargetType: "Policy",
				TargetID:   p.PolicyID,
				Field:      "max_amount_per_request",
				Code:       "MAX_AMOUNT_EXCEEDS_ROLLING_BUDGET",
				Message:    fmt.Sprintf("单笔限额 (%d) 超出了 24 小时滚动预算上限 (%d)", p.MaxAmountPerRequest, p.Rolling24hBudgetCap),
			})
		}

		if p.CircuitBreakerMaxRPM < 0 {
			valErrors = append(valErrors, ValidationError{
				TargetType: "Policy",
				TargetID:   p.PolicyID,
				Field:      "circuit_breaker_max_rpm",
				Code:       "INVALID_CIRCUIT_BREAKER_RPM",
				Message:    "熔断阈值 RPM 不能小于 0",
			})
		}

		for _, net := range p.WhitelistedNetworks {
			if !caip2ValidationRegex.MatchString(net) {
				valErrors = append(valErrors, ValidationError{
					TargetType: "Policy",
					TargetID:   p.PolicyID,
					Field:      "whitelisted_networks",
					Code:       "INVALID_NETWORK_IDENTIFIER",
					Message:    fmt.Sprintf("网络标识符 '%s' 不符合 CAIP-2 规范 (如 eip155:1 或 eip155:42161)", net),
				})
			}
		}
	}

	if len(valErrors) > 0 {
		return nil, ErrConfigValidationFailed
	}

	// 2. Build compiled RCU route and policy structs
	rcuRoutes := make([]rcu.RouteConfig, 0, len(routes))
	for _, r := range routes {
		rcuRoutes = append(rcuRoutes, rcu.RouteConfig{
			RouteID:               r.RouteID,
			ProjectID:             r.ProjectID,
			PathPattern:           r.PathPattern,
			UpstreamURL:           r.UpstreamURL,
			PriceBaseUnits:        r.PriceBaseUnits,
			Asset:                 r.Asset,
			PayeeAddress:          r.PayeeAddress,
			AllowedPaymentMethods: r.AllowedPaymentMethods,
			TimeoutMS:             r.TimeoutMs,
			RateLimitRPM:          r.RateLimitRPM,
			Status:                string(r.Status),
			UpstreamAuth:          r.UpstreamAuth,
		})
	}

	rcuPolicies := make([]rcu.PolicyConfig, 0, len(policies))
	for _, p := range policies {
		rcuPolicies = append(rcuPolicies, rcu.PolicyConfig{
			PolicyID:             p.PolicyID,
			ProjectID:            p.ProjectID,
			RouteID:              &p.RouteID,
			MaxAmountPerRequest:  p.MaxAmountPerRequest,
			Rolling24hBudgetCap:  p.Rolling24hBudgetCap,
			CircuitBreakerMaxRPM: p.CircuitBreakerMaxRPM,
			WhitelistedNetworks:  p.WhitelistedNetworks,
			WhitelistedRoutes:    p.WhitelistedRoutes,
			IsActive:             p.IsActive,
		})
	}

	// 3. Compute deterministic canonical snapshot hash
	snapshotHash := rcu.ComputeCanonicalSnapshotHash(rcuRoutes, rcuPolicies)

	// 4. Determine next version number
	history := s.configVersionHistory[tenantID]
	var maxVerNum int64 = 0
	for _, v := range history {
		if v.VersionNumber > maxVerNum {
			maxVerNum = v.VersionNumber
		}
	}
	newVersionNumber := maxVerNum + 1
	versionID := GenerateConfigVersionID()
	now := time.Now().Unix()

	summary := strings.TrimSpace(changeSummary)
	if summary == "" {
		summary = fmt.Sprintf("Publish configuration version v%d", newVersionNumber)
	}

	snap := &rcu.ConfigSnapshot{
		SchemaVersion: "2.0.0",
		TenantID:      tenantID,
		VersionID:     versionID,
		VersionNumber: newVersionNumber,
		PublishedAt:   now,
		PublishedBy:   actorMemberID,
		ChangeSummary: summary,
		SnapshotHash:  snapshotHash,
		Routes:        rcuRoutes,
		Policies:      rcuPolicies,
	}

	// 5. Deactivate previous versions
	for _, v := range history {
		v.IsActive = false
	}

	// 6. Record new active snapshot
	newVer := &ConfigVersion{
		VersionID:      versionID,
		TenantID:       tenantID,
		VersionNumber:  newVersionNumber,
		SnapshotJSON:   snap,
		SnapshotHash:   snapshotHash,
		PublishedBy:    actorMemberID,
		PublisherEmail: publisherEmail,
		ChangeSummary:  summary,
		IsActive:       true,
		RoutesCount:    len(rcuRoutes),
		PoliciesCount:  len(rcuPolicies),
		CreatedAt:      now,
	}

	s.configVersions[versionID] = newVer
	s.configVersionHistory[tenantID] = append(s.configVersionHistory[tenantID], newVer)

	// 7. Atomic RCU swap in data plane
	if s.rcuMgr != nil {
		_ = s.rcuMgr.Update(snap)
	}

	// 8. Audit event
	s.recordAudit(tenantID, actorMemberID, "config.published", "ConfigVersion", versionID, nil, map[string]interface{}{
		"version_number": newVersionNumber,
		"snapshot_hash":  snapshotHash,
		"change_summary": summary,
		"routes_count":   len(rcuRoutes),
		"policies_count": len(rcuPolicies),
	}, "127.0.0.1")

	return newVer, nil
}

func (s *MemoryService) ListConfigVersions(ctx context.Context, tenantID string) ([]*ConfigVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history, ok := s.configVersionHistory[tenantID]
	if !ok || len(history) == 0 {
		return []*ConfigVersion{}, nil
	}

	res := make([]*ConfigVersion, len(history))
	for i, v := range history {
		res[len(history)-1-i] = v
	}
	return res, nil
}

func (s *MemoryService) GetConfigVersion(ctx context.Context, tenantID, versionID string) (*ConfigVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ver, ok := s.configVersions[versionID]
	if !ok || ver.TenantID != tenantID {
		return nil, ErrConfigVersionNotFound
	}
	return ver, nil
}

func (s *MemoryService) RollbackConfig(ctx context.Context, tenantID, targetVersionID, reason, actorMemberID string) (*ConfigVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.tenants[tenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}

	var publisherEmail string
	if actorMemberID != "" {
		actor, ok := s.members[actorMemberID]
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionRouteWrite); err != nil {
			return nil, err
		}
		publisherEmail = actor.Email
	} else {
		publisherEmail = "admin@agentpay.io"
	}

	targetVer, ok := s.configVersions[targetVersionID]
	if !ok || targetVer.TenantID != tenantID {
		return nil, ErrConfigVersionNotFound
	}
	if targetVer.IsActive {
		return nil, ErrCannotRollbackToActive
	}

	history := s.configVersionHistory[tenantID]
	var maxVerNum int64 = 0
	for _, v := range history {
		if v.VersionNumber > maxVerNum {
			maxVerNum = v.VersionNumber
		}
	}
	newVersionNumber := maxVerNum + 1
	versionID := GenerateConfigVersionID()
	now := time.Now().Unix()

	summary := fmt.Sprintf("Rollback to version v%d", targetVer.VersionNumber)
	if strings.TrimSpace(reason) != "" {
		summary = fmt.Sprintf("Rollback to version v%d: %s", targetVer.VersionNumber, strings.TrimSpace(reason))
	}

	// Snapshot content is identical to target version, retaining targetVer.SnapshotHash
	restoredRoutes := make([]rcu.RouteConfig, len(targetVer.SnapshotJSON.Routes))
	copy(restoredRoutes, targetVer.SnapshotJSON.Routes)
	restoredPolicies := make([]rcu.PolicyConfig, len(targetVer.SnapshotJSON.Policies))
	copy(restoredPolicies, targetVer.SnapshotJSON.Policies)

	restoredSnap := &rcu.ConfigSnapshot{
		SchemaVersion: "2.0.0",
		TenantID:      tenantID,
		VersionID:     versionID,
		VersionNumber: newVersionNumber,
		PublishedAt:   now,
		PublishedBy:   actorMemberID,
		ChangeSummary: summary,
		SnapshotHash:  targetVer.SnapshotHash,
		Routes:        restoredRoutes,
		Policies:      restoredPolicies,
	}

	// Deactivate existing
	for _, v := range history {
		v.IsActive = false
	}

	newVer := &ConfigVersion{
		VersionID:      versionID,
		TenantID:       tenantID,
		VersionNumber:  newVersionNumber,
		SnapshotJSON:   restoredSnap,
		SnapshotHash:   targetVer.SnapshotHash,
		PublishedBy:    actorMemberID,
		PublisherEmail: publisherEmail,
		ChangeSummary:  summary,
		IsActive:       true,
		RoutesCount:    len(restoredRoutes),
		PoliciesCount:  len(restoredPolicies),
		CreatedAt:      now,
	}

	s.configVersions[versionID] = newVer
	s.configVersionHistory[tenantID] = append(s.configVersionHistory[tenantID], newVer)

	// Sync restored routes back to memory service active routes
	for id, r := range s.routes {
		if r.TenantID == tenantID {
			delete(s.routes, id)
		}
	}
	for _, rr := range restoredRoutes {
		s.routes[rr.RouteID] = &Route{
			RouteID:               rr.RouteID,
			TenantID:              tenantID,
			ProjectID:             rr.ProjectID,
			PathPattern:           rr.PathPattern,
			UpstreamURL:           rr.UpstreamURL,
			PriceBaseUnits:        rr.PriceBaseUnits,
			Asset:                 rr.Asset,
			PayeeAddress:          rr.PayeeAddress,
			AllowedPaymentMethods: rr.AllowedPaymentMethods,
			TimeoutMs:             rr.TimeoutMS,
			RateLimitRPM:          rr.RateLimitRPM,
			Status:                RouteStatus(rr.Status),
			UpstreamAuth:          rr.UpstreamAuth,
			CreatedAt:             now,
			UpdatedAt:             now,
		}
	}

	// Atomic RCU swap
	if s.rcuMgr != nil {
		_ = s.rcuMgr.Update(restoredSnap)
	}

	// Audit
	s.recordAudit(tenantID, actorMemberID, "config.rolled_back", "ConfigVersion", versionID, map[string]interface{}{
		"target_version_id":     targetVer.VersionID,
		"target_version_number": targetVer.VersionNumber,
	}, map[string]interface{}{
		"new_version_id":     versionID,
		"new_version_number": newVersionNumber,
		"active_hash":        targetVer.SnapshotHash,
		"reason":             reason,
	}, "127.0.0.1")

	return newVer, nil
}

func (s *MemoryService) GetConfigVersionDiff(ctx context.Context, tenantID, baseVersionID, targetVersionID string) (*ConfigDraftDiff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var baseSnap *rcu.ConfigSnapshot
	var targetSnap *rcu.ConfigSnapshot

	if baseVersionID != "" {
		v, ok := s.configVersions[baseVersionID]
		if !ok || v.TenantID != tenantID {
			return nil, ErrConfigVersionNotFound
		}
		baseSnap = v.SnapshotJSON
	} else {
		for _, v := range s.configVersionHistory[tenantID] {
			if v.IsActive {
				baseSnap = v.SnapshotJSON
				break
			}
		}
	}

	if targetVersionID != "" {
		v, ok := s.configVersions[targetVersionID]
		if !ok || v.TenantID != tenantID {
			return nil, ErrConfigVersionNotFound
		}
		targetSnap = v.SnapshotJSON
	}

	routeDiffs := make([]RouteDiffItem, 0)
	policyDiffs := make([]PolicyDiffItem, 0)

	baseRouteMap := make(map[string]rcu.RouteConfig)
	if baseSnap != nil {
		for _, r := range baseSnap.Routes {
			baseRouteMap[r.RouteID] = r
		}
	}

	targetRouteMap := make(map[string]rcu.RouteConfig)
	if targetSnap != nil {
		for _, r := range targetSnap.Routes {
			targetRouteMap[r.RouteID] = r
		}
	}

	for id, tr := range targetRouteMap {
		br, exists := baseRouteMap[id]
		if !exists {
			routeDiffs = append(routeDiffs, RouteDiffItem{
				RouteID:     id,
				Action:      "added",
				PathPattern: tr.PathPattern,
				After: map[string]interface{}{
					"upstream_url":     tr.UpstreamURL,
					"price_base_units": tr.PriceBaseUnits,
				},
			})
		} else if br.UpstreamURL != tr.UpstreamURL || br.PriceBaseUnits != tr.PriceBaseUnits || br.PathPattern != tr.PathPattern {
			routeDiffs = append(routeDiffs, RouteDiffItem{
				RouteID:     id,
				Action:      "modified",
				PathPattern: tr.PathPattern,
				Before: map[string]interface{}{
					"upstream_url":     br.UpstreamURL,
					"price_base_units": br.PriceBaseUnits,
				},
				After: map[string]interface{}{
					"upstream_url":     tr.UpstreamURL,
					"price_base_units": tr.PriceBaseUnits,
				},
			})
		}
	}

	for id, br := range baseRouteMap {
		if _, exists := targetRouteMap[id]; !exists {
			routeDiffs = append(routeDiffs, RouteDiffItem{
				RouteID:     id,
				Action:      "deleted",
				PathPattern: br.PathPattern,
				Before: map[string]interface{}{
					"upstream_url":     br.UpstreamURL,
					"price_base_units": br.PriceBaseUnits,
				},
			})
		}
	}

	activeVerID := ""
	if baseSnap != nil {
		activeVerID = baseSnap.VersionID
	}

	return &ConfigDraftDiff{
		TenantID:        tenantID,
		ActiveVersionID: activeVerID,
		HasChanges:      len(routeDiffs) > 0 || len(policyDiffs) > 0,
		RouteDiffs:      routeDiffs,
		PolicyDiffs:     policyDiffs,
	}, nil
}

// ----------------------------------------------------------------------------
// Upstream Authentication & Connectivity Testing (T4.2 Enhancement)
// ----------------------------------------------------------------------------

func (s *MemoryService) GetAuthTypes() []upstreamauth.AuthTypeSchema {
	return upstreamauth.ListSchemas()
}

func (s *MemoryService) TestUpstream(ctx context.Context, tenantID string, input *TestUpstreamInput, actorMemberID string) (*TestUpstreamResult, error) {
	if actorMemberID != "" {
		s.mu.RLock()
		actor, ok := s.members[actorMemberID]
		s.mu.RUnlock()
		if !ok || actor.TenantID != tenantID {
			return nil, ErrPermissionDenied
		}
		if err := EnforcePermission(actor, PermissionRouteRead); err != nil {
			return nil, err
		}
	}

	if input == nil || strings.TrimSpace(input.UpstreamURL) == "" {
		return &TestUpstreamResult{
			Success: false,
			Error:   "upstream_url is required",
		}, nil
	}

	cleanURL := strings.TrimSpace(input.UpstreamURL)
	targetURL, err := url.Parse(cleanURL)
	if err != nil || targetURL.Scheme == "" || targetURL.Host == "" {
		return &TestUpstreamResult{
			Success: false,
			Error:   "invalid upstream_url: must include scheme (http/https) and host",
		}, nil
	}

	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, cleanURL, nil)
	if err != nil {
		return &TestUpstreamResult{
			Success: false,
			Error:   fmt.Sprintf("failed to construct test request: %v", err),
		}, nil
	}

	// Apply configured upstream authentication
	if input.UpstreamAuth != nil && input.UpstreamAuth.Type != "" && input.UpstreamAuth.Type != "none" {
		if auth, ok := upstreamauth.Get(input.UpstreamAuth.Type); ok {
			secrets := input.UpstreamAuth.Secrets
			if len(secrets) == 0 && len(input.UpstreamAuth.SecretEnc) > 0 {
				dec, _ := upstreamauth.DecryptSecrets(input.UpstreamAuth.SecretEnc, nil)
				secrets = dec
			}
			if err := auth.Apply(ctx, req, input.UpstreamAuth.Params, secrets); err != nil {
				return &TestUpstreamResult{
					Success: false,
					Error:   fmt.Sprintf("failed to apply upstream auth: %v", err),
				}, nil
			}
		}
	}

	// Sanitize request headers for display
	sentHeaders := make(map[string]string)
	for k, vv := range req.Header {
		val := strings.Join(vv, ", ")
		sentHeaders[k] = upstreamauth.MaskHeader(k, val)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("stopped after 3 redirects")
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()

	result := &TestUpstreamResult{
		SentHeaders: sentHeaders,
		LatencyMs:   latency,
	}

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("upstream connection error: %v", err)
	} else {
		defer resp.Body.Close()
		result.StatusCode = resp.StatusCode
		result.StatusText = resp.Status
		result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
	}

	s.recordAudit(tenantID, actorMemberID, "route.tested", "Route", "", nil, map[string]interface{}{
		"url":         cleanURL,
		"auth_type":   "",
		"success":     result.Success,
		"status_code": result.StatusCode,
	}, "127.0.0.1")

	return result, nil
}

// SetUsageService configures a custom usage service on MemoryService.
func (s *MemoryService) SetUsageService(svc usage.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usageSvc = svc
}

// GetUsageService returns the active usage service or initializes an in-memory one.
func (s *MemoryService) GetUsageService() usage.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.usageSvc == nil {
		s.usageSvc = usage.NewMemoryService()
	}
	return s.usageSvc
}

// ListRequestRecords retrieves request records with RBAC and tenant boundary enforcement.
func (s *MemoryService) ListRequestRecords(ctx context.Context, tenantID string, filter usage.RequestFilter, actorMemberID string) ([]*usage.RequestRecord, int64, error) {
	if actorMemberID != "" {
		member, err := s.GetMember(ctx, tenantID, actorMemberID)
		if err != nil {
			return nil, 0, err
		}
		if err := EnforcePermission(member, PermissionAuditRead); err != nil {
			return nil, 0, err
		}
	}
	filter.TenantID = tenantID
	return s.GetUsageService().ListRequestRecords(ctx, filter)
}

// GetRequestRecord retrieves a single request record by ID with RBAC and tenant boundary enforcement.
func (s *MemoryService) GetRequestRecord(ctx context.Context, tenantID, requestID string, actorMemberID string) (*usage.RequestRecord, error) {
	if actorMemberID != "" {
		member, err := s.GetMember(ctx, tenantID, actorMemberID)
		if err != nil {
			return nil, err
		}
		if err := EnforcePermission(member, PermissionAuditRead); err != nil {
			return nil, err
		}
	}
	return s.GetUsageService().GetRequestRecord(ctx, tenantID, requestID)
}

// ListUsageEvents retrieves usage events with RBAC and tenant boundary enforcement.
func (s *MemoryService) ListUsageEvents(ctx context.Context, tenantID string, filter usage.UsageFilter, actorMemberID string) ([]*usage.UsageEvent, int64, error) {
	if actorMemberID != "" {
		member, err := s.GetMember(ctx, tenantID, actorMemberID)
		if err != nil {
			return nil, 0, err
		}
		if err := EnforcePermission(member, PermissionAuditRead); err != nil {
			return nil, 0, err
		}
	}
	filter.TenantID = tenantID
	return s.GetUsageService().ListUsageEvents(ctx, filter)
}

// GetUsageEvent retrieves a single usage event by ID with RBAC and tenant boundary enforcement.
func (s *MemoryService) GetUsageEvent(ctx context.Context, tenantID, eventID string, actorMemberID string) (*usage.UsageEvent, error) {
	if actorMemberID != "" {
		member, err := s.GetMember(ctx, tenantID, actorMemberID)
		if err != nil {
			return nil, err
		}
		if err := EnforcePermission(member, PermissionAuditRead); err != nil {
			return nil, err
		}
	}
	return s.GetUsageService().GetUsageEvent(ctx, tenantID, eventID)
}

// SetPaymentService configures the payment correlation service.
func (s *MemoryService) SetPaymentService(svc payment.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paymentSvc = svc
}

// GetPaymentService returns the active payment correlation service.
func (s *MemoryService) GetPaymentService() payment.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.paymentSvc == nil {
		return payment.NewMemoryService()
	}
	return s.paymentSvc
}

// GetPaymentAudit retrieves a single payment correlation audit record with RBAC and tenant boundary check.
func (s *MemoryService) GetPaymentAudit(ctx context.Context, tenantID, intentID string, actorMemberID string) (*payment.PaymentAuditRecord, error) {
	if actorMemberID != "" {
		member, err := s.GetMember(ctx, tenantID, actorMemberID)
		if err != nil {
			return nil, err
		}
		if err := EnforcePermission(member, PermissionAuditRead); err != nil {
			return nil, err
		}
	}
	return s.GetPaymentService().GetPaymentAudit(ctx, tenantID, intentID)
}

// GetPaymentAuditByRequestID retrieves a payment correlation record by request_id.
func (s *MemoryService) GetPaymentAuditByRequestID(ctx context.Context, tenantID, requestID string, actorMemberID string) (*payment.PaymentAuditRecord, error) {
	if actorMemberID != "" {
		member, err := s.GetMember(ctx, tenantID, actorMemberID)
		if err != nil {
			return nil, err
		}
		if err := EnforcePermission(member, PermissionAuditRead); err != nil {
			return nil, err
		}
	}
	return s.GetPaymentService().GetPaymentAuditByRequestID(ctx, tenantID, requestID)
}

// ListPaymentAudits retrieves a paginated list of payment correlation records with RBAC.
func (s *MemoryService) ListPaymentAudits(ctx context.Context, tenantID string, filter payment.PaymentAuditFilter, actorMemberID string) ([]*payment.PaymentAuditRecord, int64, error) {
	if actorMemberID != "" {
		member, err := s.GetMember(ctx, tenantID, actorMemberID)
		if err != nil {
			return nil, 0, err
		}
		if err := EnforcePermission(member, PermissionAuditRead); err != nil {
			return nil, 0, err
		}
	}
	filter.TenantID = tenantID
	return s.GetPaymentService().ListPaymentAudits(ctx, filter)
}

// SetTimelineService configures the timeline service.
func (s *MemoryService) SetTimelineService(svc timeline.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timelineSvc = svc
}

// GetTimelineService returns the active timeline service.
func (s *MemoryService) GetTimelineService() timeline.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.timelineSvc == nil {
		return timeline.NewMemoryService()
	}
	return s.timelineSvc
}

// GetAgentTimeline retrieves agent timeline events with RBAC, tenant ownership, and session aggregation.
func (s *MemoryService) GetAgentTimeline(ctx context.Context, tenantID, agentID string, filter timeline.TimelineFilter, actorMemberID string) ([]*timeline.TimelineEvent, int64, []string, error) {
	if actorMemberID != "" {
		member, err := s.GetMember(ctx, tenantID, actorMemberID)
		if err != nil {
			return nil, 0, nil, err
		}
		if err := EnforcePermission(member, PermissionAuditRead); err != nil {
			return nil, 0, nil, err
		}
	}

	ag, err := s.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, 0, nil, err
	}
	if ag == nil || ag.TenantID != tenantID {
		return nil, 0, nil, ErrAgentNotFound
	}

	filter.TenantID = tenantID
	filter.AgentID = agentID
	return s.GetTimelineService().GetTimeline(ctx, tenantID, agentID, filter)
}
