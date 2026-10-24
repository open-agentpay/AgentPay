package admin

import (
	"context"
	"database/sql"
	"encoding/json"
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

var (
	pgEthAddressValidationRegex = regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)
	pgCaip2ValidationRegex      = regexp.MustCompile(`^[-a-z0-9]{3,8}:[-_a-zA-Z0-9]{1,32}$`)
)

// PostgresService implements admin.Service backed by PostgreSQL.
type PostgresService struct {
	db             *sql.DB
	agentReg       *agent.Registry
	walletSvc      wallet.Service
	usageSvc       usage.Service
	paymentSvc     payment.Service
	timelineSvc    timeline.Service
	mu             sync.RWMutex
	lastValidation map[string]*ValidationResult
	rcuMgr         *rcu.Manager
}

// NewPostgresService initializes a PostgreSQL admin service and synchronizes the active agents/keys into the in-memory registry.
func NewPostgresService(db *sql.DB, agentReg *agent.Registry, walletSvc wallet.Service) (*PostgresService, error) {
	svc := &PostgresService{
		db:             db,
		agentReg:       agentReg,
		walletSvc:      walletSvc,
		lastValidation: make(map[string]*ValidationResult),
	}
	if err := svc.initSync(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to synchronize agent registry from postgres: %w", err)
	}
	return svc, nil
}

// initSync loads active agents and active API keys from PostgreSQL into the fast-path in-memory registry.
func (p *PostgresService) initSync(ctx context.Context) error {
	// 0. Ensure api_auth_configs table and marketplace columns on ap_route exist
	_, _ = p.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS api_auth_configs (
			id VARCHAR(64) PRIMARY KEY,
			tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
			api_id VARCHAR(64) NOT NULL UNIQUE REFERENCES ap_route(route_id) ON DELETE CASCADE,
			type VARCHAR(64) NOT NULL,
			params JSONB NOT NULL DEFAULT '{}'::jsonb,
			secret_enc BYTEA NULL,
			key_version INT NOT NULL DEFAULT 1,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_api_auth_configs_tenant ON api_auth_configs(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_api_auth_configs_api ON api_auth_configs(api_id);

		ALTER TABLE ap_route ADD COLUMN IF NOT EXISTS slug VARCHAR(128) NULL;
		ALTER TABLE ap_route ADD COLUMN IF NOT EXISTS title VARCHAR(256) NOT NULL DEFAULT '';
		ALTER TABLE ap_route ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE ap_route ADD COLUMN IF NOT EXISTS category VARCHAR(64) NOT NULL DEFAULT 'General';
		ALTER TABLE ap_route ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb;
		ALTER TABLE ap_route ADD COLUMN IF NOT EXISTS description_md TEXT NOT NULL DEFAULT '';
		ALTER TABLE ap_route ADD COLUMN IF NOT EXISTS network VARCHAR(64) NOT NULL DEFAULT 'eip155:421614';
	`)

	if p.agentReg == nil {
		return nil
	}

	// 1. Load active/suspended agents
	agentRows, err := p.db.QueryContext(ctx, `
		SELECT agent_id, tenant_id, project_id, name, status, policy_config
		FROM ap_agent
		WHERE status != 'revoked';
	`)
	if err != nil {
		return err
	}
	defer agentRows.Close()

	for agentRows.Next() {
		var agID, tID, pID, name, status string
		var polCfgBytes []byte
		if err := agentRows.Scan(&agID, &tID, &pID, &name, &status, &polCfgBytes); err != nil {
			return err
		}

		var polCfg map[string]interface{}
		_ = json.Unmarshal(polCfgBytes, &polCfg)

		p.agentReg.RegisterAgent(&agent.AgentIdentity{
			AgentID:   agID,
			TenantID:  tID,
			ProjectID: pID,
			Name:      name,
			Status:    agent.AgentStatus(status),
		}, nil)
	}

	// 2. Load active API keys
	keyRows, err := p.db.QueryContext(ctx, `
		SELECT key_hash, agent_id
		FROM ap_api_key
		WHERE status = 'active';
	`)
	if err != nil {
		return err
	}
	defer keyRows.Close()

	for keyRows.Next() {
		var keyHash, agID string
		if err := keyRows.Scan(&keyHash, &agID); err != nil {
			return err
		}
		p.agentReg.BindAPIKeyHash(keyHash, agID)
	}

	return nil
}

func (p *PostgresService) recordAudit(ctx context.Context, tenantID, operatorID, action, targetType, targetID string, before, after map[string]interface{}, clientIP string) {
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}
	auditID := GenerateAuditID()

	var beforeJSON, afterJSON []byte
	if before != nil {
		beforeJSON, _ = json.Marshal(before)
	}
	if after != nil {
		afterJSON, _ = json.Marshal(after)
	}

	query := `
		INSERT INTO ap_audit_event (
			audit_id, tenant_id, operator_id, action, target_type, target_id, before_state, after_state, client_ip, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW());
	`
	_, _ = p.db.ExecContext(ctx, query, auditID, tenantID, operatorID, action, targetType, targetID, beforeJSON, afterJSON, clientIP)
}

func (p *PostgresService) getMember(ctx context.Context, tenantID, memberID string) (*Member, error) {
	query := `
		SELECT member_id, tenant_id, email, password_hash, role, status,
		       EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_member
		WHERE member_id = $1;
	`
	var m Member
	var pwd sql.NullString
	err := p.db.QueryRowContext(ctx, query, memberID).Scan(
		&m.MemberID, &m.TenantID, &m.Email, &pwd, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	if tenantID != "" && m.TenantID != tenantID {
		return nil, ErrMemberNotFound
	}
	if pwd.Valid {
		m.PasswordHash = pwd.String
	}
	return &m, nil
}

func (p *PostgresService) enforcePermission(ctx context.Context, actorMemberID, tenantID string, perm Permission) error {
	if actorMemberID == "" {
		return nil
	}
	actor, err := p.getMember(ctx, tenantID, actorMemberID)
	if err != nil {
		return ErrPermissionDenied
	}
	if actor.TenantID != tenantID {
		return ErrPermissionDenied
	}
	return EnforcePermission(actor, perm)
}

// ----------------------------------------------------------------------------
// Auth
// ----------------------------------------------------------------------------

func (p *PostgresService) Register(ctx context.Context, tenantName, email, password string, currency ...string) (*Tenant, *Member, error) {
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

	rawCurr := ""
	if len(currency) > 0 {
		rawCurr = strings.TrimSpace(currency[0])
	}
	curr, err := NormalizeCurrency(rawCurr)
	if err != nil {
		return nil, nil, err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, nil, err
	}

	var exists bool
	_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_tenant WHERE name = $1);`, cleanName).Scan(&exists)
	if exists {
		return nil, nil, ErrTenantAlreadyExists
	}
	_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_member WHERE LOWER(email) = LOWER($1));`, cleanEmail).Scan(&exists)
	if exists {
		return nil, nil, ErrEmailAlreadyRegistered
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	tid := GenerateTenantID()
	mid := GenerateMemberID()
	now := time.Now()
	nowUnix := now.Unix()

	// 1. Insert Tenant
	_, err = tx.ExecContext(ctx, `
		INSERT INTO ap_tenant (tenant_id, name, currency, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', $4, $4);
	`, tid, cleanName, curr, now)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	// 2. Insert Owner Member
	_, err = tx.ExecContext(ctx, `
		INSERT INTO ap_member (member_id, tenant_id, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'owner', 'active', $5, $5);
	`, mid, tid, cleanEmail, hash, now)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create owner member: %w", err)
	}

	// 3. Insert Master Virtual Wallet
	walletID := fmt.Sprintf("wlt_%s", tid)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO ap_wallet (wallet_id, tenant_id, asset, status, version, available_cache, held_cache, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', 1, 0, 0, $4, $4)
		ON CONFLICT (tenant_id) DO NOTHING;
	`, walletID, tid, curr, now)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize wallet: %w", err)
	}

	// 4. Audit event
	auditID := GenerateAuditID()
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO ap_audit_event (audit_id, tenant_id, operator_id, action, target_type, target_id, after_state, client_ip, created_at)
		VALUES ($1, $2, $3, 'auth.register', 'Tenant', $2, $4, '127.0.0.1', $5);
	`, auditID, tid, mid, `{"name":"`+cleanName+`","email":"`+cleanEmail+`","currency":"`+curr+`"}`, now)

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	t := &Tenant{
		TenantID:  tid,
		Name:      cleanName,
		Currency:  curr,
		Status:    TenantStatusActive,
		CreatedAt: nowUnix,
		UpdatedAt: nowUnix,
	}
	m := &Member{
		MemberID:     mid,
		TenantID:     tid,
		Email:        cleanEmail,
		PasswordHash: hash,
		Role:         RoleOwner,
		Status:       MemberStatusActive,
		CreatedAt:    nowUnix,
		UpdatedAt:    nowUnix,
	}
	return t, m, nil
}

func (p *PostgresService) Login(ctx context.Context, email, password string) (*Member, *Tenant, error) {
	cleanEmail := strings.TrimSpace(email)
	if cleanEmail == "" || password == "" {
		return nil, nil, ErrInvalidCredentials
	}

	query := `
		SELECT m.member_id, m.tenant_id, m.email, m.password_hash, m.role, m.status,
		       EXTRACT(EPOCH FROM m.created_at)::bigint, EXTRACT(EPOCH FROM m.updated_at)::bigint,
		       t.name, t.currency, t.status, EXTRACT(EPOCH FROM t.created_at)::bigint, EXTRACT(EPOCH FROM t.updated_at)::bigint
		FROM ap_member m
		JOIN ap_tenant t ON m.tenant_id = t.tenant_id
		WHERE LOWER(m.email) = LOWER($1);
	`
	var m Member
	var t Tenant
	var pwd sql.NullString
	err := p.db.QueryRowContext(ctx, query, cleanEmail).Scan(
		&m.MemberID, &m.TenantID, &m.Email, &pwd, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt,
		&t.Name, &t.Currency, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, err
	}
	t.TenantID = m.TenantID
	if t.Currency == "" {
		t.Currency = "USD"
	}

	if m.Status == MemberStatusDisabled {
		return nil, nil, ErrMemberDisabled
	}
	if t.Status == TenantStatusTerminated {
		return nil, nil, ErrTenantTerminated
	}
	if t.Status == TenantStatusSuspended {
		return nil, nil, ErrTenantSuspended
	}

	if !pwd.Valid || !auth.VerifyPassword(password, pwd.String) {
		return nil, nil, ErrInvalidCredentials
	}

	m.PasswordHash = pwd.String
	p.recordAudit(ctx, t.TenantID, m.MemberID, "auth.login", "Member", m.MemberID, nil, map[string]interface{}{"email": m.Email}, "127.0.0.1")
	return &m, &t, nil
}

func (p *PostgresService) GetMemberByEmail(ctx context.Context, email string) (*Member, *Tenant, error) {
	cleanEmail := strings.TrimSpace(email)
	query := `
		SELECT m.member_id, m.tenant_id, m.email, m.password_hash, m.role, m.status,
		       EXTRACT(EPOCH FROM m.created_at)::bigint, EXTRACT(EPOCH FROM m.updated_at)::bigint,
		       t.name, t.currency, t.status, EXTRACT(EPOCH FROM t.created_at)::bigint, EXTRACT(EPOCH FROM t.updated_at)::bigint
		FROM ap_member m
		JOIN ap_tenant t ON m.tenant_id = t.tenant_id
		WHERE LOWER(m.email) = LOWER($1);
	`
	var m Member
	var t Tenant
	var pwd sql.NullString
	err := p.db.QueryRowContext(ctx, query, cleanEmail).Scan(
		&m.MemberID, &m.TenantID, &m.Email, &pwd, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt,
		&t.Name, &t.Currency, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	t.TenantID = m.TenantID
	if t.Currency == "" {
		t.Currency = "USD"
	}
	if pwd.Valid {
		m.PasswordHash = pwd.String
	}
	return &m, &t, nil
}

func (p *PostgresService) SetMemberPassword(ctx context.Context, tenantID, memberID, password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	res, err := p.db.ExecContext(ctx, `
		UPDATE ap_member
		SET password_hash = $1, updated_at = NOW()
		WHERE tenant_id = $2 AND member_id = $3;
	`, hash, tenantID, memberID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// ----------------------------------------------------------------------------
// Tenants
// ----------------------------------------------------------------------------

func (p *PostgresService) CreateTenant(ctx context.Context, name string, tenantID string, ownerEmail string, currency ...string) (*Tenant, *Member, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, nil, fmt.Errorf("tenant name cannot be empty")
	}

	rawCurr := ""
	if len(currency) > 0 {
		rawCurr = strings.TrimSpace(currency[0])
	}
	curr, err := NormalizeCurrency(rawCurr)
	if err != nil {
		return nil, nil, err
	}

	var exists bool
	_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_tenant WHERE name = $1);`, cleanName).Scan(&exists)
	if exists {
		return nil, nil, ErrTenantAlreadyExists
	}

	tid := tenantID
	if tid == "" {
		tid = GenerateTenantID()
	}

	now := time.Now()
	nowUnix := now.Unix()

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO ap_tenant (tenant_id, name, currency, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', $4, $4);
	`, tid, cleanName, curr, now)
	if err != nil {
		return nil, nil, err
	}

	var ownerMember *Member
	if ownerEmail != "" {
		cleanOwnerEmail := strings.TrimSpace(ownerEmail)
		mid := GenerateMemberID()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ap_member (member_id, tenant_id, email, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'owner', 'active', $4, $4);
		`, mid, tid, cleanOwnerEmail, now)
		if err != nil {
			return nil, nil, err
		}
		ownerMember = &Member{
			MemberID:  mid,
			TenantID:  tid,
			Email:     cleanOwnerEmail,
			Role:      RoleOwner,
			Status:    MemberStatusActive,
			CreatedAt: nowUnix,
			UpdatedAt: nowUnix,
		}
	}

	// Master virtual wallet
	walletID := fmt.Sprintf("wlt_%s", tid)
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO ap_wallet (wallet_id, tenant_id, asset, status, version, available_cache, held_cache, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', 1, 0, 0, $4, $4)
		ON CONFLICT (tenant_id) DO NOTHING;
	`, walletID, tid, curr, now)

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	t := &Tenant{
		TenantID:  tid,
		Name:      cleanName,
		Currency:  curr,
		Status:    TenantStatusActive,
		CreatedAt: nowUnix,
		UpdatedAt: nowUnix,
	}
	p.recordAudit(ctx, tid, "system", "tenant.created", "Tenant", tid, nil, map[string]interface{}{"name": cleanName, "currency": curr}, "127.0.0.1")
	return t, ownerMember, nil
}

func (p *PostgresService) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	query := `
		SELECT tenant_id, name, currency, status, EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_tenant
		WHERE tenant_id = $1;
	`
	var t Tenant
	err := p.db.QueryRowContext(ctx, query, tenantID).Scan(&t.TenantID, &t.Name, &t.Currency, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTenantNotFound
	}
	if err != nil {
		return nil, err
	}
	if t.Currency == "" {
		t.Currency = "USD"
	}
	return &t, nil
}

func (p *PostgresService) ListTenants(ctx context.Context) ([]*Tenant, error) {
	query := `
		SELECT tenant_id, name, currency, status, EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_tenant
		ORDER BY created_at DESC;
	`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tenants := make([]*Tenant, 0)
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.TenantID, &t.Name, &t.Currency, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if t.Currency == "" {
			t.Currency = "USD"
		}
		tenants = append(tenants, &t)
	}
	return tenants, nil
}

func (p *PostgresService) UpdateTenant(ctx context.Context, tenantID string, name string, actorMemberID string) (*Tenant, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("tenant name cannot be empty")
	}

	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionTenantUpdate); err != nil {
		return nil, err
	}

	t, err := p.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if t.Status == TenantStatusTerminated {
		return nil, ErrTenantTerminated
	}

	before := map[string]interface{}{"name": t.Name}
	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_tenant
		SET name = $1, updated_at = NOW()
		WHERE tenant_id = $2;
	`, cleanName, tenantID)
	if err != nil {
		return nil, err
	}

	t.Name = cleanName
	t.UpdatedAt = time.Now().Unix()
	p.recordAudit(ctx, tenantID, actorMemberID, "tenant.updated", "Tenant", tenantID, before, map[string]interface{}{"name": cleanName}, "127.0.0.1")
	return t, nil
}

func (p *PostgresService) UpdateTenantCurrency(ctx context.Context, tenantID string, currency string, actorMemberID string) (*Tenant, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionTenantUpdate); err != nil {
		return nil, err
	}

	curr, err := NormalizeCurrency(strings.TrimSpace(currency))
	if err != nil {
		return nil, err
	}

	t, err := p.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if t.Status == TenantStatusTerminated {
		return nil, ErrTenantTerminated
	}

	// Enforce C2 immutability: check if routes or ledger entries exist
	var routeCount, ledgerCount int
	_ = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ap_route WHERE tenant_id = $1;`, tenantID).Scan(&routeCount)
	_ = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ap_ledger_entry WHERE tenant_id = $1;`, tenantID).Scan(&ledgerCount)
	if routeCount > 0 || ledgerCount > 0 {
		return nil, ErrCurrencyLocked
	}

	before := map[string]interface{}{"currency": t.Currency}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `UPDATE ap_tenant SET currency = $1, updated_at = NOW() WHERE tenant_id = $2;`, curr, tenantID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE ap_wallet SET asset = $1, updated_at = NOW() WHERE tenant_id = $2;`, curr, tenantID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	t.Currency = curr
	t.UpdatedAt = time.Now().Unix()
	p.recordAudit(ctx, tenantID, actorMemberID, "tenant.currency.updated", "Tenant", tenantID, before, map[string]interface{}{"currency": curr}, "127.0.0.1")
	return t, nil
}

func (p *PostgresService) UpdateTenantStatus(ctx context.Context, tenantID string, status TenantStatus, actorMemberID string) (*Tenant, error) {
	if actorMemberID != "" {
		actor, err := p.getMember(ctx, tenantID, actorMemberID)
		if err != nil || actor.TenantID != tenantID || actor.Role != RoleOwner {
			return nil, ErrPermissionDenied
		}
	}

	t, err := p.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if t.Status == TenantStatusTerminated && status != TenantStatusTerminated {
		return nil, ErrInvalidStatusTransition
	}

	before := map[string]interface{}{"status": string(t.Status)}
	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_tenant
		SET status = $1, updated_at = NOW()
		WHERE tenant_id = $2;
	`, string(status), tenantID)
	if err != nil {
		return nil, err
	}

	t.Status = status
	t.UpdatedAt = time.Now().Unix()
	p.recordAudit(ctx, tenantID, actorMemberID, "tenant.status.changed", "Tenant", tenantID, before, map[string]interface{}{"status": string(status)}, "127.0.0.1")
	return t, nil
}

// ----------------------------------------------------------------------------
// Projects
// ----------------------------------------------------------------------------

func (p *PostgresService) CreateProject(ctx context.Context, tenantID, name string, budgetCap int64, projectID string, actorMemberID string) (*Project, error) {
	t, err := p.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if t.Status == TenantStatusSuspended {
		return nil, ErrTenantSuspended
	}
	if t.Status == TenantStatusTerminated {
		return nil, ErrTenantTerminated
	}

	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionProjectCreate); err != nil {
		return nil, err
	}

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	var exists bool
	_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_project WHERE tenant_id = $1 AND name = $2);`, tenantID, cleanName).Scan(&exists)
	if exists {
		return nil, ErrProjectAlreadyExists
	}

	pid := projectID
	if pid == "" {
		pid = GenerateProjectID()
	}

	now := time.Now()
	nowUnix := now.Unix()

	_, err = p.db.ExecContext(ctx, `
		INSERT INTO ap_project (project_id, tenant_id, name, budget_cap_base_units, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', $5, $5);
	`, pid, tenantID, cleanName, budgetCap, now)
	if err != nil {
		return nil, err
	}

	prj := &Project{
		ProjectID:          pid,
		TenantID:           tenantID,
		Name:               cleanName,
		BudgetCapBaseUnits: budgetCap,
		Status:             ProjectStatusActive,
		CreatedAt:          nowUnix,
		UpdatedAt:          nowUnix,
	}
	p.recordAudit(ctx, tenantID, actorMemberID, "project.created", "Project", pid, nil, map[string]interface{}{"name": cleanName}, "127.0.0.1")
	return prj, nil
}

func (p *PostgresService) GetProject(ctx context.Context, tenantID, projectID string) (*Project, error) {
	query := `
		SELECT project_id, tenant_id, name, budget_cap_base_units, status,
		       EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_project
		WHERE project_id = $1;
	`
	var prj Project
	err := p.db.QueryRowContext(ctx, query, projectID).Scan(
		&prj.ProjectID, &prj.TenantID, &prj.Name, &prj.BudgetCapBaseUnits, &prj.Status, &prj.CreatedAt, &prj.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, err
	}
	if tenantID != "" && prj.TenantID != tenantID {
		return nil, ErrProjectTenantMismatch
	}
	return &prj, nil
}

func (p *PostgresService) ListProjects(ctx context.Context, tenantID string) ([]*Project, error) {
	query := `
		SELECT project_id, tenant_id, name, budget_cap_base_units, status,
		       EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_project
		WHERE tenant_id = $1
		ORDER BY created_at DESC;
	`
	rows, err := p.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]*Project, 0)
	for rows.Next() {
		var prj Project
		if err := rows.Scan(&prj.ProjectID, &prj.TenantID, &prj.Name, &prj.BudgetCapBaseUnits, &prj.Status, &prj.CreatedAt, &prj.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, &prj)
	}
	return projects, nil
}

func (p *PostgresService) UpdateProject(ctx context.Context, tenantID, projectID string, name string, budgetCap *int64, status *ProjectStatus, actorMemberID string) (*Project, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionProjectUpdate); err != nil {
		return nil, err
	}

	prj, err := p.GetProject(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}

	before := map[string]interface{}{"name": prj.Name, "budget_cap": prj.BudgetCapBaseUnits, "status": string(prj.Status)}

	if name != "" {
		cleanName := strings.TrimSpace(name)
		var exists bool
		_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_project WHERE tenant_id = $1 AND name = $2 AND project_id != $3);`, tenantID, cleanName, projectID).Scan(&exists)
		if exists {
			return nil, ErrProjectAlreadyExists
		}
		prj.Name = cleanName
	}
	if budgetCap != nil {
		prj.BudgetCapBaseUnits = *budgetCap
	}
	if status != nil {
		prj.Status = *status
	}

	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_project
		SET name = $1, budget_cap_base_units = $2, status = $3, updated_at = NOW()
		WHERE project_id = $4;
	`, prj.Name, prj.BudgetCapBaseUnits, string(prj.Status), projectID)
	if err != nil {
		return nil, err
	}

	prj.UpdatedAt = time.Now().Unix()
	p.recordAudit(ctx, tenantID, actorMemberID, "project.updated", "Project", projectID, before, map[string]interface{}{"name": prj.Name, "status": string(prj.Status)}, "127.0.0.1")
	return prj, nil
}

func (p *PostgresService) DeleteProject(ctx context.Context, tenantID, projectID string, actorMemberID string) error {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionProjectDelete); err != nil {
		return err
	}

	prj, err := p.GetProject(ctx, tenantID, projectID)
	if err != nil {
		return err
	}

	_, err = p.db.ExecContext(ctx, `DELETE FROM ap_project WHERE project_id = $1;`, projectID)
	if err != nil {
		return err
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "project.deleted", "Project", projectID, map[string]interface{}{"name": prj.Name}, nil, "127.0.0.1")
	return nil
}

// ----------------------------------------------------------------------------
// Members
// ----------------------------------------------------------------------------

func (p *PostgresService) AddMember(ctx context.Context, tenantID, email string, role MemberRole, actorMemberID string) (*Member, error) {
	cleanEmail := strings.TrimSpace(strings.ToLower(email))
	if cleanEmail == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}

	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionMemberCreate); err != nil {
		return nil, err
	}

	var exists bool
	_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_member WHERE tenant_id = $1 AND email = $2);`, tenantID, cleanEmail).Scan(&exists)
	if exists {
		return nil, ErrMemberAlreadyExists
	}

	mid := GenerateMemberID()
	now := time.Now()
	nowUnix := now.Unix()

	_, err := p.db.ExecContext(ctx, `
		INSERT INTO ap_member (member_id, tenant_id, email, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', $5, $5);
	`, mid, tenantID, cleanEmail, string(role), now)
	if err != nil {
		return nil, err
	}

	m := &Member{
		MemberID:  mid,
		TenantID:  tenantID,
		Email:     cleanEmail,
		Role:      role,
		Status:    MemberStatusActive,
		CreatedAt: nowUnix,
		UpdatedAt: nowUnix,
	}
	p.recordAudit(ctx, tenantID, actorMemberID, "member.added", "Member", mid, nil, map[string]interface{}{"email": cleanEmail, "role": string(role)}, "127.0.0.1")
	return m, nil
}

func (p *PostgresService) GetMember(ctx context.Context, tenantID, memberID string) (*Member, error) {
	return p.getMember(ctx, tenantID, memberID)
}

func (p *PostgresService) ListMembers(ctx context.Context, tenantID string) ([]*Member, error) {
	query := `
		SELECT member_id, tenant_id, email, password_hash, role, status,
		       EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_member
		WHERE tenant_id = $1
		ORDER BY created_at ASC;
	`
	rows, err := p.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]*Member, 0)
	for rows.Next() {
		var m Member
		var pwd sql.NullString
		if err := rows.Scan(&m.MemberID, &m.TenantID, &m.Email, &pwd, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if pwd.Valid {
			m.PasswordHash = pwd.String
		}
		members = append(members, &m)
	}
	return members, nil
}

func (p *PostgresService) countActiveOwners(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := p.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM ap_member WHERE tenant_id = $1 AND role = 'owner' AND status = 'active';
	`, tenantID).Scan(&count)
	return count, err
}

func (p *PostgresService) UpdateMember(ctx context.Context, tenantID, memberID string, role *MemberRole, status *MemberStatus, actorMemberID string) (*Member, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionMemberUpdate); err != nil {
		return nil, err
	}

	m, err := p.getMember(ctx, tenantID, memberID)
	if err != nil {
		return nil, err
	}

	// Last Owner protection
	if m.Role == RoleOwner && m.Status == MemberStatusActive {
		activeOwners, err := p.countActiveOwners(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		if activeOwners <= 1 {
			if role != nil && *role != RoleOwner {
				return nil, ErrCannotDemoteLastOwner
			}
			if status != nil && *status == MemberStatusDisabled {
				return nil, ErrCannotRemoveLastOwner
			}
		}
	}

	before := map[string]interface{}{"role": string(m.Role), "status": string(m.Status)}
	if role != nil {
		m.Role = *role
	}
	if status != nil {
		m.Status = *status
	}

	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_member
		SET role = $1, status = $2, updated_at = NOW()
		WHERE member_id = $3;
	`, string(m.Role), string(m.Status), memberID)
	if err != nil {
		return nil, err
	}

	m.UpdatedAt = time.Now().Unix()
	p.recordAudit(ctx, tenantID, actorMemberID, "member.updated", "Member", memberID, before, map[string]interface{}{"role": string(m.Role), "status": string(m.Status)}, "127.0.0.1")
	return m, nil
}

func (p *PostgresService) RemoveMember(ctx context.Context, tenantID, memberID string, actorMemberID string) error {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionMemberDelete); err != nil {
		return err
	}

	m, err := p.getMember(ctx, tenantID, memberID)
	if err != nil {
		return err
	}

	if m.Role == RoleOwner && m.Status == MemberStatusActive {
		activeOwners, err := p.countActiveOwners(ctx, tenantID)
		if err != nil {
			return err
		}
		if activeOwners <= 1 {
			return ErrCannotRemoveLastOwner
		}
	}

	_, err = p.db.ExecContext(ctx, `DELETE FROM ap_member WHERE member_id = $1;`, memberID)
	if err != nil {
		return err
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "member.removed", "Member", memberID, map[string]interface{}{"email": m.Email}, nil, "127.0.0.1")
	return nil
}

// ----------------------------------------------------------------------------
// Agent & Kill Switch
// ----------------------------------------------------------------------------

func (p *PostgresService) CreateAgent(ctx context.Context, tenantID, projectID, name string, policyCfg map[string]interface{}, actorMemberID string) (*Agent, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAgentCreate); err != nil {
		return nil, err
	}

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	var exists bool
	_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_agent WHERE project_id = $1 AND name = $2);`, projectID, cleanName).Scan(&exists)
	if exists {
		return nil, ErrAgentAlreadyExists
	}

	agID := GenerateAgentID()
	now := time.Now()
	nowUnix := now.Unix()

	if policyCfg == nil {
		policyCfg = make(map[string]interface{})
	}
	cfgBytes, _ := json.Marshal(policyCfg)

	_, err := p.db.ExecContext(ctx, `
		INSERT INTO ap_agent (agent_id, tenant_id, project_id, name, status, policy_config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', $5, $6, $6);
	`, agID, tenantID, projectID, cleanName, cfgBytes, now)
	if err != nil {
		return nil, err
	}

	ag := &Agent{
		AgentID:      agID,
		TenantID:     tenantID,
		ProjectID:    projectID,
		Name:         cleanName,
		Status:       AgentStatusActive,
		PolicyConfig: policyCfg,
		CreatedAt:    nowUnix,
		UpdatedAt:    nowUnix,
	}

	if p.agentReg != nil {
		p.agentReg.RegisterAgent(&agent.AgentIdentity{
			AgentID:   agID,
			TenantID:  tenantID,
			ProjectID: projectID,
			Name:      cleanName,
			Status:    agent.AgentStatusActive,
		}, nil)
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "agent.created", "Agent", agID, nil, map[string]interface{}{"name": cleanName}, "127.0.0.1")
	return ag, nil
}

func (p *PostgresService) GetAgent(ctx context.Context, tenantID, agentID string) (*Agent, error) {
	query := `
		SELECT agent_id, tenant_id, project_id, name, status, policy_config,
		       EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_agent
		WHERE agent_id = $1;
	`
	var ag Agent
	var cfgBytes []byte
	err := p.db.QueryRowContext(ctx, query, agentID).Scan(
		&ag.AgentID, &ag.TenantID, &ag.ProjectID, &ag.Name, &ag.Status, &cfgBytes, &ag.CreatedAt, &ag.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	if tenantID != "" && ag.TenantID != tenantID {
		return nil, ErrAgentTenantMismatch
	}
	_ = json.Unmarshal(cfgBytes, &ag.PolicyConfig)
	return &ag, nil
}

func (p *PostgresService) ListAgents(ctx context.Context, tenantID, projectID string, status *AgentStatus) ([]*Agent, error) {
	var sb strings.Builder
	sb.WriteString(`
		SELECT agent_id, tenant_id, project_id, name, status, policy_config,
		       EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_agent
		WHERE 1=1
	`)
	args := make([]interface{}, 0)
	idx := 1

	if tenantID != "" {
		sb.WriteString(fmt.Sprintf(" AND tenant_id = $%d", idx))
		args = append(args, tenantID)
		idx++
	}
	if projectID != "" {
		sb.WriteString(fmt.Sprintf(" AND project_id = $%d", idx))
		args = append(args, projectID)
		idx++
	}
	if status != nil {
		sb.WriteString(fmt.Sprintf(" AND status = $%d", idx))
		args = append(args, string(*status))
		idx++
	}
	sb.WriteString(" ORDER BY created_at DESC;")

	rows, err := p.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agentsList := make([]*Agent, 0)
	for rows.Next() {
		var ag Agent
		var cfgBytes []byte
		if err := rows.Scan(&ag.AgentID, &ag.TenantID, &ag.ProjectID, &ag.Name, &ag.Status, &cfgBytes, &ag.CreatedAt, &ag.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(cfgBytes, &ag.PolicyConfig)
		agentsList = append(agentsList, &ag)
	}
	return agentsList, nil
}

func (p *PostgresService) UpdateAgent(ctx context.Context, tenantID, agentID string, name string, policyCfg map[string]interface{}, actorMemberID string) (*Agent, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAgentUpdate); err != nil {
		return nil, err
	}

	ag, err := p.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	if ag.Status == AgentStatusRevoked {
		return nil, ErrAgentRevoked
	}

	before := map[string]interface{}{"name": ag.Name, "status": string(ag.Status)}
	if name != "" {
		ag.Name = strings.TrimSpace(name)
	}
	if policyCfg != nil {
		ag.PolicyConfig = policyCfg
	}

	cfgBytes, _ := json.Marshal(ag.PolicyConfig)
	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_agent
		SET name = $1, policy_config = $2, updated_at = NOW()
		WHERE agent_id = $3;
	`, ag.Name, cfgBytes, agentID)
	if err != nil {
		return nil, err
	}

	ag.UpdatedAt = time.Now().Unix()
	if p.agentReg != nil {
		p.agentReg.RegisterAgent(&agent.AgentIdentity{
			AgentID:   ag.AgentID,
			TenantID:  ag.TenantID,
			ProjectID: ag.ProjectID,
			Name:      ag.Name,
			Status:    agent.AgentStatus(ag.Status),
		}, nil)
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "agent.updated", "Agent", agentID, before, map[string]interface{}{"name": ag.Name}, "127.0.0.1")
	return ag, nil
}

func (p *PostgresService) UpdateAgentStatus(ctx context.Context, tenantID, agentID string, status AgentStatus, actorMemberID string) (*Agent, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAgentUpdate); err != nil {
		return nil, err
	}

	ag, err := p.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	if ag.Status == AgentStatusRevoked {
		return nil, ErrAgentRevoked
	}

	before := map[string]interface{}{"status": string(ag.Status)}
	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_agent
		SET status = $1, updated_at = NOW()
		WHERE agent_id = $2;
	`, string(status), agentID)
	if err != nil {
		return nil, err
	}

	ag.Status = status
	ag.UpdatedAt = time.Now().Unix()

	if p.agentReg != nil {
		p.agentReg.SetAgentStatus(agentID, agent.AgentStatus(status))
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "agent.status_updated", "Agent", agentID, before, map[string]interface{}{"status": string(status)}, "127.0.0.1")
	return ag, nil
}

func (p *PostgresService) KillAgent(ctx context.Context, tenantID, agentID, reason string, actorMemberID string, revoke bool, clientIP string) (*Agent, []string, error) {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return nil, nil, ErrMissingReason
	}

	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAgentKill); err != nil {
		return nil, nil, err
	}

	ag, err := p.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, nil, err
	}

	// 1. Instantaneous in-memory fast-path kill
	if p.agentReg != nil {
		p.agentReg.SetAgentStatus(agentID, agent.AgentStatusSuspended)
	}

	// 2. Persist suspended status
	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_agent
		SET status = 'suspended', updated_at = NOW()
		WHERE agent_id = $1;
	`, agentID)
	if err != nil {
		return nil, nil, err
	}
	ag.Status = AgentStatusSuspended
	ag.UpdatedAt = time.Now().Unix()

	revokedKeys := make([]string, 0)
	if revoke {
		rows, err := p.db.QueryContext(ctx, `
			SELECT key_id, key_hash
			FROM ap_api_key
			WHERE agent_id = $1 AND status = 'active';
		`, agentID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var kID, kHash string
				if err := rows.Scan(&kID, &kHash); err == nil {
					revokedKeys = append(revokedKeys, kID)
					if p.agentReg != nil {
						p.agentReg.RevokeKey(kHash)
					}
				}
			}
			_, _ = p.db.ExecContext(ctx, `
				UPDATE ap_api_key
				SET status = 'revoked', updated_at = NOW()
				WHERE agent_id = $1 AND status = 'active';
			`, agentID)
		}
	}

	p.recordAudit(ctx, ag.TenantID, actorMemberID, "agent.kill", "Agent", agentID, nil, map[string]interface{}{
		"reason":       cleanReason,
		"revoked_keys": revokedKeys,
	}, clientIP)

	return ag, revokedKeys, nil
}

func (p *PostgresService) DeleteAgent(ctx context.Context, tenantID, agentID, actorMemberID string) error {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAgentKill); err != nil {
		return err
	}

	ag, err := p.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return err
	}

	if p.agentReg != nil {
		p.agentReg.SetAgentStatus(agentID, agent.AgentStatusRevoked)
	}

	_, err = p.db.ExecContext(ctx, `DELETE FROM ap_agent WHERE agent_id = $1;`, agentID)
	if err != nil {
		return err
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "agent.deleted", "Agent", agentID, map[string]interface{}{"name": ag.Name}, nil, "127.0.0.1")
	return nil
}

// ----------------------------------------------------------------------------
// API Keys
// ----------------------------------------------------------------------------

func (p *PostgresService) CreateAPIKey(ctx context.Context, tenantID, projectID, agentID, name string, scopes []string, expiresAt *int64, actorMemberID string) (*CreateAPIKeyResult, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAPIKeyCreate); err != nil {
		return nil, err
	}

	ag, err := p.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	if ag.Status == AgentStatusRevoked {
		return nil, ErrAgentRevoked
	}

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("api key name cannot be empty")
	}

	rawKey, keyHash, keyPrefix := GenerateRawAPIKey()
	keyID := GenerateAPIKeyID()
	now := time.Now()
	nowUnix := now.Unix()

	if len(scopes) == 0 {
		scopes = []string{"gateway:call"}
	}
	scopesBytes, _ := json.Marshal(scopes)

	var expTime *time.Time
	if expiresAt != nil {
		t := time.Unix(*expiresAt, 0)
		expTime = &t
	}

	_, err = p.db.ExecContext(ctx, `
		INSERT INTO ap_api_key (key_id, tenant_id, project_id, agent_id, name, key_hash, key_prefix, scopes, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active', $9, $10, $10);
	`, keyID, tenantID, projectID, agentID, cleanName, keyHash, keyPrefix, scopesBytes, expTime, now)
	if err != nil {
		return nil, err
	}

	if p.agentReg != nil {
		p.agentReg.BindAPIKeyHash(keyHash, agentID)
	}

	apiKey := &APIKey{
		KeyID:     keyID,
		TenantID:  tenantID,
		ProjectID: projectID,
		AgentID:   agentID,
		Name:      cleanName,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Scopes:    scopes,
		Status:    APIKeyStatusActive,
		ExpiresAt: expiresAt,
		CreatedAt: nowUnix,
		UpdatedAt: nowUnix,
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "apikey.created", "APIKey", keyID, nil, map[string]interface{}{"agent_id": agentID, "name": cleanName}, "127.0.0.1")
	return &CreateAPIKeyResult{
		APIKey: apiKey,
		RawKey: rawKey,
	}, nil
}

func (p *PostgresService) GetAPIKey(ctx context.Context, tenantID, keyID string) (*APIKey, error) {
	query := `
		SELECT key_id, tenant_id, project_id, agent_id, name, key_hash, key_prefix, scopes, status,
		       EXTRACT(EPOCH FROM expires_at)::bigint, EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_api_key
		WHERE key_id = $1;
	`
	var k APIKey
	var scopesBytes []byte
	var exp sql.NullInt64
	err := p.db.QueryRowContext(ctx, query, keyID).Scan(
		&k.KeyID, &k.TenantID, &k.ProjectID, &k.AgentID, &k.Name, &k.KeyHash, &k.KeyPrefix, &scopesBytes, &k.Status,
		&exp, &k.CreatedAt, &k.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	if tenantID != "" && k.TenantID != tenantID {
		return nil, ErrAPIKeyNotFound
	}
	if exp.Valid {
		v := exp.Int64
		k.ExpiresAt = &v
	}
	_ = json.Unmarshal(scopesBytes, &k.Scopes)
	return &k, nil
}

func (p *PostgresService) ListAPIKeys(ctx context.Context, tenantID, agentID string) ([]*APIKey, error) {
	var sb strings.Builder
	sb.WriteString(`
		SELECT key_id, tenant_id, project_id, agent_id, name, key_hash, key_prefix, scopes, status,
		       EXTRACT(EPOCH FROM expires_at)::bigint, EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_api_key
		WHERE 1=1
	`)
	args := make([]interface{}, 0)
	idx := 1
	if tenantID != "" {
		sb.WriteString(fmt.Sprintf(" AND tenant_id = $%d", idx))
		args = append(args, tenantID)
		idx++
	}
	if agentID != "" {
		sb.WriteString(fmt.Sprintf(" AND agent_id = $%d", idx))
		args = append(args, agentID)
		idx++
	}
	sb.WriteString(" ORDER BY created_at DESC;")

	rows, err := p.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]*APIKey, 0)
	for rows.Next() {
		var k APIKey
		var scopesBytes []byte
		var exp sql.NullInt64
		if err := rows.Scan(&k.KeyID, &k.TenantID, &k.ProjectID, &k.AgentID, &k.Name, &k.KeyHash, &k.KeyPrefix, &scopesBytes, &k.Status, &exp, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, err
		}
		if exp.Valid {
			v := exp.Int64
			k.ExpiresAt = &v
		}
		_ = json.Unmarshal(scopesBytes, &k.Scopes)
		keys = append(keys, &k)
	}
	return keys, nil
}

func (p *PostgresService) RevokeAPIKey(ctx context.Context, tenantID, keyID, actorMemberID string) (*APIKey, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAPIKeyRevoke); err != nil {
		return nil, err
	}

	k, err := p.GetAPIKey(ctx, tenantID, keyID)
	if err != nil {
		return nil, err
	}
	if k.Status == APIKeyStatusRevoked {
		return k, nil
	}

	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_api_key
		SET status = 'revoked', updated_at = NOW()
		WHERE key_id = $1;
	`, keyID)
	if err != nil {
		return nil, err
	}

	if p.agentReg != nil {
		p.agentReg.RevokeKey(k.KeyHash)
	}

	k.Status = APIKeyStatusRevoked
	k.UpdatedAt = time.Now().Unix()
	p.recordAudit(ctx, tenantID, actorMemberID, "apikey.revoked", "APIKey", keyID, nil, map[string]interface{}{"status": "revoked"}, "127.0.0.1")
	return k, nil
}

func (p *PostgresService) RotateAPIKey(ctx context.Context, tenantID, keyID, actorMemberID string) (*CreateAPIKeyResult, error) {
	oldKey, err := p.RevokeAPIKey(ctx, tenantID, keyID, actorMemberID)
	if err != nil {
		return nil, err
	}
	return p.CreateAPIKey(ctx, tenantID, oldKey.ProjectID, oldKey.AgentID, oldKey.Name, oldKey.Scopes, oldKey.ExpiresAt, actorMemberID)
}

// ----------------------------------------------------------------------------
// Routes
// ----------------------------------------------------------------------------

func (p *PostgresService) CreateRoute(ctx context.Context, route *Route, actorMemberID string) (*Route, error) {
	if route == nil {
		return nil, fmt.Errorf("route cannot be nil")
	}
	if err := p.enforcePermission(ctx, actorMemberID, route.TenantID, PermissionRouteWrite); err != nil {
		return nil, err
	}

	cleanPath := strings.TrimSpace(route.PathPattern)
	if cleanPath == "" || !strings.HasPrefix(cleanPath, "/") {
		return nil, fmt.Errorf("path_pattern must begin with '/'")
	}

	var exists bool
	_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_route WHERE tenant_id = $1 AND path_pattern = $2);`, route.TenantID, cleanPath).Scan(&exists)
	if exists {
		return nil, ErrRouteAlreadyExists
	}

	if route.RouteID == "" {
		route.RouteID = GenerateRouteID()
	}
	route.Slug = NormalizeSlug(route.Slug, cleanPath)
	route.Title = DefaultTitleFromSlug(route.Title, route.Slug, cleanPath)
	if strings.TrimSpace(route.Category) == "" {
		route.Category = "AI & LLM"
	} else {
		route.Category = strings.TrimSpace(route.Category)
	}
	if strings.TrimSpace(route.Network) == "" {
		route.Network = "eip155:421614"
	} else {
		route.Network = strings.TrimSpace(route.Network)
	}
	if route.Tags == nil {
		route.Tags = []string{}
	}
	if route.AllowedPaymentMethods == "" {
		route.AllowedPaymentMethods = "both"
	}
	if route.Asset == "" {
		if route.AllowedPaymentMethods == "onchain" {
			route.Asset = "USDC"
		} else if t, err := p.GetTenant(ctx, route.TenantID); err == nil && t.Currency != "" && (route.AllowedPaymentMethods == "wallet" || t.Currency != "USD") {
			route.Asset = t.Currency
		} else {
			route.Asset = "USDC"
		}
	}
	if route.OnchainAsset == "" && route.AllowedPaymentMethods != "wallet" {
		route.OnchainAsset = "USDC"
	}
	if route.TimeoutMs == 0 {
		route.TimeoutMs = 30000
	}
	if route.RateLimitRPM == 0 {
		route.RateLimitRPM = 600
	}
	if route.Status == "" {
		route.Status = RouteStatusActive
	}

	now := time.Now()
	nowUnix := now.Unix()
	route.CreatedAt = nowUnix
	route.UpdatedAt = nowUnix

	var onchainPrice interface{}
	if route.OnchainPriceBaseUnits != nil {
		onchainPrice = *route.OnchainPriceBaseUnits
	}
	onchainAsset := route.OnchainAsset
	if onchainAsset == "" {
		onchainAsset = "USDC"
	}
	tagsJSON, _ := json.Marshal(route.Tags)

	_, err := p.db.ExecContext(ctx, `
		INSERT INTO ap_route (
			route_id, tenant_id, project_id, slug, title, is_public, category, tags, description_md, network,
			path_pattern, upstream_url, price_base_units, asset, onchain_price_base_units, onchain_asset,
			payee_address, allowed_payment_methods, timeout_ms, rate_limit_rpm, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $22);
	`, route.RouteID, route.TenantID, route.ProjectID, route.Slug, route.Title, route.IsPublic, route.Category, tagsJSON, route.DescriptionMD, route.Network,
		cleanPath, route.UpstreamURL, route.PriceBaseUnits, route.Asset, onchainPrice, onchainAsset,
		route.PayeeAddress, route.AllowedPaymentMethods, route.TimeoutMs, route.RateLimitRPM, string(route.Status), now)
	if err != nil {
		return nil, err
	}

	if route.UpstreamAuth != nil {
		if route.UpstreamAuth.Type == "" {
			route.UpstreamAuth.Type = "none"
		}
		if auth, ok := upstreamauth.Get(route.UpstreamAuth.Type); ok {
			if err := auth.Validate(route.UpstreamAuth.Params, route.UpstreamAuth.Secrets); err != nil {
				return nil, fmt.Errorf("upstream_auth validation error: %w", err)
			}
		}
		if len(route.UpstreamAuth.Secrets) > 0 {
			enc, err := upstreamauth.EncryptSecrets(route.UpstreamAuth.Secrets, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt upstream secrets: %w", err)
			}
			route.UpstreamAuth.SecretEnc = enc
			route.UpstreamAuth.HasSecret = true
			route.UpstreamAuth.KeyVersion = 1
			route.UpstreamAuth.Secrets = nil
		}
		route.UpstreamAuth.UpdatedAt = nowUnix

		paramsJSON, _ := json.Marshal(route.UpstreamAuth.Params)
		_, err = p.db.ExecContext(ctx, `
			INSERT INTO api_auth_configs (id, tenant_id, api_id, type, params, secret_enc, key_version, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (api_id) DO UPDATE SET
				type = EXCLUDED.type,
				params = EXCLUDED.params,
				secret_enc = COALESCE(EXCLUDED.secret_enc, api_auth_configs.secret_enc),
				key_version = EXCLUDED.key_version,
				updated_at = EXCLUDED.updated_at;
		`, "auth_"+route.RouteID, route.TenantID, route.RouteID, route.UpstreamAuth.Type, paramsJSON, route.UpstreamAuth.SecretEnc, route.UpstreamAuth.KeyVersion, now)
		if err != nil {
			return nil, fmt.Errorf("failed to save upstream_auth: %w", err)
		}
	}

	p.recordAudit(ctx, route.TenantID, actorMemberID, "route.created", "Route", route.RouteID, nil, map[string]interface{}{"path": cleanPath, "slug": route.Slug, "is_public": route.IsPublic}, "127.0.0.1")
	_ = p.SyncRCU(ctx)
	return route, nil
}

func (p *PostgresService) GetRoute(ctx context.Context, tenantID, routeID string) (*Route, error) {
	query := `
		SELECT r.route_id, r.tenant_id, r.project_id, COALESCE(r.slug, ''), COALESCE(r.title, ''), r.is_public,
		       COALESCE(r.category, 'General'), COALESCE(r.tags, '[]'::jsonb), COALESCE(r.description_md, ''), COALESCE(r.network, 'eip155:421614'),
		       r.path_pattern, r.upstream_url, r.price_base_units, r.asset,
		       r.onchain_price_base_units, r.onchain_asset, r.payee_address, r.allowed_payment_methods, r.timeout_ms, r.rate_limit_rpm, r.status,
		       EXTRACT(EPOCH FROM r.created_at)::bigint, EXTRACT(EPOCH FROM r.updated_at)::bigint,
		       c.type, c.params, c.secret_enc, c.key_version, EXTRACT(EPOCH FROM c.updated_at)::bigint
		FROM ap_route r
		LEFT JOIN api_auth_configs c ON c.api_id = r.route_id
		WHERE r.route_id = $1;
	`
	var r Route
	var tagsBytes []byte
	var onchainPrice sql.NullInt64
	var onchainAsset sql.NullString
	var cType sql.NullString
	var cParamsBytes, cSecretEnc []byte
	var cKeyVersion, cUpdatedAt sql.NullInt64

	err := p.db.QueryRowContext(ctx, query, routeID).Scan(
		&r.RouteID, &r.TenantID, &r.ProjectID, &r.Slug, &r.Title, &r.IsPublic,
		&r.Category, &tagsBytes, &r.DescriptionMD, &r.Network,
		&r.PathPattern, &r.UpstreamURL, &r.PriceBaseUnits, &r.Asset,
		&onchainPrice, &onchainAsset, &r.PayeeAddress, &r.AllowedPaymentMethods, &r.TimeoutMs, &r.RateLimitRPM, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		&cType, &cParamsBytes, &cSecretEnc, &cKeyVersion, &cUpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRouteNotFound
	}
	if err != nil {
		return nil, err
	}
	if tenantID != "" && r.TenantID != tenantID {
		return nil, ErrRouteNotFound
	}
	if len(tagsBytes) > 0 {
		_ = json.Unmarshal(tagsBytes, &r.Tags)
	}
	if r.Tags == nil {
		r.Tags = []string{}
	}
	r.Slug = NormalizeSlug(r.Slug, r.PathPattern)
	r.Title = DefaultTitleFromSlug(r.Title, r.Slug, r.PathPattern)
	if onchainPrice.Valid {
		v := onchainPrice.Int64
		r.OnchainPriceBaseUnits = &v
	}
	if onchainAsset.Valid {
		r.OnchainAsset = onchainAsset.String
	}

	if cType.Valid && cType.String != "" {
		var params map[string]interface{}
		if len(cParamsBytes) > 0 {
			_ = json.Unmarshal(cParamsBytes, &params)
		}
		r.UpstreamAuth = &UpstreamAuthConfig{
			Type:       cType.String,
			Params:     params,
			SecretEnc:  cSecretEnc,
			HasSecret:  len(cSecretEnc) > 0,
			KeyVersion: int(cKeyVersion.Int64),
			UpdatedAt:  cUpdatedAt.Int64,
		}
	}
	return &r, nil
}

func (p *PostgresService) ListRoutes(ctx context.Context, tenantID string) ([]*Route, error) {
	query := `
		SELECT r.route_id, r.tenant_id, r.project_id, COALESCE(r.slug, ''), COALESCE(r.title, ''), r.is_public,
		       COALESCE(r.category, 'General'), COALESCE(r.tags, '[]'::jsonb), COALESCE(r.description_md, ''), COALESCE(r.network, 'eip155:421614'),
		       r.path_pattern, r.upstream_url, r.price_base_units, r.asset,
		       r.onchain_price_base_units, r.onchain_asset, r.payee_address, r.allowed_payment_methods, r.timeout_ms, r.rate_limit_rpm, r.status,
		       EXTRACT(EPOCH FROM r.created_at)::bigint, EXTRACT(EPOCH FROM r.updated_at)::bigint,
		       c.type, c.params, c.secret_enc, c.key_version, EXTRACT(EPOCH FROM c.updated_at)::bigint
		FROM ap_route r
		LEFT JOIN api_auth_configs c ON c.api_id = r.route_id
		WHERE r.tenant_id = $1
		ORDER BY r.created_at DESC;
	`
	rows, err := p.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make([]*Route, 0)
	for rows.Next() {
		var r Route
		var tagsBytes []byte
		var onchainPrice sql.NullInt64
		var onchainAsset sql.NullString
		var cType sql.NullString
		var cParamsBytes, cSecretEnc []byte
		var cKeyVersion, cUpdatedAt sql.NullInt64

		if err := rows.Scan(
			&r.RouteID, &r.TenantID, &r.ProjectID, &r.Slug, &r.Title, &r.IsPublic,
			&r.Category, &tagsBytes, &r.DescriptionMD, &r.Network,
			&r.PathPattern, &r.UpstreamURL, &r.PriceBaseUnits, &r.Asset,
			&onchainPrice, &onchainAsset, &r.PayeeAddress, &r.AllowedPaymentMethods, &r.TimeoutMs, &r.RateLimitRPM, &r.Status, &r.CreatedAt, &r.UpdatedAt,
			&cType, &cParamsBytes, &cSecretEnc, &cKeyVersion, &cUpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(tagsBytes) > 0 {
			_ = json.Unmarshal(tagsBytes, &r.Tags)
		}
		if r.Tags == nil {
			r.Tags = []string{}
		}
		r.Slug = NormalizeSlug(r.Slug, r.PathPattern)
		r.Title = DefaultTitleFromSlug(r.Title, r.Slug, r.PathPattern)
		if onchainPrice.Valid {
			v := onchainPrice.Int64
			r.OnchainPriceBaseUnits = &v
		}
		if onchainAsset.Valid {
			r.OnchainAsset = onchainAsset.String
		}
		if cType.Valid && cType.String != "" {
			var params map[string]interface{}
			if len(cParamsBytes) > 0 {
				_ = json.Unmarshal(cParamsBytes, &params)
			}
			r.UpstreamAuth = &UpstreamAuthConfig{
				Type:       cType.String,
				Params:     params,
				SecretEnc:  cSecretEnc,
				HasSecret:  len(cSecretEnc) > 0,
				KeyVersion: int(cKeyVersion.Int64),
				UpdatedAt:  cUpdatedAt.Int64,
			}
		}
		routes = append(routes, &r)
	}
	return routes, nil
}

func (p *PostgresService) UpdateRoute(ctx context.Context, input *UpdateRouteInput, actorMemberID string) (*Route, error) {
	if input == nil {
		return nil, fmt.Errorf("update input cannot be nil")
	}
	if err := p.enforcePermission(ctx, actorMemberID, input.TenantID, PermissionRouteWrite); err != nil {
		return nil, err
	}

	r, err := p.GetRoute(ctx, input.TenantID, input.RouteID)
	if err != nil {
		return nil, err
	}

	before := map[string]interface{}{"path": r.PathPattern, "slug": r.Slug, "is_public": r.IsPublic, "upstream": r.UpstreamURL, "price": r.PriceBaseUnits}

	if input.PathPattern != nil {
		clean := strings.TrimSpace(*input.PathPattern)
		var exists bool
		_ = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ap_route WHERE tenant_id = $1 AND path_pattern = $2 AND route_id != $3);`, input.TenantID, clean, input.RouteID).Scan(&exists)
		if exists {
			return nil, ErrRouteAlreadyExists
		}
		r.PathPattern = clean
	}
	if input.Slug != nil {
		r.Slug = NormalizeSlug(*input.Slug, r.PathPattern)
	} else if r.Slug == "" {
		r.Slug = NormalizeSlug("", r.PathPattern)
	}
	if input.Title != nil {
		r.Title = DefaultTitleFromSlug(*input.Title, r.Slug, r.PathPattern)
	}
	if input.IsPublic != nil {
		r.IsPublic = *input.IsPublic
	}
	if input.Category != nil {
		r.Category = strings.TrimSpace(*input.Category)
	}
	if input.Tags != nil {
		r.Tags = input.Tags
	}
	if input.DescriptionMD != nil {
		r.DescriptionMD = *input.DescriptionMD
	}
	if input.Network != nil && strings.TrimSpace(*input.Network) != "" {
		r.Network = strings.TrimSpace(*input.Network)
	}
	if input.UpstreamURL != nil {
		r.UpstreamURL = *input.UpstreamURL
	}
	if input.PriceBaseUnits != nil {
		r.PriceBaseUnits = *input.PriceBaseUnits
	}
	if input.Asset != nil {
		r.Asset = *input.Asset
	}
	if input.OnchainPriceBaseUnits != nil {
		v := *input.OnchainPriceBaseUnits
		r.OnchainPriceBaseUnits = &v
	}
	if input.OnchainAsset != nil {
		r.OnchainAsset = *input.OnchainAsset
	}
	if input.PayeeAddress != nil {
		r.PayeeAddress = *input.PayeeAddress
	}
	if input.AllowedPaymentMethods != nil {
		r.AllowedPaymentMethods = *input.AllowedPaymentMethods
	}
	if input.RateLimitRPM != nil {
		r.RateLimitRPM = *input.RateLimitRPM
	}
	if input.TimeoutMs != nil {
		r.TimeoutMs = *input.TimeoutMs
	}
	if input.Status != nil {
		r.Status = *input.Status
	}

	var onchainPrice interface{}
	if r.OnchainPriceBaseUnits != nil {
		onchainPrice = *r.OnchainPriceBaseUnits
	}
	onchainAsset := r.OnchainAsset
	if onchainAsset == "" {
		onchainAsset = "USDC"
	}
	tagsJSON, _ := json.Marshal(r.Tags)

	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_route
		SET path_pattern = $1, upstream_url = $2, price_base_units = $3, asset = $4, onchain_price_base_units = $5, onchain_asset = $6,
		    payee_address = $7, allowed_payment_methods = $8, timeout_ms = $9, rate_limit_rpm = $10, status = $11,
		    slug = $12, title = $13, is_public = $14, category = $15, tags = $16, description_md = $17, network = $18,
		    updated_at = NOW()
		WHERE route_id = $19;
	`, r.PathPattern, r.UpstreamURL, r.PriceBaseUnits, r.Asset, onchainPrice, onchainAsset,
		r.PayeeAddress, r.AllowedPaymentMethods, r.TimeoutMs, r.RateLimitRPM, string(r.Status),
		r.Slug, r.Title, r.IsPublic, r.Category, tagsJSON, r.DescriptionMD, r.Network, input.RouteID)
	if err != nil {
		return nil, err
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
		nowUnix := time.Now().Unix()
		if len(authCfg.Secrets) > 0 {
			enc, err := upstreamauth.EncryptSecrets(authCfg.Secrets, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt upstream secrets: %w", err)
			}
			authCfg.SecretEnc = enc
			authCfg.HasSecret = true
			if r.UpstreamAuth != nil {
				authCfg.KeyVersion = r.UpstreamAuth.KeyVersion + 1
			} else {
				authCfg.KeyVersion = 1
			}
			authCfg.Secrets = nil
		} else if r.UpstreamAuth != nil && r.UpstreamAuth.Type == authCfg.Type {
			authCfg.SecretEnc = r.UpstreamAuth.SecretEnc
			authCfg.HasSecret = r.UpstreamAuth.HasSecret
			authCfg.KeyVersion = r.UpstreamAuth.KeyVersion
		}
		authCfg.UpdatedAt = nowUnix

		paramsJSON, _ := json.Marshal(authCfg.Params)
		_, err = p.db.ExecContext(ctx, `
			INSERT INTO api_auth_configs (id, tenant_id, api_id, type, params, secret_enc, key_version, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			ON CONFLICT (api_id) DO UPDATE SET
				type = EXCLUDED.type,
				params = EXCLUDED.params,
				secret_enc = COALESCE(EXCLUDED.secret_enc, api_auth_configs.secret_enc),
				key_version = EXCLUDED.key_version,
				updated_at = NOW();
		`, "auth_"+input.RouteID, input.TenantID, input.RouteID, authCfg.Type, paramsJSON, authCfg.SecretEnc, authCfg.KeyVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to update upstream_auth: %w", err)
		}
		r.UpstreamAuth = authCfg
	}

	r.UpdatedAt = time.Now().Unix()
	p.recordAudit(ctx, input.TenantID, actorMemberID, "route.updated", "Route", input.RouteID, before, map[string]interface{}{"path": r.PathPattern}, "127.0.0.1")
	_ = p.SyncRCU(ctx)
	return r, nil
}

func (p *PostgresService) DeleteRoute(ctx context.Context, tenantID, routeID, actorMemberID string) error {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionRouteWrite); err != nil {
		return err
	}

	r, err := p.GetRoute(ctx, tenantID, routeID)
	if err != nil {
		return err
	}

	_, _ = p.db.ExecContext(ctx, `DELETE FROM api_auth_configs WHERE api_id = $1;`, routeID)
	_, err = p.db.ExecContext(ctx, `DELETE FROM ap_route WHERE route_id = $1;`, routeID)
	if err != nil {
		return err
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "route.deleted", "Route", routeID, map[string]interface{}{"path": r.PathPattern}, nil, "127.0.0.1")
	_ = p.SyncRCU(ctx)
	return nil
}

// ----------------------------------------------------------------------------
// Policies
// ----------------------------------------------------------------------------

func (p *PostgresService) CreatePolicy(ctx context.Context, pol *Policy, actorMemberID string) (*Policy, error) {
	if pol == nil {
		return nil, fmt.Errorf("policy cannot be nil")
	}
	if err := p.enforcePermission(ctx, actorMemberID, pol.TenantID, PermissionPolicyWrite); err != nil {
		return nil, err
	}

	if pol.PolicyID == "" {
		pol.PolicyID = GeneratePolicyID()
	}
	if pol.WhitelistedPayees == nil {
		pol.WhitelistedPayees = []string{}
	}
	if pol.WhitelistedNetworks == nil {
		pol.WhitelistedNetworks = []string{"eip155:42161"}
	}
	if pol.WhitelistedRoutes == nil {
		pol.WhitelistedRoutes = []string{}
	}

	payeesBytes, _ := json.Marshal(pol.WhitelistedPayees)
	networksBytes, _ := json.Marshal(pol.WhitelistedNetworks)
	routesBytes, _ := json.Marshal(pol.WhitelistedRoutes)

	var agID, rteID sql.NullString
	if pol.AgentID != "" {
		agID = sql.NullString{String: pol.AgentID, Valid: true}
	}
	if pol.RouteID != "" {
		rteID = sql.NullString{String: pol.RouteID, Valid: true}
	}

	now := time.Now()
	nowUnix := now.Unix()
	pol.CreatedAt = nowUnix
	pol.UpdatedAt = nowUnix
	pol.IsActive = true

	_, err := p.db.ExecContext(ctx, `
		INSERT INTO ap_policy (policy_id, tenant_id, project_id, agent_id, route_id, max_amount_per_request, rolling_24h_budget_cap, circuit_breaker_max_rpm, whitelisted_payees, whitelisted_networks, whitelisted_routes, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13);
	`, pol.PolicyID, pol.TenantID, pol.ProjectID, agID, rteID, pol.MaxAmountPerRequest, pol.Rolling24hBudgetCap, pol.CircuitBreakerMaxRPM, payeesBytes, networksBytes, routesBytes, pol.IsActive, now)
	if err != nil {
		return nil, err
	}

	p.recordAudit(ctx, pol.TenantID, actorMemberID, "policy.created", "Policy", pol.PolicyID, nil, nil, "127.0.0.1")
	_ = p.SyncRCU(ctx)
	return pol, nil
}

func (p *PostgresService) GetPolicy(ctx context.Context, tenantID, policyID string) (*Policy, error) {
	query := `
		SELECT policy_id, tenant_id, project_id, agent_id, route_id, max_amount_per_request, rolling_24h_budget_cap, circuit_breaker_max_rpm, whitelisted_payees, whitelisted_networks, whitelisted_routes, is_active,
		       EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_policy
		WHERE policy_id = $1;
	`
	var pol Policy
	var agID, rteID sql.NullString
	var payeesBytes, networksBytes, routesBytes []byte

	err := p.db.QueryRowContext(ctx, query, policyID).Scan(
		&pol.PolicyID, &pol.TenantID, &pol.ProjectID, &agID, &rteID, &pol.MaxAmountPerRequest, &pol.Rolling24hBudgetCap, &pol.CircuitBreakerMaxRPM, &payeesBytes, &networksBytes, &routesBytes, &pol.IsActive, &pol.CreatedAt, &pol.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPolicyNotFound
	}
	if err != nil {
		return nil, err
	}
	if tenantID != "" && pol.TenantID != tenantID {
		return nil, ErrPolicyNotFound
	}
	if agID.Valid {
		pol.AgentID = agID.String
	}
	if rteID.Valid {
		pol.RouteID = rteID.String
	}
	_ = json.Unmarshal(payeesBytes, &pol.WhitelistedPayees)
	_ = json.Unmarshal(networksBytes, &pol.WhitelistedNetworks)
	_ = json.Unmarshal(routesBytes, &pol.WhitelistedRoutes)
	return &pol, nil
}

func (p *PostgresService) ListPolicies(ctx context.Context, tenantID, projectID, agentID string) ([]*Policy, error) {
	var sb strings.Builder
	sb.WriteString(`
		SELECT policy_id, tenant_id, project_id, agent_id, route_id, max_amount_per_request, rolling_24h_budget_cap, circuit_breaker_max_rpm, whitelisted_payees, whitelisted_networks, whitelisted_routes, is_active,
		       EXTRACT(EPOCH FROM created_at)::bigint, EXTRACT(EPOCH FROM updated_at)::bigint
		FROM ap_policy
		WHERE 1=1
	`)
	args := make([]interface{}, 0)
	idx := 1
	if tenantID != "" {
		sb.WriteString(fmt.Sprintf(" AND tenant_id = $%d", idx))
		args = append(args, tenantID)
		idx++
	}
	if projectID != "" {
		sb.WriteString(fmt.Sprintf(" AND project_id = $%d", idx))
		args = append(args, projectID)
		idx++
	}
	if agentID != "" {
		sb.WriteString(fmt.Sprintf(" AND agent_id = $%d", idx))
		args = append(args, agentID)
		idx++
	}
	sb.WriteString(" ORDER BY created_at DESC;")

	rows, err := p.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := make([]*Policy, 0)
	for rows.Next() {
		var pol Policy
		var agID, rteID sql.NullString
		var payeesBytes, networksBytes, routesBytes []byte

		if err := rows.Scan(&pol.PolicyID, &pol.TenantID, &pol.ProjectID, &agID, &rteID, &pol.MaxAmountPerRequest, &pol.Rolling24hBudgetCap, &pol.CircuitBreakerMaxRPM, &payeesBytes, &networksBytes, &routesBytes, &pol.IsActive, &pol.CreatedAt, &pol.UpdatedAt); err != nil {
			return nil, err
		}
		if agID.Valid {
			pol.AgentID = agID.String
		}
		if rteID.Valid {
			pol.RouteID = rteID.String
		}
		_ = json.Unmarshal(payeesBytes, &pol.WhitelistedPayees)
		_ = json.Unmarshal(networksBytes, &pol.WhitelistedNetworks)
		_ = json.Unmarshal(routesBytes, &pol.WhitelistedRoutes)
		policies = append(policies, &pol)
	}
	return policies, nil
}

func (p *PostgresService) UpdatePolicy(ctx context.Context, pol *Policy, actorMemberID string) (*Policy, error) {
	if pol == nil {
		return nil, fmt.Errorf("policy cannot be nil")
	}
	if err := p.enforcePermission(ctx, actorMemberID, pol.TenantID, PermissionPolicyWrite); err != nil {
		return nil, err
	}

	existing, err := p.GetPolicy(ctx, pol.TenantID, pol.PolicyID)
	if err != nil {
		return nil, err
	}

	payeesBytes, _ := json.Marshal(pol.WhitelistedPayees)
	networksBytes, _ := json.Marshal(pol.WhitelistedNetworks)
	routesBytes, _ := json.Marshal(pol.WhitelistedRoutes)

	var agID, rteID sql.NullString
	if pol.AgentID != "" {
		agID = sql.NullString{String: pol.AgentID, Valid: true}
	}
	if pol.RouteID != "" {
		rteID = sql.NullString{String: pol.RouteID, Valid: true}
	}

	_, err = p.db.ExecContext(ctx, `
		UPDATE ap_policy
		SET agent_id = $1, route_id = $2, max_amount_per_request = $3, rolling_24h_budget_cap = $4, circuit_breaker_max_rpm = $5, whitelisted_payees = $6, whitelisted_networks = $7, whitelisted_routes = $8, is_active = $9, updated_at = NOW()
		WHERE policy_id = $10;
	`, agID, rteID, pol.MaxAmountPerRequest, pol.Rolling24hBudgetCap, pol.CircuitBreakerMaxRPM, payeesBytes, networksBytes, routesBytes, pol.IsActive, pol.PolicyID)
	if err != nil {
		return nil, err
	}

	pol.CreatedAt = existing.CreatedAt
	pol.UpdatedAt = time.Now().Unix()
	p.recordAudit(ctx, pol.TenantID, actorMemberID, "policy.updated", "Policy", pol.PolicyID, nil, nil, "127.0.0.1")
	_ = p.SyncRCU(ctx)
	return pol, nil
}

func (p *PostgresService) DeletePolicy(ctx context.Context, tenantID, policyID, actorMemberID string) error {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionPolicyWrite); err != nil {
		return err
	}

	_, err := p.GetPolicy(ctx, tenantID, policyID)
	if err != nil {
		return err
	}

	_, err = p.db.ExecContext(ctx, `DELETE FROM ap_policy WHERE policy_id = $1;`, policyID)
	if err != nil {
		return err
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "policy.deleted", "Policy", policyID, nil, nil, "127.0.0.1")
	_ = p.SyncRCU(ctx)
	return nil
}

// ----------------------------------------------------------------------------
// Config Draft & Validation (T4.3)
// ----------------------------------------------------------------------------

func (p *PostgresService) ValidateConfig(ctx context.Context, tenantID string, actorMemberID string) (*ValidationResult, error) {
	_, err := p.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionRouteRead); err != nil {
		return nil, err
	}

	routes, err := p.ListRoutes(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	policies, err := p.ListPolicies(ctx, tenantID, "", "")
	if err != nil {
		return nil, err
	}

	errorsList := make([]ValidationError, 0)
	seenPatterns := make(map[string]string)

	for _, r := range routes {
		if !strings.HasPrefix(r.UpstreamURL, "http://") && !strings.HasPrefix(r.UpstreamURL, "https://") {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "upstream_url",
				Code:       "INVALID_UPSTREAM_URL",
				Message:    fmt.Sprintf("上游地址 '%s' 非法，必须以 http:// 或 https:// 开头", r.UpstreamURL),
			})
		}

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

		if r.AllowedPaymentMethods != "wallet" {
			if r.PayeeAddress == "" {
				errorsList = append(errorsList, ValidationError{
					TargetType: "Route",
					TargetID:   r.RouteID,
					Field:      "payee_address",
					Code:       "MISSING_PAYEE_ADDRESS",
					Message:    "支持链上支付的路由必须配置收款地址 (payee_address)",
				})
			} else if !pgEthAddressValidationRegex.MatchString(r.PayeeAddress) {
				errorsList = append(errorsList, ValidationError{
					TargetType: "Route",
					TargetID:   r.RouteID,
					Field:      "payee_address",
					Code:       "INVALID_PAYEE_ADDRESS",
					Message:    fmt.Sprintf("收款地址 '%s' 非法，必须为标准的 0x 开头 40 位十六进制以太坊地址", r.PayeeAddress),
				})
			}
		}

		if r.RateLimitRPM < 0 {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "rate_limit_rpm",
				Code:       "INVALID_RATE_LIMIT",
				Message:    "限流阈值 RPM 不能小于 0",
			})
		}
		if r.TimeoutMs <= 0 {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Route",
				TargetID:   r.RouteID,
				Field:      "timeout_ms",
				Code:       "INVALID_TIMEOUT",
				Message:    "超时时间必须大于 0 毫秒",
			})
		}
	}

	for _, pol := range policies {
		if pol.MaxAmountPerRequest <= 0 {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Policy",
				TargetID:   pol.PolicyID,
				Field:      "max_amount_per_request",
				Code:       "INVALID_MAX_AMOUNT",
				Message:    "单笔最大支付金额必须大于 0",
			})
		}
		if pol.Rolling24hBudgetCap < pol.MaxAmountPerRequest {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Policy",
				TargetID:   pol.PolicyID,
				Field:      "rolling_24h_budget_cap",
				Code:       "BUDGET_CAP_LESS_THAN_MAX_PER_TX",
				Message:    fmt.Sprintf("24h 预算上限 (%d) 不能小于单笔支付上限 (%d)", pol.Rolling24hBudgetCap, pol.MaxAmountPerRequest),
			})
		}
		if pol.CircuitBreakerMaxRPM < 0 {
			errorsList = append(errorsList, ValidationError{
				TargetType: "Policy",
				TargetID:   pol.PolicyID,
				Field:      "circuit_breaker_max_rpm",
				Code:       "INVALID_CIRCUIT_BREAKER_RPM",
				Message:    "熔断阈值 RPM 不能小于 0",
			})
		}
		for _, net := range pol.WhitelistedNetworks {
			if !pgCaip2ValidationRegex.MatchString(net) {
				errorsList = append(errorsList, ValidationError{
					TargetType: "Policy",
					TargetID:   pol.PolicyID,
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

	p.mu.Lock()
	p.lastValidation[tenantID] = res
	p.mu.Unlock()

	p.recordAudit(ctx, tenantID, actorMemberID, "config.validated", "Tenant", tenantID, nil, map[string]interface{}{
		"valid":       res.Valid,
		"error_count": len(errorsList),
	}, "127.0.0.1")

	return res, nil
}

func (p *PostgresService) GetDraftSummary(ctx context.Context, tenantID string) (*ConfigDraftSummary, error) {
	routes, err := p.ListRoutes(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	policies, err := p.ListPolicies(ctx, tenantID, "", "")
	if err != nil {
		return nil, err
	}

	p.mu.RLock()
	lastRes, hasLast := p.lastValidation[tenantID]
	p.mu.RUnlock()

	var lastTime *int64
	isValidated := false
	if hasLast && lastRes != nil {
		lastTime = &lastRes.ValidatedAt
		isValidated = lastRes.Valid
	}

	return &ConfigDraftSummary{
		TenantID:        tenantID,
		PendingChanges:  len(routes) + len(policies),
		RoutesCount:     len(routes),
		PoliciesCount:   len(policies),
		LastValidatedAt: lastTime,
		IsValidated:     isValidated,
		LastResult:      lastRes,
	}, nil
}

func (p *PostgresService) GetDraftDiff(ctx context.Context, tenantID string) (*ConfigDraftDiff, error) {
	routes, err := p.ListRoutes(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	policies, err := p.ListPolicies(ctx, tenantID, "", "")
	if err != nil {
		return nil, err
	}

	routeDiffs := make([]RouteDiffItem, len(routes))
	for i, r := range routes {
		routeDiffs[i] = RouteDiffItem{
			RouteID:     r.RouteID,
			Action:      "added",
			PathPattern: r.PathPattern,
			After: map[string]interface{}{
				"upstream_url":     r.UpstreamURL,
				"price_base_units": r.PriceBaseUnits,
			},
		}
	}

	policyDiffs := make([]PolicyDiffItem, len(policies))
	for i, pol := range policies {
		policyDiffs[i] = PolicyDiffItem{
			PolicyID: pol.PolicyID,
			Action:   "added",
			After: map[string]interface{}{
				"max_amount_per_request": pol.MaxAmountPerRequest,
				"rolling_24h_budget_cap": pol.Rolling24hBudgetCap,
			},
		}
	}

	return &ConfigDraftDiff{
		TenantID:        tenantID,
		ActiveVersionID: "",
		HasChanges:      len(routeDiffs) > 0 || len(policyDiffs) > 0,
		RouteDiffs:      routeDiffs,
		PolicyDiffs:     policyDiffs,
	}, nil
}

// ----------------------------------------------------------------------------
// Config Publishing, Versions & Rollback (T4.4)
// ----------------------------------------------------------------------------

func (p *PostgresService) PublishConfig(ctx context.Context, tenantID string, changeSummary string, actorMemberID string) (*ConfigVersion, error) {
	_, err := p.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionRouteWrite); err != nil {
		return nil, err
	}

	// 1. Static validation matrix gate
	valResult, err := p.ValidateConfig(ctx, tenantID, actorMemberID)
	if err != nil {
		return nil, err
	}
	if !valResult.Valid {
		return nil, ErrConfigValidationFailed
	}

	routes, err := p.ListRoutes(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	policies, err := p.ListPolicies(ctx, tenantID, "", "")
	if err != nil {
		return nil, err
	}

	rcuRoutes := make([]rcu.RouteConfig, 0, len(routes))
	for _, r := range routes {
		rcuRoutes = append(rcuRoutes, rcu.RouteConfig{
			RouteID:               r.RouteID,
			ProjectID:             r.ProjectID,
			Network:               r.Network,
			PathPattern:           r.PathPattern,
			UpstreamURL:           r.UpstreamURL,
			PriceBaseUnits:        r.PriceBaseUnits,
			Asset:                 r.Asset,
			OnchainPriceBaseUnits: r.OnchainPriceBaseUnits,
			OnchainAsset:          r.OnchainAsset,
			PayeeAddress:          r.PayeeAddress,
			AllowedPaymentMethods: r.AllowedPaymentMethods,
			TimeoutMS:             r.TimeoutMs,
			RateLimitRPM:          r.RateLimitRPM,
			Status:                string(r.Status),
			UpstreamAuth:          r.UpstreamAuth,
		})
	}

	rcuPolicies := make([]rcu.PolicyConfig, 0, len(policies))
	for _, pol := range policies {
		rcuPolicies = append(rcuPolicies, rcu.PolicyConfig{
			PolicyID:             pol.PolicyID,
			ProjectID:            pol.ProjectID,
			RouteID:              &pol.RouteID,
			MaxAmountPerRequest:  pol.MaxAmountPerRequest,
			Rolling24hBudgetCap:  pol.Rolling24hBudgetCap,
			CircuitBreakerMaxRPM: pol.CircuitBreakerMaxRPM,
			WhitelistedNetworks:  pol.WhitelistedNetworks,
			WhitelistedRoutes:    pol.WhitelistedRoutes,
			IsActive:             pol.IsActive,
		})
	}

	snapshotHash := rcu.ComputeCanonicalSnapshotHash(rcuRoutes, rcuPolicies)

	var maxVerNum int64 = 0
	_ = p.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version_number), 0) FROM ap_config_version WHERE tenant_id = $1", tenantID).Scan(&maxVerNum)
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

	snapBytes, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "UPDATE ap_config_version SET is_active = FALSE WHERE tenant_id = $1 AND is_active = TRUE", tenantID)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO ap_config_version (
			version_id, tenant_id, version_number, snapshot_json, published_by, change_summary, is_active, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, TRUE, NOW())
	`, versionID, tenantID, newVersionNumber, snapBytes, actorMemberID, summary)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var publisherEmail string
	_ = p.db.QueryRowContext(ctx, "SELECT email FROM ap_member WHERE member_id = $1", actorMemberID).Scan(&publisherEmail)
	if publisherEmail == "" {
		publisherEmail = actorMemberID
	}

	if p.rcuMgr != nil {
		_ = p.rcuMgr.Update(snap)
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "config.published", "ConfigVersion", versionID, nil, map[string]interface{}{
		"version_number": newVersionNumber,
		"snapshot_hash":  snapshotHash,
		"change_summary": summary,
		"routes_count":   len(rcuRoutes),
		"policies_count": len(rcuPolicies),
	}, "127.0.0.1")

	return &ConfigVersion{
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
	}, nil
}

func (p *PostgresService) ListConfigVersions(ctx context.Context, tenantID string) ([]*ConfigVersion, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT v.version_id, v.tenant_id, v.version_number, v.snapshot_json, v.published_by, COALESCE(m.email, v.published_by), v.change_summary, v.is_active, EXTRACT(EPOCH FROM v.created_at)::BIGINT
		FROM ap_config_version v
		LEFT JOIN ap_member m ON v.published_by = m.member_id
		WHERE v.tenant_id = $1
		ORDER BY v.version_number DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*ConfigVersion, 0)
	for rows.Next() {
		var v ConfigVersion
		var snapBytes []byte
		err := rows.Scan(
			&v.VersionID,
			&v.TenantID,
			&v.VersionNumber,
			&snapBytes,
			&v.PublishedBy,
			&v.PublisherEmail,
			&v.ChangeSummary,
			&v.IsActive,
			&v.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		var snap rcu.ConfigSnapshot
		if err := json.Unmarshal(snapBytes, &snap); err == nil {
			v.SnapshotJSON = &snap
			v.SnapshotHash = snap.SnapshotHash
			v.RoutesCount = len(snap.Routes)
			v.PoliciesCount = len(snap.Policies)
		}
		list = append(list, &v)
	}
	return list, nil
}

func (p *PostgresService) GetConfigVersion(ctx context.Context, tenantID, versionID string) (*ConfigVersion, error) {
	var v ConfigVersion
	var snapBytes []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT v.version_id, v.tenant_id, v.version_number, v.snapshot_json, v.published_by, COALESCE(m.email, v.published_by), v.change_summary, v.is_active, EXTRACT(EPOCH FROM v.created_at)::BIGINT
		FROM ap_config_version v
		LEFT JOIN ap_member m ON v.published_by = m.member_id
		WHERE v.tenant_id = $1 AND v.version_id = $2
	`, tenantID, versionID).Scan(
		&v.VersionID,
		&v.TenantID,
		&v.VersionNumber,
		&snapBytes,
		&v.PublishedBy,
		&v.PublisherEmail,
		&v.ChangeSummary,
		&v.IsActive,
		&v.CreatedAt,
	)
	if err != nil {
		return nil, ErrConfigVersionNotFound
	}

	var snap rcu.ConfigSnapshot
	if err := json.Unmarshal(snapBytes, &snap); err == nil {
		v.SnapshotJSON = &snap
		v.SnapshotHash = snap.SnapshotHash
		v.RoutesCount = len(snap.Routes)
		v.PoliciesCount = len(snap.Policies)
	}
	return &v, nil
}

func (p *PostgresService) RollbackConfig(ctx context.Context, tenantID, targetVersionID, reason, actorMemberID string) (*ConfigVersion, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionRouteWrite); err != nil {
		return nil, err
	}

	targetVer, err := p.GetConfigVersion(ctx, tenantID, targetVersionID)
	if err != nil {
		return nil, err
	}
	if targetVer.IsActive {
		return nil, ErrCannotRollbackToActive
	}

	var maxVerNum int64 = 0
	_ = p.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version_number), 0) FROM ap_config_version WHERE tenant_id = $1", tenantID).Scan(&maxVerNum)
	newVersionNumber := maxVerNum + 1
	versionID := GenerateConfigVersionID()
	now := time.Now().Unix()

	summary := fmt.Sprintf("Rollback to version v%d", targetVer.VersionNumber)
	if strings.TrimSpace(reason) != "" {
		summary = fmt.Sprintf("Rollback to version v%d: %s", targetVer.VersionNumber, strings.TrimSpace(reason))
	}

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

	snapBytes, err := json.Marshal(restoredSnap)
	if err != nil {
		return nil, err
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "UPDATE ap_config_version SET is_active = FALSE WHERE tenant_id = $1 AND is_active = TRUE", tenantID)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO ap_config_version (
			version_id, tenant_id, version_number, snapshot_json, published_by, change_summary, is_active, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, TRUE, NOW())
	`, versionID, tenantID, newVersionNumber, snapBytes, actorMemberID, summary)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var publisherEmail string
	_ = p.db.QueryRowContext(ctx, "SELECT email FROM ap_member WHERE member_id = $1", actorMemberID).Scan(&publisherEmail)
	if publisherEmail == "" {
		publisherEmail = actorMemberID
	}

	if p.rcuMgr != nil {
		_ = p.rcuMgr.Update(restoredSnap)
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "config.rolled_back", "ConfigVersion", versionID, map[string]interface{}{
		"target_version_id": targetVer.VersionID,
	}, map[string]interface{}{
		"new_version_id":     versionID,
		"new_version_number": newVersionNumber,
		"active_hash":        targetVer.SnapshotHash,
		"reason":             reason,
	}, "127.0.0.1")

	return &ConfigVersion{
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
	}, nil
}

func (p *PostgresService) GetConfigVersionDiff(ctx context.Context, tenantID, baseVersionID, targetVersionID string) (*ConfigDraftDiff, error) {
	var baseSnap *rcu.ConfigSnapshot
	var targetSnap *rcu.ConfigSnapshot

	if baseVersionID != "" {
		v, err := p.GetConfigVersion(ctx, tenantID, baseVersionID)
		if err != nil {
			return nil, err
		}
		baseSnap = v.SnapshotJSON
	} else {
		var snapBytes []byte
		var activeVerID string
		err := p.db.QueryRowContext(ctx, "SELECT version_id, snapshot_json FROM ap_config_version WHERE tenant_id = $1 AND is_active = TRUE", tenantID).Scan(&activeVerID, &snapBytes)
		if err == nil {
			var s rcu.ConfigSnapshot
			if json.Unmarshal(snapBytes, &s) == nil {
				baseSnap = &s
			}
		}
	}

	if targetVersionID != "" {
		v, err := p.GetConfigVersion(ctx, tenantID, targetVersionID)
		if err != nil {
			return nil, err
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
// Virtual Wallet Admin
// ----------------------------------------------------------------------------

func (p *PostgresService) AdminGrantWallet(ctx context.Context, tenantID string, amount int64, reason string, idempotencyKey string, actorMemberID string, clientIP string) error {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return ErrMissingReason
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}

	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionWalletAdjust); err != nil {
		return err
	}

	w, err := p.walletSvc.GetWalletByTenant(ctx, tenantID)
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

	if err := p.walletSvc.CreditTopUp(ctx, order); err != nil {
		return err
	}

	p.recordAudit(ctx, tenantID, actorMemberID, "wallet.admin_grant", "Wallet", w.WalletID, nil, map[string]interface{}{"amount": amount, "reason": cleanReason}, clientIP)
	return nil
}

func (p *PostgresService) AdjustWallet(ctx context.Context, walletID string, amount int64, reason string, idempotencyKey string, actorMemberID string, clientIP string) error {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return ErrMissingReason
	}

	w, err := p.walletSvc.GetWallet(ctx, walletID)
	if err != nil {
		return ErrWalletNotFound
	}

	if err := p.enforcePermission(ctx, actorMemberID, w.TenantID, PermissionWalletAdjust); err != nil {
		return err
	}

	if err := p.walletSvc.AdjustBalance(ctx, walletID, amount, cleanReason, idempotencyKey, actorMemberID); err != nil {
		return err
	}

	p.recordAudit(ctx, w.TenantID, actorMemberID, "wallet.adjust", "Wallet", walletID, nil, map[string]interface{}{"amount": amount, "reason": cleanReason}, clientIP)
	return nil
}

func (p *PostgresService) FreezeWallet(ctx context.Context, walletID string, reason string, actorMemberID string, clientIP string) error {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return ErrMissingReason
	}

	w, err := p.walletSvc.GetWallet(ctx, walletID)
	if err != nil {
		return ErrWalletNotFound
	}

	if err := p.enforcePermission(ctx, actorMemberID, w.TenantID, PermissionWalletAdjust); err != nil {
		return err
	}

	if err := p.walletSvc.SetWalletStatus(ctx, walletID, wallet.StatusFrozen); err != nil {
		return err
	}

	p.recordAudit(ctx, w.TenantID, actorMemberID, "wallet.frozen", "Wallet", walletID, nil, map[string]interface{}{"reason": cleanReason}, clientIP)
	return nil
}

func (p *PostgresService) UnfreezeWallet(ctx context.Context, walletID string, reason string, actorMemberID string, clientIP string) error {
	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		return ErrMissingReason
	}

	w, err := p.walletSvc.GetWallet(ctx, walletID)
	if err != nil {
		return ErrWalletNotFound
	}

	if err := p.enforcePermission(ctx, actorMemberID, w.TenantID, PermissionWalletAdjust); err != nil {
		return err
	}

	if w.Status == wallet.StatusFrozenArrears && w.AvailableCache < 0 {
		return fmt.Errorf("cannot unfreeze wallet in arrears")
	}

	if err := p.walletSvc.SetWalletStatus(ctx, walletID, wallet.StatusActive); err != nil {
		return err
	}

	p.recordAudit(ctx, w.TenantID, actorMemberID, "wallet.unfrozen", "Wallet", walletID, nil, map[string]interface{}{"reason": cleanReason}, clientIP)
	return nil
}

func (p *PostgresService) RecalculateWallet(ctx context.Context, walletID string, actorMemberID string) (map[string]interface{}, error) {
	rep, err := p.walletSvc.ReconcileBalance(ctx, walletID)
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

func (p *PostgresService) GetWallet(ctx context.Context, tenantID string) (*wallet.Wallet, error) {
	return p.walletSvc.GetWalletByTenant(ctx, tenantID)
}

func (p *PostgresService) GetWalletByID(ctx context.Context, walletID string) (*wallet.Wallet, error) {
	return p.walletSvc.GetWallet(ctx, walletID)
}

func (p *PostgresService) GetLedgerEntries(ctx context.Context, walletID string) ([]*wallet.LedgerEntry, error) {
	return p.walletSvc.GetLedgerEntries(ctx, walletID)
}

func (p *PostgresService) GetTopUpOrders(ctx context.Context, walletID string) ([]*wallet.TopUpOrder, error) {
	return p.walletSvc.GetTopUpOrders(ctx, walletID)
}

// ----------------------------------------------------------------------------
// Audit Events
// ----------------------------------------------------------------------------

func (p *PostgresService) ListAuditEvents(ctx context.Context, tenantID, targetType, targetID string, limit int) ([]*AuditEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var sb strings.Builder
	sb.WriteString(`
		SELECT audit_id, tenant_id, operator_id, action, target_type, target_id, before_state, after_state, client_ip,
		       EXTRACT(EPOCH FROM created_at)::bigint
		FROM ap_audit_event
		WHERE 1=1
	`)
	args := make([]interface{}, 0)
	idx := 1

	if tenantID != "" {
		sb.WriteString(fmt.Sprintf(" AND tenant_id = $%d", idx))
		args = append(args, tenantID)
		idx++
	}
	if targetType != "" {
		sb.WriteString(fmt.Sprintf(" AND target_type = $%d", idx))
		args = append(args, targetType)
		idx++
	}
	if targetID != "" {
		sb.WriteString(fmt.Sprintf(" AND target_id = $%d", idx))
		args = append(args, targetID)
		idx++
	}
	sb.WriteString(fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d;", idx))
	args = append(args, limit)

	rows, err := p.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]*AuditEvent, 0)
	for rows.Next() {
		var ev AuditEvent
		var beforeBytes, afterBytes []byte
		if err := rows.Scan(&ev.AuditID, &ev.TenantID, &ev.OperatorID, &ev.Action, &ev.TargetType, &ev.TargetID, &beforeBytes, &afterBytes, &ev.ClientIP, &ev.CreatedAt); err != nil {
			return nil, err
		}
		if len(beforeBytes) > 0 {
			_ = json.Unmarshal(beforeBytes, &ev.BeforeState)
		}
		if len(afterBytes) > 0 {
			_ = json.Unmarshal(afterBytes, &ev.AfterState)
		}
		events = append(events, &ev)
	}
	return events, nil
}

// ----------------------------------------------------------------------------
// Upstream Authentication & Connectivity Testing (T4.2 Enhancement)
// ----------------------------------------------------------------------------

func (p *PostgresService) GetAuthTypes() []upstreamauth.AuthTypeSchema {
	return upstreamauth.ListSchemas()
}

func (p *PostgresService) TestUpstream(ctx context.Context, tenantID string, input *TestUpstreamInput, actorMemberID string) (*TestUpstreamResult, error) {
	if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionRouteRead); err != nil {
		return nil, err
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

	p.recordAudit(ctx, tenantID, actorMemberID, "route.tested", "Route", "", nil, map[string]interface{}{
		"url":         cleanURL,
		"auth_type":   "",
		"success":     result.Success,
		"status_code": result.StatusCode,
	}, "127.0.0.1")

	return result, nil
}

// ----------------------------------------------------------------------------
// RCU Data Plane Synchronization
// ----------------------------------------------------------------------------

func (p *PostgresService) SetRCUManager(mgr *rcu.Manager) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rcuMgr = mgr
}

func (p *PostgresService) SyncRCU(ctx context.Context) error {
	p.mu.RLock()
	mgr := p.rcuMgr
	p.mu.RUnlock()
	if mgr == nil {
		return nil
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT r.route_id, r.tenant_id, r.project_id, r.path_pattern, r.upstream_url,
		       r.price_base_units, r.asset, r.onchain_price_base_units, r.onchain_asset, r.payee_address, r.allowed_payment_methods,
		       r.timeout_ms, r.rate_limit_rpm, r.status,
		       c.type, c.params, c.secret_enc, c.key_version
		FROM ap_route r
		LEFT JOIN api_auth_configs c ON c.api_id = r.route_id
		WHERE r.status = 'active';
	`)
	if err != nil {
		return fmt.Errorf("failed to query active routes: %w", err)
	}
	defer rows.Close()

	var routes []rcu.RouteConfig
	for rows.Next() {
		var r rcu.RouteConfig
		var onchainPrice sql.NullInt64
		var onchainAsset sql.NullString
		var cType sql.NullString
		var cParamsBytes, cSecretEnc []byte
		var cKeyVersion sql.NullInt64

		if err := rows.Scan(
			&r.RouteID, &r.TenantID, &r.ProjectID, &r.PathPattern, &r.UpstreamURL,
			&r.PriceBaseUnits, &r.Asset, &onchainPrice, &onchainAsset, &r.PayeeAddress, &r.AllowedPaymentMethods,
			&r.TimeoutMS, &r.RateLimitRPM, &r.Status,
			&cType, &cParamsBytes, &cSecretEnc, &cKeyVersion,
		); err != nil {
			return fmt.Errorf("failed to scan active route: %w", err)
		}
		if onchainPrice.Valid {
			v := onchainPrice.Int64
			r.OnchainPriceBaseUnits = &v
		}
		if onchainAsset.Valid {
			r.OnchainAsset = onchainAsset.String
		}

		if cType.Valid && cType.String != "" {
			var params map[string]interface{}
			if len(cParamsBytes) > 0 {
				_ = json.Unmarshal(cParamsBytes, &params)
			}
			r.UpstreamAuth = &upstreamauth.UpstreamAuthConfig{
				Type:       cType.String,
				Params:     params,
				SecretEnc:  cSecretEnc,
				HasSecret:  len(cSecretEnc) > 0,
				KeyVersion: int(cKeyVersion.Int64),
			}
		}
		routes = append(routes, r)
	}

	polRows, err := p.db.QueryContext(ctx, `
		SELECT policy_id, tenant_id, project_id, agent_id, route_id,
		       max_amount_per_request, rolling_24h_budget_cap, circuit_breaker_max_rpm,
		       whitelisted_payees, whitelisted_networks, whitelisted_routes, is_active
		FROM ap_policy
		WHERE is_active = TRUE;
	`)
	var policies []rcu.PolicyConfig
	if err == nil {
		defer polRows.Close()
		for polRows.Next() {
			var pol rcu.PolicyConfig
			var agID, rteID sql.NullString
			var payeesBytes, networksBytes, routesBytes []byte
			if err := polRows.Scan(
				&pol.PolicyID, &pol.TenantID, &pol.ProjectID, &agID, &rteID,
				&pol.MaxAmountPerRequest, &pol.Rolling24hBudgetCap, &pol.CircuitBreakerMaxRPM,
				&payeesBytes, &networksBytes, &routesBytes, &pol.IsActive,
			); err == nil {
				if agID.Valid {
					pol.AgentID = &agID.String
				}
				if rteID.Valid {
					pol.RouteID = &rteID.String
				}
				_ = json.Unmarshal(networksBytes, &pol.WhitelistedNetworks)
				_ = json.Unmarshal(routesBytes, &pol.WhitelistedRoutes)
				policies = append(policies, pol)
			}
		}
	}

	// Merge with bootstrap routes and policies if any
	curr := mgr.Current()
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
		PublishedBy:   "admin_postgres_sync",
		Routes:        routes,
		Policies:      policies,
	}

	return mgr.Update(newSnap)
}

// SetUsageService configures a custom usage service on PostgresService.
func (p *PostgresService) SetUsageService(svc usage.Service) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usageSvc = svc
}

// GetUsageService returns the active usage service or initializes a PostgreSQL-backed one.
func (p *PostgresService) GetUsageService() usage.Service {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.usageSvc == nil {
		p.usageSvc = usage.NewPostgresService(p.db)
	}
	return p.usageSvc
}

// ListRequestRecords retrieves request records with RBAC and tenant boundary enforcement.
func (p *PostgresService) ListRequestRecords(ctx context.Context, tenantID string, filter usage.RequestFilter, actorMemberID string) ([]*usage.RequestRecord, int64, error) {
	if actorMemberID != "" {
		if err := p.enforcePermission(ctx, tenantID, actorMemberID, PermissionAuditRead); err != nil {
			return nil, 0, err
		}
	}
	filter.TenantID = tenantID
	return p.GetUsageService().ListRequestRecords(ctx, filter)
}

// GetRequestRecord retrieves a single request record by ID with RBAC and tenant boundary enforcement.
func (p *PostgresService) GetRequestRecord(ctx context.Context, tenantID, requestID string, actorMemberID string) (*usage.RequestRecord, error) {
	if actorMemberID != "" {
		if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAuditRead); err != nil {
			return nil, err
		}
	}
	return p.GetUsageService().GetRequestRecord(ctx, tenantID, requestID)
}

// ListUsageEvents retrieves usage events with RBAC and tenant boundary enforcement.
func (p *PostgresService) ListUsageEvents(ctx context.Context, tenantID string, filter usage.UsageFilter, actorMemberID string) ([]*usage.UsageEvent, int64, error) {
	if actorMemberID != "" {
		if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAuditRead); err != nil {
			return nil, 0, err
		}
	}
	filter.TenantID = tenantID
	return p.GetUsageService().ListUsageEvents(ctx, filter)
}

// GetUsageEvent retrieves a single usage event by ID with RBAC and tenant boundary enforcement.
func (p *PostgresService) GetUsageEvent(ctx context.Context, tenantID, eventID string, actorMemberID string) (*usage.UsageEvent, error) {
	if actorMemberID != "" {
		if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAuditRead); err != nil {
			return nil, err
		}
	}
	return p.GetUsageService().GetUsageEvent(ctx, tenantID, eventID)
}

// SetPaymentService configures the payment correlation service.
func (p *PostgresService) SetPaymentService(svc payment.Service) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paymentSvc = svc
}

// GetPaymentService returns the active payment correlation service.
func (p *PostgresService) GetPaymentService() payment.Service {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.paymentSvc == nil {
		return payment.NewPostgresService(p.db)
	}
	return p.paymentSvc
}

// GetPaymentAudit retrieves a single payment correlation audit record with RBAC and tenant boundary check.
func (p *PostgresService) GetPaymentAudit(ctx context.Context, tenantID, intentID string, actorMemberID string) (*payment.PaymentAuditRecord, error) {
	if actorMemberID != "" {
		if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAuditRead); err != nil {
			return nil, err
		}
	}
	return p.GetPaymentService().GetPaymentAudit(ctx, tenantID, intentID)
}

// GetPaymentAuditByRequestID retrieves a payment correlation record by request_id.
func (p *PostgresService) GetPaymentAuditByRequestID(ctx context.Context, tenantID, requestID string, actorMemberID string) (*payment.PaymentAuditRecord, error) {
	if actorMemberID != "" {
		if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAuditRead); err != nil {
			return nil, err
		}
	}
	return p.GetPaymentService().GetPaymentAuditByRequestID(ctx, tenantID, requestID)
}

// ListPaymentAudits retrieves a paginated list of payment correlation records with RBAC.
func (p *PostgresService) ListPaymentAudits(ctx context.Context, tenantID string, filter payment.PaymentAuditFilter, actorMemberID string) ([]*payment.PaymentAuditRecord, int64, error) {
	if actorMemberID != "" {
		if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAuditRead); err != nil {
			return nil, 0, err
		}
	}
	filter.TenantID = tenantID
	return p.GetPaymentService().ListPaymentAudits(ctx, filter)
}

// SetTimelineService configures the timeline service.
func (p *PostgresService) SetTimelineService(svc timeline.Service) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.timelineSvc = svc
}

// GetTimelineService returns the active timeline service.
func (p *PostgresService) GetTimelineService() timeline.Service {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.timelineSvc == nil {
		return timeline.NewPostgresService(p.db)
	}
	return p.timelineSvc
}

// GetAgentTimeline retrieves agent timeline events with RBAC, tenant ownership, and session aggregation.
func (p *PostgresService) GetAgentTimeline(ctx context.Context, tenantID, agentID string, filter timeline.TimelineFilter, actorMemberID string) ([]*timeline.TimelineEvent, int64, []string, error) {
	if actorMemberID != "" {
		if err := p.enforcePermission(ctx, actorMemberID, tenantID, PermissionAuditRead); err != nil {
			return nil, 0, nil, err
		}
	}

	ag, err := p.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, 0, nil, err
	}
	if ag == nil || ag.TenantID != tenantID {
		return nil, 0, nil, ErrAgentNotFound
	}

	filter.TenantID = tenantID
	filter.AgentID = agentID
	return p.GetTimelineService().GetTimeline(ctx, tenantID, agentID, filter)
}



