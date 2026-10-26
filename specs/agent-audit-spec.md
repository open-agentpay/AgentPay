# Agent Identity & Behavior Audit Specification (T1.8 Specification)

> **Version**: v1.0.0 (T1.8 Specification: Agent Identity & Behavior Audit)  
> **Status**: **Approved Specification**  
> **Scope**: AgentPay Go Gateway Data Plane, Control Plane, Audit Stream & Management Console  
> **Prerequisites**: [specs/multitenant-data-model.md](./multitenant-data-model.md), [specs/audit-events-spec.md](./audit-events-spec.md)

---

## 1. Goals & Boundaries

Three distinct requirements map to three distinct system capabilities and must not be conflated:

| Requirement | Nature | Latency Requirement |
|---|---|---|
| Visibility & Troubleshooting: Tenants inspect what their Agents did | Querying & timeline reconstruction | Near real-time (short delay acceptable) |
| Risk Control: Detect and block runaway Agents | **Enforcement**, not post-hoc auditing | **Synchronous, in the request path** |
| Compliance & Retention: Post-incident traceability and export | Integrity, retention, and export | Durable and verifiable |

> [!IMPORTANT]
> **Boundary Statement (Published in Public Documentation)**  
> The gateway can only audit an Agent's **observable behaviors** (requests, payments, policy decisions). It cannot inspect the Agent's internal reasoning chain or prove its subjective intent.

---

## 2. Core Design Principles

1. **One Event Stream, Three Consumers**: All behaviors are emitted as canonical audit events. Visibility queries, anomaly detection, and compliance exports consume the same event stream rather than maintaining separate logs.
2. **Enforcement Never Depends on the Audit Pipeline**: Budget limits, rate limits, allowlists, and kill-switch checks are evaluated **synchronously in the request path** using local and real-time counters—never waiting for asynchronous log analysis before blocking.
3. **Strict Separation of Observed vs. Declared Fields**:
   - What the gateway directly observes (`observed`) is objective fact (e.g., `agent_id` bound to the API Key, actual latency, upstream status code, deducted amount).
   - What the Agent self-reports via request headers (`declared`, such as `session_id`, `task_id`, `tool_name`) is untrusted, used solely for correlation and display, and **strictly prohibited from authorization or billing decisions**.
4. **Metadata Only by Default**: Request and response bodies are excluded from audit logs by default. Payload sampling must be explicitly enabled, redacted, assigned a separate retention period, and access-restricted.
5. **Append-Only**: Audit events cannot be modified or deleted. Corrections are recorded by appending new audit events.

---

## 3. Agent Entity Model

| Field | Type | Description & Security Constraints |
|---|---|---|
| `agent_id` | string (PK) | Format: `agt_...`, primary key |
| `tenant_id` | string (FK) | Tenant ownership; strict isolation boundary |
| `project_id` | string (FK) | Project ownership |
| `name` | string | Display name |
| `status` | string | Enum: `active` / `suspended` / `revoked` |
| Key Binding | Association Table | One Agent may hold multiple API Keys; each API Key must belong to exactly one Agent |
| Policy Reference | JSONB / Association | Budget, rate limits, route allowlist, and risk rules (can inherit project defaults) |

### Core Runtime Constraints
- **Single Source of Truth for Identity**: Every inbound request must resolve to an `agent_id` determined strictly by its cryptographic credential (API Key / signature)—**never from client-supplied request headers**.
- **One-Click Kill Switch**: When an Agent's status is set to `suspended` or its API Key is `revoked`, the gateway must reject subsequent requests within seconds (via instant invalidation notifications or RCU refresh); in-flight requests complete or release their holds according to existing state machine rules.

---

## 4. Audit Event Schema Specification

### 4.1 Common Field Provenance Specification

| Field | Provenance | Type & Description |
|---|---|---|
| `event_id` | Observed (`observed`) | Globally unique UUID/CUID serving as the write idempotency key |
| `event_type` | Observed (`observed`) | Event enum (see Section 4.2) |
| `occurred_at` | Observed (`observed`) | High-precision gateway timestamp (UTC) |
| `tenant_id`, `project_id`, `agent_id` | Observed (`observed`) | Resolved from the API Key; tamper-proof |
| `request_id` | Observed (`observed`) | Unified inbound trace ID assigned by the gateway |
| `route_id`, `config_version` | Observed (`observed`) | Matched route ID and active configuration fingerprint |
| `actor` | Observed (`observed`) | `agent` / `admin` / `system` |
| `session_id`, `task_id`, `parent_request_id`, `tool_name` | **Declared (`declared`)** | Self-reported by the Agent; strictly length-bounded (max 128 chars) and character-filtered against injection; **completely optional** |
| Correlation IDs | Observed (`observed`) | `payment_intent_id`, `settlement_id`, `fulfillment_id`, `hold_id` |
| `prev_hash`, `hash` | Observed (`observed`) | Optional hash-chain tamper-evidence fingerprint (reserved interface) |

### 4.1.1 Declared Fields Default Handling & Security Invariants

Declared fields are self-reported by clients via optional headers such as `X-Session-ID`, `X-Task-ID`, and `X-Tool-Name`.

| Processing Dimension | Client Supplies Declared Header | Client Omits Declared Header (Default) |
|---|---|---|
| **Gateway Data Plane** | Whitelist-filtered and truncated to 128 characters before storing in the event | Extracted as `""`; **request proceeds normally without error** |
| **Billing & Deductions** | **Strictly excluded** from all authorization or billing decisions | **Completely unaffected**; atomic transfer executes against the API Key's bound wallet |
| **Risk Control & Circuit Breaking** | Synchronously enforces per-tx cap, 24h budget, and Kill Switch | **Completely unaffected**; internal gateway counters enforce limits strictly |
| **Console Presentation** | Supports multi-call folding, aggregation, and session auditing by `session_id` | Grouped under "All Sessions" and displayed as independent atomic requests in chronological order |
| **Timeline Badges** | Displays green `[Gateway Observed]` + yellow `[Self-Reported]` badges | Displays only the green `[Gateway Observed]` badge with zero loss of fidelity |

### 4.2 Canonical Event Types

| Type | Domain | Meaning & Included Payload |
|---|---|---|
| `agent.request` | Request | Request arrival and outcome (HTTP status code, latency in ms, upstream payload size, request fingerprint hash) |
| `agent.policy_decision` | Policy | Policy verdict: `allow` / `deny`, matched rule ID, rejection reason code, config version |
| `agent.spend` | Payment | Fund operations: hold, capture, release, on-chain settlement (amount, asset, payment method) |
| `agent.anomaly` | Risk | Risk rule triggered: rule ID, safety threshold, observed metric value |
| `agent.control` | Control | Administrative action: suspend, resume, key revoke/rotate, budget adjustment (operator and reason) |
| `agent.export` | Compliance | Audit data export operation (exporter, filter criteria, exported row count) |

---

## 5. Risk Control (Enforcement Layer)

### 5.1 v1 Synchronous Policy Enforcement
- **Per-Agent Independent Budgets**: Configurable 24-hour rolling window and monthly total spending caps per Agent, directly integrated with virtual wallet Holds.
- **Per-Agent Rate Limiting**: Fine-grained QPS/RPM limits per Agent to prevent infinite loops or high-frequency request storms.
- **Route Allowlist**: Strictly restricts an Agent to calling only authorized upstream APIs.
- **Max Per Transaction (`Max Per Tx`)**: Caps the maximum token expenditure for any single interaction.
- **Sub-Second Kill Switch**: When an administrator suspends an Agent or revokes an API Key, the gateway invalidates its cache immediately and blocks inbound requests within milliseconds.

### 5.2 v1.x Anomaly Detection & Alerting
- **Behavioral Anomaly Rules**: Traffic spikes, spend-rate surges, high-frequency consecutive denials, first-time access to rare routes, and unusual off-hours activity.
- **Alert-Only by Default**: Prevents false positives from interrupting production workflows; automatic blocking requires explicit administrator opt-in and a configured cooldown period.
- **Notification Channels**: Web Console notifications and Webhook callback pushes.

---

## 6. Visibility (Query Layer)

1. **Agent Overview & Directory**: Status, recent call volume, total spend, and policy denial rate.
2. **Behavior Timeline**: Displays the event stream chronologically and supports grouping workflow sessions by declared `session_id` / `task_id`.
3. **Multi-Dimensional Filtering**: Time range, target route, HTTP status code, policy decision outcome, payment method, and event type.
4. **Drill-Down Inspection**: One-click drill-down from a single request to its associated `payment_intent`, `settlement`, `hold`, `fulfillment`, and exact policy version evaluated.
5. **Multi-Tenant Permission Boundary**: 100% invisible across tenants; any cross-tenant administrative operation must emit an independent high-severity audit event.

---

## 7. Compliance Retention & Export

1. **Append-Only Integrity**: Audit tables permit only `INSERT` operations, with `UPDATE` and `DELETE` privileges revoked at the database level.
2. **Transactional Outbox**: Events involving fund movements (`Hold`/`Capture`/`Release`) must be written atomically within the same database transaction as the ledger entry, guaranteeing that every deduction produces an audit record.
3. **Tiered Retention**:
   - Metadata events: Long-term retention (1+ year by default).
   - Payload samples (if explicitly enabled by the tenant): Short-term retention (7–30 days by default), governed by independent redaction policies and access controls.
4. **Secure Export**: Restricted to tenant administrators (CSV / JSONL formats); sensitive fields are automatically redacted prior to export, and every export operation is mandatorily recorded as an `agent.export` event.

---

## 8. Threats & Risk Mitigations

| Threat Scenario | Risk Level | Gateway Mitigation Mechanism |
|---|---|---|
| Spoofing `agent_id` via request headers | **Critical** | Gateway forcibly strips client-supplied `X-Agent-ID` headers and resolves identity solely from the API Key |
| Log injection via declared fields | **High** | Declared fields enforce character whitelisting and a 128-character limit, truncating or neutralizing malformed input |
| Credential leakage / Agent impersonation | **High** | Mitigated via IP allowlists, per-tx caps, and instant one-click Kill Switch |
| Propagation delay after suspension | **High** | Suspension and revocation propagate via in-memory RCU instant notifications rather than waiting for local TTL expiry |
| Cross-tenant unauthorized audit queries | **Critical** | Data access layer forcibly injects `tenant_id` predicates backed by Row-Level Security (RLS) |
