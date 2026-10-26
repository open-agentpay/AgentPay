# ADR-002: Database Selection Upgrade to PostgreSQL

> **Status**: **Accepted**  
> **Deciders**: AgentPay Architecture Team & Business Leads  
> **Date**: 2026-10-18 (Updated: 2026-10-26)  
> **Prerequisite Task**: T1.2 Design Multi-Tenant Logical Data Model  
> **Supersedes**: Replaces Phase 0's single-node local file approach relying solely on SQLite  
> **Related Specifications**:
> - [`specs/multitenant-data-model.md`](./multitenant-data-model.md) (Multi-Tenant Logical Data Model)
> - [`specs/wallet-ledger-spec.md`](./wallet-ledger-spec.md) (Virtual Wallet & Append-Only Ledger Specification)
> - [`infra/postgres/schema.sql`](../infra/postgres/schema.sql) (Authoritative PostgreSQL DDL Schema)

---

## 1. Context & Problem Statement

During AgentPay Phase 0 (the minimal Python prototype stage), the system relied on Python's built-in SQLite (`sqlite3`) combined with WAL (`Write-Ahead Logging`) mode and `BEGIN IMMEDIATE` pessimistic locking to achieve rapid, zero-dependency local validation.

However, as AgentPay evolves into a **production-grade multi-tenant payment gateway, high-concurrency cloud-native Go Gateway, Web Management Console, and dual-rail virtual wallet ledger (T1.7)**, the single-file embedded database approach faces fundamental architectural bottlenecks:

1. **Multi-Node Stateless Horizontal Scaling**: Both the Go Gateway data plane and the Python/Web control plane must deploy across multiple stateless instances (Kubernetes Pods / containers) for high availability and horizontal scaling. Because SQLite relies on OS-level file locks on a local disk, it cannot safely support concurrent read/write operations across distributed container nodes.
2. **High-Concurrency Row-Level Locking for Virtual Wallets**: The internal virtual wallet introduces two-phase fund reservations (`Hold`), settlement deductions (`Capture`), and quota releases (`Release`). SQLite serializes write transactions with a coarse database-level lock (`database is locked` under concurrent load). Financial ledger accounting requires isolated row-level locking (`SELECT ... FOR UPDATE`) so that concurrent requests from different tenants—or distinct wallets—never block one another.
3. **Cross-Instance Challenge & Nonce Replay Prevention**: In a multi-replica Go Gateway deployment, virtual-wallet challenges (`ap_wallet_challenge`) and on-chain EIP-3009 settlement nonces (`ap_settlement_record`) must be claimed atomically across all gateway instances so that a retry routed to another instance cannot bypass one-time consumption.
4. **High-Volume Audit Events & Append-Only Ledger Storage**: Request traces (`ap_request_record`), agent behavior timelines (`ap_agent_audit_event`), on-chain settlements (`ap_settlement_record`), and double-entry ledger streams (`ap_ledger_entry`) are high-throughput, append-only time-series datasets requiring partial B-Tree indexes, composite uniqueness invariants, and native table partitioning.
5. **Immutable Configuration Snapshots & JSONB Indexing**: Publishing route rules (`ap_route`) and security policies (`ap_policy`) requires persisting immutable configuration snapshots (`ap_config_version.snapshot_json`) for RCU hot-reloading, version diffing, and instant rollback—workloads where PostgreSQL's binary `JSONB` storage and GIN indexing excel.

---

## 2. Decision

We formally decide to **adopt PostgreSQL (15+, standardized on PostgreSQL 16 in production and Docker Compose) as the unified core relational and transactional data store for AgentPay**.

### 2.1 Core Architectural Standards

1. **Unified Multi-Service Data Store**:
   - The Python Control Plane API, Go Gateway data plane, Go Facilitator service, Virtual Wallet & Ledger engine, and Audit & Reconciliation subsystem all connect to a unified PostgreSQL cluster (`infra/postgres/schema.sql`).
2. **Domain-Scoped Schema Organization (18 Core Tables across 5 Domains)**:
   - **Identity, Multi-Tenancy & RBAC**: `ap_tenant`, `ap_project`, `ap_member`, `ap_agent`, `ap_api_key` (with partial index on active `key_hash`).
   - **Routing, Policy & Config Versioning**: `ap_route`, `api_auth_configs` (encrypted upstream credentials), `ap_policy`, `ap_config_version` (immutable `JSONB` snapshots).
   - **Virtual Wallet & Append-Only Ledger**: `ap_wallet`, `ap_ledger_entry`, `ap_hold`, `ap_topup_order`.
   - **Payment Intent, Challenge, On-Chain Settlement & Fulfillment**: `ap_payment_intent`, `ap_wallet_challenge`, `ap_settlement_record`, `ap_fulfillment_record`.
   - **Request Tracing, Usage Metering & Audit**: `ap_request_record`, `ap_usage_event`, `ap_audit_event`, `ap_agent_audit_event`.
3. **Strong Transactional Isolation & Row-Level Locking**:
   - All monetary amounts are stored as exact integer base units (`BIGINT`, e.g., `1 USDC = 1,000,000` base units) to eliminate floating-point rounding errors.
   - Wallet balance mutations (`Hold` / `Capture` / `Release` / `TopUp`) execute inside explicit ACID transactions acquiring a row-level lock on the target wallet:
     ```sql
     SELECT wallet_id, status, version, available_cache, held_cache
     FROM ap_wallet
     WHERE wallet_id = $1
     FOR UPDATE;
     ```
   - Database-level `CHECK` constraints enforce financial invariants directly in the storage engine (e.g., `CHECK (available_cache >= 0 OR status = 'frozen_arrears')` and `CHECK (held_cache >= 0)`), making negative-balance bugs impossible even under application race conditions.
4. **High-Performance Connection Pooling & Timeout Governance**:
   - **Go Gateway Data Plane**: Uses `jackc/pgx/v5/pgxpool` with bounded pool sizing (`MaxConns`, `MinConns`), health checks, and strict per-transaction `lock_timeout` (e.g., `3000ms`) and `statement_timeout` (e.g., `5000ms`) to prevent slow queries from cascading into gateway goroutine exhaustion.
   - **Python Control Plane**: Uses `psycopg` / `asyncpg` connection pools with automatic prepared statement caching and tenant-scoped query guards.
5. **Local Development & Tiered Testing Strategy**:
   - Both `docker-compose.yml` and `docker-compose.prod.yml` provision `postgres:16-alpine` with automatic initialization from `infra/postgres/schema.sql` and readiness health checks (`pg_isready`).
   - Pure algorithmic and cryptographic unit tests (such as x402 header codecs, EIP-712/EIP-3009 signatures, and state-machine transition matrices) remain zero-dependency for sub-second feedback, while multi-tenant isolation, wallet ledger concurrency, and E2E conformance tests run against PostgreSQL.

---

## 3. Options Considered

| Evaluation Dimension | SQLite 3 (Phase 0 Baseline) | MySQL 8.0+ (InnoDB) | **PostgreSQL 16 (Selected)** |
| :--- | :--- | :--- | :--- |
| **Concurrency & Lock Granularity** | Coarse database-level write lock (single writer globally) | Row-level locking (InnoDB index-record locks) | **Row-level locking (MVCC + deterministic `FOR UPDATE` / `SKIP LOCKED`)** |
| **Multi-Node Distributed Access** | Unsupported (local filesystem lock only) | Native client/server & replica support | **Native cloud-native HA (Patroni / CloudNativePG / Read Replicas / PgBouncer)** |
| **Semi-Structured Data (`JSONB`)** | Basic `JSON1` text parsing functions | `JSON` column type (limited indexing) | **First-class binary `JSONB` with GIN inverted indexes & path operators** |
| **Financial Invariants & Indexing** | Basic `CHECK` constraints; limited partial index ergonomics | `CHECK` constraints supported (8.0.16+), no native partial indexes | **Strict `CHECK` constraints, Partial Indexes (`WHERE status = 'active'`), `ON DELETE RESTRICT` ledger guards** |
| **Cross-Language Driver Performance** | CGo / `sqlite3` bindings (CGo overhead in Go) | Mature (`go-sql-driver/mysql`, `aiomysql`) | **Binary protocol via Go `pgx/v5` (zero-CGo) & Python `psycopg3`/`asyncpg`** |
| **Operational & Cloud Ecosystem** | Local CLI/embedded use only | Ubiquitous managed cloud offerings | **Industry standard for fintech ledgers (Supabase, RDS, Neon, CloudSQL, self-hosted Docker)** |

---

## 4. Technical Architecture & Key Design Patterns

### 4.1 Atomic Two-Phase Wallet Accounting (`Hold -> Capture / Release`)

```sql
BEGIN;

-- 1. Lock only the specific tenant's wallet row (zero contention across other tenants)
SELECT wallet_id, available_cache, held_cache, status
FROM ap_wallet
WHERE wallet_id = $1 AND tenant_id = $2
FOR UPDATE;

-- 2. Insert idempotent hold record bound to PaymentIntent
INSERT INTO ap_hold (hold_id, wallet_id, payment_intent_id, amount, status, expires_at)
VALUES ($3, $1, $4, $5, 'held', CURRENT_TIMESTAMP + INTERVAL '120 seconds');

-- 3. Move funds from available_cache to held_cache (guarded by CHECK constraint)
UPDATE ap_wallet
SET available_cache = available_cache - $5,
    held_cache      = held_cache + $5,
    version         = version + 1
WHERE wallet_id = $1;

COMMIT;
```

### 4.2 Partial Indexes for Hot-Path Gateway Lookups

To keep data-plane authentication, nonce replay checks, and hold expiration sweeps under `1 ms` as historical tables grow into millions of rows, `infra/postgres/schema.sql` leverages PostgreSQL **Partial Indexes**:

- **Active API Key Lookup**: `CREATE INDEX idx_api_key_lookup ON ap_api_key(key_hash) WHERE status = 'active';`
- **Active Hold Expiration Sweep**: `CREATE INDEX idx_hold_active_expires ON ap_hold(wallet_id, expires_at) WHERE status = 'held';`
- **In-Flight Payment Intent Recovery**: `CREATE INDEX idx_intent_active_status ON ap_payment_intent(status) WHERE status NOT IN ('completed', 'failed', 'rejected', 'expired');`
- **Pending/Unknown On-Chain Settlement Polling**: `CREATE INDEX idx_settlement_pending_unknown ON ap_settlement_record(status) WHERE status IN ('submitted', 'pending', 'unknown');`

### 4.3 Referential Integrity & Anti-Tampering Guards

- **Financial Records Use `ON DELETE RESTRICT`**: `ap_wallet`, `ap_ledger_entry`, `ap_hold`, `ap_topup_order`, and `ap_wallet_challenge` enforce `ON DELETE RESTRICT` on foreign keys so that no tenant or wallet with financial history can ever be accidentally purged via cascading deletes.
- **Idempotency & Replay Uniqueness**:
  - `uq_wallet_ledger_idempotency UNIQUE (wallet_id, idempotency_key)` prevents duplicate ledger postings.
  - `uq_nonce_replay_guard UNIQUE (network_id, asset_address, payer_address, nonce)` prevents on-chain EIP-3009 authorization replay.
  - `uq_wallet_challenge_nonce UNIQUE (wallet_id, nonce)` guarantees one-time consumption of virtual-wallet challenges across gateway replicas.

---

## 5. Consequences, Risks & Mitigations

### 5.1 Positive Consequences
- **High-Throughput Horizontal Scalability**: Removes the SQLite single-writer bottleneck, enabling multi-replica Go Gateway deployments capable of sustaining thousands of concurrent payment transactions per second.
- **Provable Financial Consistency**: ACID transactions, row-level `FOR UPDATE` locks, `BIGINT` base units, and database-enforced `CHECK` and `UNIQUE` constraints guarantee zero double-spending and 100% deterministic ledger reconciliation (`ReconcileBalance`).
- **Zero-Downtime Config Hot-Reloading**: `ap_config_version` stores immutable `JSONB` snapshots with SHA-256 digests, allowing Go Gateway instances to poll or subscribe and atomically swap in-memory RCU routing tables.

### 5.2 Risks & Mitigations
- **Risk — Connection Exhaustion Under Traffic Spikes**: High concurrency across multiple gateway pods could exhaust PostgreSQL `max_connections`.
  - *Mitigation*: Enforce bounded `pgxpool` sizing per pod, short idle connection lifetimes, and optional PgBouncer transaction pooling in production deployments.
- **Risk — Hot-Wallet Row Lock Contention for a Single Ultra-High-QPS Tenant**: While different tenants never block each other, a single tenant issuing ribuan concurrent requests per second against one `wallet_id` serializes on that wallet row.
  - *Mitigation*: Keep the `FOR UPDATE` transaction strictly limited to in-database memory/index operations (`< 1 ms`), never performing network I/O or upstream HTTP calls while holding a database transaction lock (the Hold is committed *before* dispatching the upstream request, and Capture/Release runs in a separate short transaction *after* upstream completion).
- **Risk — Local Developer Onboarding Friction**: Requiring a running PostgreSQL instance adds setup overhead compared to an embedded file.
  - *Mitigation*: Provide automated container orchestration via `make compose-up` (`docker-compose.yml`) and keep unit/in-memory repository adapters available for fast local unit testing.
