-- ============================================================================
-- AgentPay Multitenant Gateway & Platform PostgreSQL Schema (T1.2 DDL)
-- Specification Version: 2.0.0
-- Target Engine: PostgreSQL 15+
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Trigger function for automatic updated_at timestamp maintenance
CREATE OR REPLACE FUNCTION set_updated_at_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 1. Identity, Multitenant & RBAC Domain
-- ============================================================================

CREATE TABLE IF NOT EXISTS ap_tenant (
    tenant_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'terminated')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER trg_tenant_updated_at BEFORE UPDATE ON ap_tenant
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_project (
    project_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    budget_cap_base_units BIGINT NOT NULL DEFAULT 0 CHECK (budget_cap_base_units >= 0),
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_project_name UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_project_tenant ON ap_project(tenant_id);
CREATE TRIGGER trg_project_updated_at BEFORE UPDATE ON ap_project
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_member (
    member_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    email VARCHAR(256) NOT NULL,
    password_hash VARCHAR(256) NULL,
    role VARCHAR(32) NOT NULL CHECK (role IN ('owner', 'admin', 'operator', 'viewer')),
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_member_email UNIQUE (tenant_id, email)
);

CREATE INDEX IF NOT EXISTS idx_member_tenant ON ap_member(tenant_id);
CREATE TRIGGER trg_member_updated_at BEFORE UPDATE ON ap_member
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_agent (
    agent_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES ap_project(project_id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'revoked')),
    policy_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_project_agent_name UNIQUE (project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_agent_tenant ON ap_agent(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_project ON ap_agent(project_id);
CREATE TRIGGER trg_agent_updated_at BEFORE UPDATE ON ap_agent
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_api_key (
    key_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES ap_project(project_id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NOT NULL REFERENCES ap_agent(agent_id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    key_hash VARCHAR(128) NOT NULL UNIQUE,
    key_prefix VARCHAR(16) NOT NULL,
    scopes JSONB NOT NULL DEFAULT '["gateway:call"]'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'expired')),
    expires_at TIMESTAMPTZ NULL,
    last_used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_key_lookup ON ap_api_key(key_hash) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_api_key_agent ON ap_api_key(agent_id);
CREATE INDEX IF NOT EXISTS idx_api_key_project ON ap_api_key(project_id);
CREATE TRIGGER trg_api_key_updated_at BEFORE UPDATE ON ap_api_key
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();


-- ============================================================================
-- 2. Routing, Policy & Config Versioning Domain
-- ============================================================================

CREATE TABLE IF NOT EXISTS ap_route (
    route_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES ap_project(project_id) ON DELETE CASCADE,
    path_pattern VARCHAR(256) NOT NULL,
    upstream_url VARCHAR(512) NOT NULL,
    price_base_units BIGINT NOT NULL CHECK (price_base_units >= 0),
    asset VARCHAR(16) NOT NULL DEFAULT 'USDC',
    payee_address VARCHAR(128) NOT NULL,
    allowed_payment_methods VARCHAR(32) NOT NULL DEFAULT 'both' CHECK (allowed_payment_methods IN ('wallet', 'onchain', 'both')),
    timeout_ms INT NOT NULL DEFAULT 30000,
    rate_limit_rpm INT NOT NULL DEFAULT 600,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_route_path UNIQUE (tenant_id, path_pattern)
);

CREATE INDEX IF NOT EXISTS idx_route_tenant_pattern ON ap_route(tenant_id, path_pattern);
CREATE TRIGGER trg_route_updated_at BEFORE UPDATE ON ap_route
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

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
CREATE TRIGGER trg_api_auth_configs_updated_at BEFORE UPDATE ON api_auth_configs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_policy (
    policy_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES ap_project(project_id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NULL REFERENCES ap_agent(agent_id) ON DELETE SET NULL,
    route_id VARCHAR(64) NULL REFERENCES ap_route(route_id) ON DELETE SET NULL,
    max_amount_per_request BIGINT NOT NULL CHECK (max_amount_per_request > 0),
    rolling_24h_budget_cap BIGINT NOT NULL CHECK (rolling_24h_budget_cap > 0),
    circuit_breaker_max_rpm INT NOT NULL DEFAULT 60,
    whitelisted_payees JSONB NOT NULL DEFAULT '[]'::jsonb,
    whitelisted_networks JSONB NOT NULL DEFAULT '["eip155:42161"]'::jsonb,
    whitelisted_routes JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_policy_tenant ON ap_policy(tenant_id);
CREATE INDEX IF NOT EXISTS idx_policy_project ON ap_policy(project_id);
CREATE INDEX IF NOT EXISTS idx_policy_agent ON ap_policy(agent_id);
CREATE INDEX IF NOT EXISTS idx_policy_route ON ap_policy(route_id);
CREATE TRIGGER trg_policy_updated_at BEFORE UPDATE ON ap_policy
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_config_version (
    version_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    snapshot_json JSONB NOT NULL,
    published_by VARCHAR(64) NOT NULL REFERENCES ap_member(member_id),
    change_summary TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_config_version UNIQUE (tenant_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_config_active ON ap_config_version(tenant_id, is_active);

-- ============================================================================
-- 3. Virtual Wallet & Append-Only Ledger Domain (T1.7)
-- ============================================================================

CREATE TABLE IF NOT EXISTS ap_wallet (
    wallet_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL UNIQUE REFERENCES ap_tenant(tenant_id) ON DELETE RESTRICT,
    asset VARCHAR(16) NOT NULL DEFAULT 'USDC',
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'frozen', 'frozen_arrears', 'closed')),
    version BIGINT NOT NULL DEFAULT 1,
    available_cache BIGINT NOT NULL DEFAULT 0 CHECK (available_cache >= 0 OR status = 'frozen_arrears'),
    held_cache BIGINT NOT NULL DEFAULT 0 CHECK (held_cache >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER trg_wallet_updated_at BEFORE UPDATE ON ap_wallet
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_ledger_entry (
    entry_id VARCHAR(64) PRIMARY KEY,
    wallet_id VARCHAR(64) NOT NULL REFERENCES ap_wallet(wallet_id) ON DELETE RESTRICT,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE RESTRICT,
    type VARCHAR(32) NOT NULL CHECK (type IN ('topup_credit', 'capture', 'hold_release', 'adjustment', 'reversal')),
    amount BIGINT NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    ref_type VARCHAR(32) NOT NULL CHECK (ref_type IN ('TopUpOrder', 'PaymentIntent', 'Hold', 'ManualAdjustment')),
    ref_id VARCHAR(64) NOT NULL,
    operator_id VARCHAR(64) NULL,
    reason TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_wallet_ledger_idempotency UNIQUE (wallet_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_ledger_wallet_created ON ap_ledger_entry(wallet_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ledger_tenant_ref ON ap_ledger_entry(tenant_id, ref_type, ref_id);

CREATE TABLE IF NOT EXISTS ap_hold (
    hold_id VARCHAR(64) PRIMARY KEY,
    wallet_id VARCHAR(64) NOT NULL REFERENCES ap_wallet(wallet_id) ON DELETE RESTRICT,
    payment_intent_id VARCHAR(64) NOT NULL UNIQUE,
    amount BIGINT NOT NULL CHECK (amount > 0),
    status VARCHAR(32) NOT NULL DEFAULT 'held' CHECK (status IN ('held', 'captured', 'released', 'expired')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_hold_active_expires ON ap_hold(wallet_id, expires_at) WHERE status = 'held';

CREATE TABLE IF NOT EXISTS ap_topup_order (
    topup_id VARCHAR(64) PRIMARY KEY,
    wallet_id VARCHAR(64) NOT NULL REFERENCES ap_wallet(wallet_id) ON DELETE RESTRICT,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE RESTRICT,
    channel VARCHAR(32) NOT NULL CHECK (channel IN ('admin_grant', 'onchain_deposit', 'fiat')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    asset VARCHAR(16) NOT NULL DEFAULT 'USDC',
    status VARCHAR(32) NOT NULL DEFAULT 'created' CHECK (status IN ('created', 'pending', 'confirmed', 'failed', 'expired', 'reversed')),
    external_ref VARCHAR(128) NULL,
    operator_id VARCHAR(64) NULL,
    reason TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    confirmed_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_topup_wallet_status ON ap_topup_order(wallet_id, status);
CREATE INDEX IF NOT EXISTS idx_topup_ext_ref ON ap_topup_order(external_ref) WHERE external_ref IS NOT NULL;

-- ============================================================================
-- 4. Payment, Onchain Settlement & Fulfillment Domain (T1.1)
-- ============================================================================

CREATE TABLE IF NOT EXISTS ap_payment_intent (
    intent_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES ap_project(project_id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NULL REFERENCES ap_agent(agent_id) ON DELETE SET NULL,
    route_id VARCHAR(64) NOT NULL REFERENCES ap_route(route_id),
    idempotency_key VARCHAR(128) NULL,
    request_id VARCHAR(64) NOT NULL,
    payment_method VARCHAR(32) NOT NULL CHECK (payment_method IN ('wallet', 'onchain')),
    amount_base_units BIGINT NOT NULL CHECK (amount_base_units >= 0),
    asset VARCHAR(16) NOT NULL DEFAULT 'USDC',
    status VARCHAR(32) NOT NULL DEFAULT 'created' CHECK (status IN (
        'created', 'challenged', 'authorized', 'rejected', 'expired', 'completed', 'failed', 'needs_review'
    )),
    config_version_id VARCHAR(64) NOT NULL REFERENCES ap_config_version(version_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tenant_idempotency_intent UNIQUE (tenant_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_intent_tenant_created ON ap_payment_intent(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_intent_active_status ON ap_payment_intent(status) 
    WHERE status NOT IN ('completed', 'failed', 'rejected', 'expired');
CREATE TRIGGER trg_intent_updated_at BEFORE UPDATE ON ap_payment_intent
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_settlement_record (
    settlement_id VARCHAR(64) PRIMARY KEY,
    intent_id VARCHAR(64) NOT NULL UNIQUE REFERENCES ap_payment_intent(intent_id) ON DELETE CASCADE,
    network_id VARCHAR(64) NOT NULL,
    asset_address VARCHAR(128) NOT NULL,
    payer_address VARCHAR(128) NOT NULL,
    payee_address VARCHAR(128) NOT NULL,
    amount_base_units BIGINT NOT NULL,
    nonce VARCHAR(128) NOT NULL,
    tx_hash VARCHAR(128) NULL,
    block_number BIGINT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN (
        'not_started', 'held', 'submitted', 'pending', 'confirmed', 'released', 'failed', 'unknown'
    )),
    error_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_nonce_replay_guard UNIQUE (network_id, asset_address, payer_address, nonce)
);

CREATE INDEX IF NOT EXISTS idx_settlement_pending_unknown ON ap_settlement_record(status) 
    WHERE status IN ('submitted', 'pending', 'unknown');
CREATE TRIGGER trg_settlement_updated_at BEFORE UPDATE ON ap_settlement_record
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

CREATE TABLE IF NOT EXISTS ap_fulfillment_record (
    fulfillment_id VARCHAR(64) PRIMARY KEY,
    intent_id VARCHAR(64) NOT NULL UNIQUE REFERENCES ap_payment_intent(intent_id) ON DELETE CASCADE,
    upstream_url VARCHAR(512) NOT NULL,
    upstream_status INT NULL,
    latency_ms INT NULL,
    response_body_hash VARCHAR(128) NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN (
        'not_started', 'in_progress', 'succeeded', 'failed', 'unknown'
    )),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER trg_fulfillment_updated_at BEFORE UPDATE ON ap_fulfillment_record
    FOR EACH ROW EXECUTE FUNCTION set_updated_at_timestamp();

-- ============================================================================
-- 5. Request Tracing, Usage Metrics & Audit Domain (T1.5)
-- ============================================================================

CREATE TABLE IF NOT EXISTS ap_request_record (
    request_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES ap_project(project_id) ON DELETE CASCADE,
    api_key_id VARCHAR(64) NULL REFERENCES ap_api_key(key_id) ON DELETE SET NULL,
    agent_id VARCHAR(64) NULL REFERENCES ap_agent(agent_id) ON DELETE SET NULL,
    route_id VARCHAR(64) NULL REFERENCES ap_route(route_id) ON DELETE SET NULL,
    config_version VARCHAR(64) NULL,
    client_ip VARCHAR(64) NOT NULL,
    http_method VARCHAR(16) NOT NULL,
    request_path VARCHAR(256) NOT NULL,
    status_code INT NOT NULL,
    duration_ms INT NOT NULL,
    error_code VARCHAR(64) NULL,
    is_replay BOOLEAN NOT NULL DEFAULT FALSE,
    intent_id VARCHAR(64) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_request_tenant_time ON ap_request_record(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_agent_time ON ap_request_record(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_intent ON ap_request_record(intent_id) WHERE intent_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS ap_usage_event (
    event_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES ap_project(project_id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NULL REFERENCES ap_agent(agent_id) ON DELETE SET NULL,
    route_id VARCHAR(64) NOT NULL REFERENCES ap_route(route_id) ON DELETE CASCADE,
    intent_id VARCHAR(64) NOT NULL REFERENCES ap_payment_intent(intent_id) ON DELETE CASCADE,
    amount_base_units BIGINT NOT NULL,
    asset VARCHAR(16) NOT NULL DEFAULT 'USDC',
    payment_method VARCHAR(32) NOT NULL,
    is_success BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_usage_tenant_hourly ON ap_usage_event(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_usage_agent ON ap_usage_event(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_usage_intent ON ap_usage_event(intent_id);

CREATE TABLE IF NOT EXISTS ap_audit_event (
    audit_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    operator_id VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    before_state JSONB NULL,
    after_state JSONB NULL,
    client_ip VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_tenant_time ON ap_audit_event(tenant_id, created_at DESC);

-- Agent Behavior Timeline & Audit Event Stream (T1.8 & T5.8)
CREATE TABLE IF NOT EXISTS ap_agent_audit_event (
    event_id VARCHAR(64) PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    version VARCHAR(16) NOT NULL DEFAULT '1.0',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    tenant_id VARCHAR(64) NOT NULL REFERENCES ap_tenant(tenant_id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES ap_project(project_id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NOT NULL REFERENCES ap_agent(agent_id) ON DELETE CASCADE,
    request_id VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(128) NULL,
    intent_id VARCHAR(64) NULL,
    hold_id VARCHAR(64) NULL,
    settlement_id VARCHAR(64) NULL,
    fulfillment_id VARCHAR(64) NULL,
    actor_type VARCHAR(32) NOT NULL,
    actor_id VARCHAR(64) NOT NULL,
    client_ip VARCHAR(64) NOT NULL,
    user_agent VARCHAR(256) NULL,
    session_id VARCHAR(128) NULL,
    task_id VARCHAR(128) NULL,
    tool_name VARCHAR(128) NULL,
    parent_request_id VARCHAR(128) NULL,
    summary TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_agent_audit_agent_time ON ap_agent_audit_event(agent_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_audit_tenant_time ON ap_agent_audit_event(tenant_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_audit_session ON ap_agent_audit_event(session_id) WHERE session_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agent_audit_request ON ap_agent_audit_event(request_id);

