# AgentPay Consumption & Operational Audit Events Specification (T1.5)

> **Specification Version**: 1.0.0  
> **Status**: Approved Specification  
> **Scope**: Go Gateway Data Plane, Control Plane API, Virtual Wallet Ledger Engine, Offline Reconciliation & Compliance Export  
> **Task Reference**: R&D Task List T1.5 (P1)

---

## 1. Contract Goals & Core Principles

Within AgentPay's dual-rail payment architecture (virtual wallet and on-chain x402) and high-concurrency multi-tenant runtime, Audit Events serve as the single source of truth for **financial traceability, dispute resolution, high-frequency usage metering, non-repudiation auditing, and cross-service observability**.

### 1.1 Design Principles
1. **Append-Only & Immutable**: Once an audit event is generated and persisted, any form of update (`UPDATE`) or physical deletion (`DELETE`) is strictly prohibited.
2. **Unified Event Envelope**: All gateway requests, payment intents, settlements, fulfillments, wallet ledger entries, and administrative operations must be wrapped in a standardized event envelope with a consistent tracing context.
3. **Correlation & Causality**: Every event must carry structured correlation identifiers (`request_id`, `idempotency_key`, `intent_id`, `hold_id`, etc.), enabling end-to-end traceability from a single inbound HTTP call down to an on-chain transaction or underlying ledger entry.
4. **Privacy & Redaction by Default**: Credentials, sensitive secrets (API Keys are recorded only by hash and prefix), private key payloads, and internal upstream stack traces are stripped or masked by default.
5. **At-Least-Once Delivery, Exactly-Once Persistence**: Event emission follows at-least-once delivery semantics, while the database and storage layers enforce strict idempotent writes using globally unique `event_id` values and business fingerprints.

---

## 2. Identifier System & Naming Conventions

All entity and event identifiers follow a unified **`prefix + globally unique ID`** format to prevent string type confusion:

| Prefix | Entity / Event Object | Example | Role & Lifecycle |
| :--- | :--- | :--- | :--- |
| `evt_` | Audit Event ID | `evt_01j7b8z4m0k9x8y7` | Globally unique identifier for a single audit event (ULID or UUIDv4 Hex) |
| `req_` | Inbound Request ID | `req_01j7b8z4m0k9x8y1` | Assigned when the gateway receives an inbound HTTP request and propagated upstream |
| `pi_` | Payment Intent ID | `pi_01j7b8z4m0k9x8y2` | Corresponds to a 402 challenge or wallet deduction intent |
| `set_` | On-Chain Settlement ID | `set_01j7b8z4m0k9x8y3` | Corresponds to an on-chain EIP-3009 authorization batch or Facilitator submission |
| `ful_` | Upstream Fulfillment ID | `ful_01j7b8z4m0k9x8y4` | Corresponds to a reverse-proxied call dispatched by the gateway to an upstream service |
| `wlt_` | Tenant Wallet ID | `wlt_01j7b8z4m0k9x8y5` | Primary virtual wallet account for a tenant |
| `hld_` | Fund Hold ID | `hld_01j7b8z4m0k9x8y6` | Quota reservation record frozen in the wallet prior to fulfillment |
| `led_` | Ledger Entry ID | `led_01j7b8z4m0k9x8y7` | Unique, immutable ledger entry ID for wallet balance changes |
| `top_` | Top-Up Order ID | `top_01j7b8z4m0k9x8y8` | Order ID for admin grants, on-chain deposits, or fiat top-ups |
| `key_` | API Key ID | `key_01j7b8z4m0k9x8y9` | Metadata identifier for an access credential |
| `adm_` | Admin Operation ID | `adm_01j7b8z4m0k9x8ya` | Control-plane administrative action trace ID |

---

## 3. Standard Event Envelope

Every audit event consists of an outer standard `Envelope` and an inner domain-specific `Payload` (`data`).

```json
{
  "event_id": "evt_01j7b8z4m0k9x8y7a1b2c3d4",
  "event_type": "wallet.hold.created",
  "version": "1.0",
  "timestamp": "2026-10-18T07:15:00.123456Z",
  "tenant_id": "tenant_enterprise_01",
  "project_id": "proj_production",
  "correlation": {
    "request_id": "req_01j7b8z4m0k9x8y100000001",
    "idempotency_key": "tx_client_uuid_998877",
    "intent_id": "pi_01j7b8z4m0k9x8y200000002",
    "hold_id": "hld_01j7b8z4m0k9x8y600000003",
    "settlement_id": null,
    "fulfillment_id": null,
    "ledger_entry_id": null,
    "topup_id": null
  },
  "actor": {
    "type": "api_key",
    "id": "key_01j7b8z4m0k9x8y900000004",
    "client_ip": "198.51.100.42",
    "user_agent": "AgentPay-Python/0.1.0"
  },
  "data": {
    "amount_base_units": 10000,
    "asset": "USDC",
    "wallet_id": "wlt_01j7b8z4m0k9x8y500000005",
    "expires_at": "2026-10-18T07:15:30.123456Z",
    "balance_available_before": 500000,
    "balance_available_after": 490000,
    "balance_held_before": 0,
    "balance_held_after": 10000
  }
}
```

### 3.1 Envelope Field Specification

| Field Name | Type | Required | Description |
| :--- | :--- | :---: | :--- |
| `event_id` | String | Yes | Globally unique ID prefixed with `evt_` (monotonically increasing ULID recommended) |
| `event_type` | String | Yes | Dot-separated hierarchical event type, e.g., `intent.authorized` |
| `version` | String | Yes | Envelope and payload schema version; currently fixed at `"1.0"` |
| `timestamp` | String | Yes | ISO 8601 UTC generation timestamp with microsecond precision |
| `tenant_id` | String | Yes | Owning tenant ID |
| `project_id` | String | No | Owning project ID; nullable for tenant-wide global actions |
| `correlation` | Object | Yes | Structured correlation tracing context (see Section 3.2) |
| `actor` | Object | Yes | Principal that triggered the event (see Section 3.3) |
| `data` | Object | Yes | Strongly typed domain payload specific to `event_type` |

### 3.2 Correlation Tracing Context (`correlation`)
- `request_id` (String, optional): Gateway HTTP request ID.
- `idempotency_key` (String, optional): Client-supplied business idempotency key.
- `intent_id` (String, optional): Payment intent ID.
- `settlement_id` (String, optional): On-chain settlement record ID.
- `fulfillment_id` (String, optional): Upstream fulfillment record ID.
- `hold_id` (String, optional): Virtual wallet reservation ID.
- `ledger_entry_id` (String, optional): Wallet ledger entry ID.
- `topup_id` (String, optional): Top-up order ID.

### 3.3 Actor Model (`actor`)
- `type` (String, required): Actor category enum:
  - `api_key`: Client Agent authenticating via API Key
  - `client_signer`: Decentralized caller with an on-chain wallet address
  - `admin_user`: Control-plane console administrator or team member
  - `system_worker`: Internal scheduled job or asynchronous reconciliation worker
- `id` (String, required): Identifier of the actor (API Key ID, Signer Address, Member ID, or Worker name)
- `client_ip` (String, required): Caller IPv4/IPv6 address (extracted from trusted reverse-proxy headers)
- `user_agent` (String, optional): HTTP client User-Agent string

---

## 4. Event Taxonomy & Definitions

Audit events are organized into six core domains:

```mermaid
graph TD
    AuditEvents[AgentPay Audit Event Taxonomy]
    AuditEvents --> Traffic[1. Gateway & Traffic Events]
    AuditEvents --> Intent[2. Payment Intent Events]
    AuditEvents --> Settle[3. On-Chain Settlement Events]
    AuditEvents --> Fulfill[4. Upstream Fulfillment Events]
    AuditEvents --> Wallet[5. Wallet Ledger & Top-Up Events]
    AuditEvents --> Admin[6. Admin & Configuration Events]
```

### 4.1 Gateway & Inbound Traffic Events

| Event Type (`event_type`) | Trigger Condition | Core Payload Fields (`data`) |
| :--- | :--- | :--- |
| `gateway.request.received` | Gateway receives an inbound HTTP request and completes initial route matching | `route_id`, `http_method`, `request_path`, `content_length` |
| `gateway.challenge.issued` | Buyer lacks authorization or sufficient balance; gateway returns HTTP 402 Payment Required | `route_id`, `price_base_units`, `asset`, `payee_address`, `challenge_id` |
| `gateway.request.completed` | End-to-end request processing finishes and final HTTP response is sent to buyer | `route_id`, `http_status`, `duration_ms`, `payment_method`, `bytes_sent` |
| `gateway.request.rejected` | Request is rejected due to allowlist/denylist rules, budget caps, auth failure, or rate limiting | `route_id`, `rejection_code`, `rejection_reason`, `http_status` |

### 4.2 Payment Intent Events

| Event Type (`event_type`) | Trigger Condition | Core Payload Fields (`data`) |
| :--- | :--- | :--- |
| `intent.created` | Payment intent is initialized | `amount_base_units`, `asset`, `payment_method`, `config_version_id` |
| `intent.authorized` | Buyer payment payload (signature or wallet authorization) passes verification | `payer_address`, `valid_after`, `valid_before`, `nonce` |
| `intent.completed` | Both settlement and fulfillment succeed; intent is archived as completed | `settlement_status`, `fulfillment_status`, `total_duration_ms` |
| `intent.failed` | Terminal failure caused by authorization rejection, settlement failure, or invalid input | `failure_code`, `failure_message` |
| `intent.expired` | 402 challenge expires before a valid payment payload is received | `challenge_ttl_ms`, `expired_at` |
| `intent.needs_review` | Settlement timeout or ambiguous fulfillment outcome escalates intent to manual review | `review_reason`, `hold_status`, `settlement_status` |

### 4.3 On-Chain Settlement Events

| Event Type (`event_type`) | Trigger Condition | Core Payload Fields (`data`) |
| :--- | :--- | :--- |
| `settlement.submitted` | Settlement transaction is broadcast via RPC or submitted to Facilitator | `network_id`, `asset_address`, `payer_address`, `nonce`, `tx_hash` |
| `settlement.pending` | Transaction is included in a block and awaiting required confirmations | `network_id`, `tx_hash`, `block_number`, `current_confirmations` |
| `settlement.confirmed` | Transaction block reaches the configured safety confirmation threshold | `network_id`, `tx_hash`, `block_number`, `gas_used_base_units` |
| `settlement.failed` | On-chain transaction explicitly reverts, nonce expires, or tx is replaced | `network_id`, `tx_hash`, `revert_reason` |
| `settlement.unknown` | Consecutive RPC timeouts or network disconnects place settlement into unknown state | `network_id`, `tx_hash`, `last_probe_time`, `probe_retry_count` |

### 4.4 Upstream Fulfillment Events

| Event Type (`event_type`) | Trigger Condition | Core Payload Fields (`data`) |
| :--- | :--- | :--- |
| `fulfillment.dispatched` | Gateway establishes connection and dispatches proxied request to upstream service | `upstream_url`, `timeout_ms` |
| `fulfillment.succeeded` | Upstream returns an explicit HTTP 2xx success response | `upstream_status`, `upstream_latency_ms`, `response_body_hash` |
| `fulfillment.failed` | Upstream returns HTTP 4xx or an explicit non-executed error | `upstream_status`, `upstream_latency_ms`, `error_code` |
| `fulfillment.unknown` | Upstream response times out or connection drops abnormally (hold must remain locked) | `timeout_ms`, `elapsed_ms`, `error_message` |

### 4.5 Platform Virtual Wallet & Ledger Events

| Event Type (`event_type`) | Trigger Condition | Core Payload Fields (`data`) |
| :--- | :--- | :--- |
| `wallet.created` | Tenant initializes its primary asset wallet | `wallet_id`, `asset`, `initial_balance` |
| `wallet.hold.created` | Available balance is locked (`Hold`) upon intent creation | `wallet_id`, `amount_base_units`, `expires_at`, `available_balance_after` |
| `wallet.hold.captured` | Upstream fulfillment succeeds and held funds are captured (`Capture`) | `wallet_id`, `hold_id`, `amount_base_units`, `ledger_entry_id` |
| `wallet.hold.released` | Fulfillment fails or is rolled back; held funds return to available balance | `wallet_id`, `hold_id`, `amount_base_units`, `release_reason` |
| `wallet.ledger.appended` | An immutable accounting entry is appended to the ledger | `wallet_id`, `entry_id`, `type`, `amount`, `ref_type`, `ref_id` |
| `wallet.status.changed` | Wallet status transitions (e.g., `active` -> `frozen_arrears`) | `wallet_id`, `old_status`, `new_status`, `reason` |
| `topup.order.created` | Top-up order is created (admin grant, on-chain deposit, or fiat order) | `topup_id`, `wallet_id`, `channel`, `amount_base_units`, `asset` |
| `topup.order.confirmed` | Top-up funds are confirmed and credited to the wallet ledger | `topup_id`, `wallet_id`, `amount_base_units`, `external_ref`, `entry_id` |
| `topup.order.failed` | Top-up order fails or is closed | `topup_id`, `failure_reason` |

### 4.6 Admin & Configuration Events

| Event Type (`event_type`) | Trigger Condition | Core Payload Fields (`data`) |
| :--- | :--- | :--- |
| `admin.adjustment.executed` | Administrator performs a manual balance adjustment (credit/debit/reversal) | `wallet_id`, `amount`, `reason`, `ledger_entry_id`, `operator_id` |
| `admin.config.published` | Administrator publishes a new immutable route/policy snapshot version | `version_id`, `config_hash`, `routes_count`, `policies_count` |
| `admin.config.rolled_back` | Administrator executes a one-click configuration snapshot rollback | `active_version_id`, `target_version_id`, `reason` |
| `admin.apikey.created` | Administrator or system issues a new API Key | `key_id`, `key_prefix`, `project_id`, `scopes` |
| `admin.apikey.revoked` | Administrator revokes a specific API Key | `key_id`, `key_prefix`, `reason` |
| `admin.tenant.updated` | Tenant profile, quota, or status is updated | `target_tenant_id`, `changed_fields` |

---

## 5. Security Redaction & Data Privacy Specification

Because audit events serve as long-term compliance archives and external exports, **recording plaintext passwords, private keys, full API Keys, or personally identifiable information (PII) is strictly prohibited**.

### 5.1 Redaction Rules Matrix

| Field Category | Plaintext Occurrence | Audit Recording Standard & Redaction Algorithm | Severity Level |
| :--- | :--- | :--- | :---: |
| **API Key Credentials** | `Authorization: Bearer apk_live_12345678abcdef...` | **Never store in plaintext**.<br>1. Extract prefix: `key_prefix` (first 8 chars, e.g., `apk_live`)<br>2. Key hash: `key_hash = sha256(raw_key)`<br>3. Reference ID: `api_key_id` | **P0 (Plaintext Prohibited)** |
| **Private Keys & Mnemonics** | Buyer local signer private key | **Physically isolated**. Neither the gateway nor the audit system may ever touch private keys. If accidentally sent in a payload, scrub immediately and raise a security alert. | **P0 (Zero Contact)** |
| **Cryptographic Signatures** | `PAYMENT-SIGNATURE` header | Compact signature string or decomposed `(v, r, s)` may be retained; any non-public client parameters must be stripped. | **P2** |
| **HTTP Request/Response Bodies** | Body payloads exchanged between Agent and upstream | 1. Record only `body_sha256` digest and `content_length` in bytes by default.<br>2. If sampling is enabled, redact sensitive JSON keys (`password`, `secret`, `token`, `credential`, `private_key` -> `[REDACTED]`) and truncate to 2048 chars max. | **P1** |
| **Client IP Address** | Caller IP address | Full IP stored internally for security risk control. In external redacted exports, mask the last octet of IPv4 (`198.51.100.xxx`) or lower 64 bits of IPv6 (`2001:db8::xxxx`). | **P2** |
| **Upstream Error Stack Traces** | HTML or stack traces from upstream 500 / 502 responses | Truncate to at most 1024 characters and scrub database connection strings, passwords, and internal private IPs. | **P1** |

---

## 6. Idempotent Event Persistence & Deduplication Rules

### 6.1 Storage Uniqueness Guarantees
The PostgreSQL table `ap_audit_event` enforces two layers of uniqueness constraints:
1. **Primary Key Uniqueness**: `audit_id` (`event_id`) is the global Primary Key.
2. **Composite Business Idempotency Unique Index**: For replayed business requests, a unique index on `(tenant_id, idempotency_key, event_type)` prevents duplicate events (while allowing administrative events with `idempotency_key IS NULL` to be inserted independently).

```sql
-- Idempotent write guarantee: ignore duplicate event IDs and return success
INSERT INTO ap_audit_event (
    audit_id, tenant_id, operator_id, action, target_type, target_id, before_state, after_state, client_ip, created_at
) VALUES (
    'evt_01j7b8z4m0k9x8y7', 'tenant_01', 'key_01', 'wallet.hold.created', 'Hold', 'hld_01', NULL, '{"amount": 10000}', '192.0.2.1', NOW()
) ON CONFLICT (audit_id) DO NOTHING;
```

### 6.2 Out-of-Order Arrival & Causal Reordering
When asynchronous message queues (such as Kafka, SQS, or internal async buffers) collect audit logs, events may arrive out of order due to network jitter:
- Consumers and query engines sort stably by `timestamp` (microseconds) combined with the entity's internal monotonic sequence (such as `version` or state machine transition index).
- Once a terminal event (`intent.completed`, `intent.failed`) arrives, the audit trace for that intent is locked; any subsequent duplicate probe events are appended solely as read-only probe logs without triggering state callbacks.

---

## 7. Data Retention Lifecycle & Export Specification

### 7.1 Tiered Storage Lifecycle

| Storage Tier | Storage Medium | Retention Period | Query Latency & Use Cases |
| :--- | :--- | :---: | :--- |
| **Hot Tier** | PostgreSQL 15+ (`ap_audit_event`, `ap_request_record`) | **90 Days** | Milliseconds. Real-time console search, recent billing troubleshooting, live alerting. |
| **Warm Tier** | Standard Object Storage (S3 / GCS), Parquet / NDJSON format | **1 Year** | Seconds to minutes. Offline five-way automated reconciliation, monthly operational rollups, financial analytics. |
| **Cold Tier** | Archival Object Storage (Glacier Deep Archive / Coldline) | **7 Years** | Hours. Financial compliance and regulatory audit retention requirements. |

### 7.2 Export Formats & Contracts

The platform provides tenant-scoped bulk export endpoints:
1. **NDJSON Format (Newline Delimited JSON)**:
   - Each line is a self-contained JSON object conforming to the event envelope schema, UTF-8 encoded and Gzip-compressed in transit.
   - File naming convention: `audit_export_{tenant_id}_{start_date}_{end_date}.ndjson.gz`
2. **Flat CSV Format (for Excel / BI)**:
   - Column headers:
     `event_id,event_type,timestamp,tenant_id,project_id,request_id,idempotency_key,intent_id,actor_type,actor_id,client_ip,amount_base_units,asset,status`
   - Complex nested payload data is serialized as a JSON string in the final column, `payload_json`.
