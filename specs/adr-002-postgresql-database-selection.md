# ADR-002: Database Selection Upgrade to PostgreSQL

> **Status**: **Accepted**  
> **Deciders**: AgentPay Architecture Team & Business Leads  
> **Date**: 2026-10-18  
> **Prerequisite Task**: T1.2 Design Multi-Tenant Logical Data Model  
> **Supersedes**: Replaces Phase 0's single-node local file approach relying solely on SQLite  

---

## 1. Context & Background

During AgentPay Phase 0 (the minimal Python prototype stage), the system relied on the standard library's SQLite (`sqlite3`) combined with WAL mode and `BEGIN IMMEDIATE` pessimistic locking to achieve rapid zero-dependency local validation.

However, as the project evolves into a **multi-tenant payment gateway, high-concurrency cloud-native Go Gateway, Web Management Console, and internal virtual wallet ledger (T1.7)**, the data layer faces fundamental architectural challenges:

1. **Multi-Node & Cluster Horizontal Scaling**: Both the Go Gateway data plane and the Web control plane must deploy across multiple instances (Pods) for high availability and seamless horizontal scaling. Because SQLite is backed by the local filesystem, it cannot share concurrent read/write access across stateless distributed container clusters.
2. **High-Concurrency Row-Level Locking for Virtual Wallets**: The internal virtual wallet introduces fund reservations (`Hold`) and settlement captures (`Capture`). SQLite enforces database-level write locks during transactions, which easily triggers `sqlite3.OperationalError: database is locked` under high concurrency. Financial ledger accounting requires different tenants—and distinct accounts within the same tenant—to operate without blocking one another, necessitating mature **row-level locking (such as `FOR UPDATE`)**.
3. **High-Volume Audit Events & Partitioned Ledger Storage**: Request records (`RequestRecord`), on-chain settlement records (`SettlementRecord`), and ledger entries (`LedgerEntry`) are high-frequency time-series datasets that require mature table partitioning, high-efficiency B-Tree indexes, and composite uniqueness constraints.
4. **Configuration Version Snapshots & High-Performance JSONB Operations**: Publishing route rules (`Route`) and security policies (`Policy`) requires recording immutable snapshots for fast lookups, where PostgreSQL's native `JSONB` indexing and querying capabilities excel.

---

## 2. Decision

We formally decide to **adopt PostgreSQL (>= 15.0) as the core production data store for AgentPay**.

Specific implementation standards are as follows:
1. **Unified Data Store**: The control-plane management service, data-plane Go Gateway, virtual wallet ledger, and consumption audit system all connect to a unified PostgreSQL instance or primary-replica cluster.
2. **Connection Pooling & High-Concurrency Management**: The data-plane Go Gateway uses `pgxpool`, while the control-plane API uses mature async connection pooling to strictly manage connection counts and query timeouts.
3. **Strong Transactional Row-Level Lock Isolation**:
   - Wallet deductions and holds must use: `SELECT ... FROM ap_wallet WHERE wallet_id = $1 FOR UPDATE;`
   - This guarantees serialized accuracy for concurrent charges within the same tenant while allowing concurrent charges across different tenants to execute in parallel with zero lock contention.
4. **Local Development & Lightweight Testing Support**:
   - The repository provides a standard `docker-compose.yml` to spin up a local development PostgreSQL instance with a single command.
   - Pure algorithmic and utility unit tests (such as x402 encoding/decoding and EIP-712 signing) remain zero-dependency, while database integration tests are uniformly verified against containerized PostgreSQL in CI and local environments.

---

## 3. Options Considered

| Evaluation Dimension | SQLite (Original) | MySQL (8.0+) | **PostgreSQL (Selected)** |
|---|---|---|---|
| **Concurrency Lock Granularity** | Database-level write lock (globally serialized) | Row-level locking (InnoDB) | **Row-level locking (MVCC + strict `FOR UPDATE` transactions)** |
| **Multi-Node Shared Access** | Unsupported (single-machine file only) | Native support | **Native support (cloud-native Operators / primary-replica / connection pooling)** |
| **Semi-Structured / JSON** | JSON1 text functions | JSON type (supports partial updates) | **JSONB (binary-optimized, supports GIN inverted indexes)** |
| **Financial Accounting & Invariants** | Basic constraint support | Complete transaction & constraint support | **Strictly enforced (`CHECK` constraints, Partial Indexes, native UUID, strong typing)** |
| **Cross-Language Driver Support** | Embedded C bindings per language | Mature | **Ultra-high throughput in Go (`pgx`) and Python (`asyncpg`/`psycopg3`)** |

---

## 4. Consequences & Action Items

1. **Positive Consequences**:
   - Eliminates high-concurrency throughput bottlenecks, supporting tens of thousands of read/write QPS at the gateway.
   - Multi-tenant concurrency safety for virtual wallets is fundamentally guaranteed by underlying database ACID properties.
   - Supports immutable configuration snapshots, full operational auditing, and large-scale batch processing for end-of-day automated reconciliation jobs.
2. **Action Items**:
   - Author the multi-tenant logical data model specification: `specs/multitenant-data-model.md`.
   - Establish the authoritative DDL schema script: `infra/postgres/schema.sql`.
   - Update the local `docker-compose.yml` to include the base PostgreSQL service configuration.
