# AgentPay Python / Go Protocol Conformance Report (T2.6 Conformance Report)

> **Report Version**: v2.1.0  
> **Execution Date**: 2026-10-18 (Updated: 2026-10-26)  
> **Test Status**: **100% PASSED**  
> **Environment**: macOS (Darwin arm64), Python 3.12.7 (pytest 9.1.1), Go 1.24  
> **Core Specifications Covered**:
> - [`specs/protocol-consistency-spec.md`](https://github.com/open-agentpay/AgentPay/blob/main/specs/protocol-consistency-spec.md) (T1.3 Unified Protocol Contract)
> - [`specs/conformance/protocol-errors.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/conformance/protocol-errors.json) (Canonical Error Taxonomy & Schema)
> - [`specs/test-vectors/dual-path-protocol.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/test-vectors/dual-path-protocol.json) (Dual-Path Protocol Conformance Test Vectors)
> - [`specs/test-vectors/config-publishing.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/test-vectors/config-publishing.json) (Config Publishing & RCU Snapshot Vectors)
> - [`specs/conformance/state-machine.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/conformance/state-machine.json) (Unified Tri-State Machine Model v2.1.0)

---

## 1. Overview & Testing Objectives

To guarantee 100% behavioral parity between AgentPay's **Python Buyer SDK** (`sdk/python`) and **Cloud-Native Go Gateway** (`services/gateway`) across data-plane and control-plane interactions, **Task T2.6 (Python/Go Conformance Testing)** verifies cross-language equivalence across five core dimensions:

1. **Canonical Error Taxonomy & Wire Envelope**: Verifies 1:1 alignment across all 19 standardized error codes (`specs/conformance/protocol-errors.json`), HTTP status codes, domain classifications, actionable guidance fields, and JSON response envelopes between the Go Gateway and Python SDK.
2. **Dual-Path Protocol Test Vectors**: Executes `TC_PROTO_01` through `TC_PROTO_06` across both languages to verify deterministic arbitration for x402 on-chain challenges, virtual wallet deductions, insufficient-balance handling without silent downgrade, arrears freezes, route payment-method restrictions, and idempotency payload conflicts.
3. **State Machine Transition Matrix & Invariant Guards**: Validates the Go state machine engine (`services/gateway/internal/statemachine`) against the Python state machine across all 35 valid transitions, terminal state immutability, 6 safety invariants (`INV_1` through `INV_6`), and 8 composite user-facing status views.
4. **Configuration Publishing & RCU Degradation Vectors**: Executes `TC_CFG_01` through `TC_CFG_05` across Python and Go to verify static policy/route validation, deterministic SHA-256 snapshot hashing, version rollback integrity, and local cache fail-safe operation when the control plane is unreachable.
5. **Live HTTP Cross-Language Interoperability**: Spawns an independent Go Gateway process alongside a simulated Python upstream backend and issues real HTTP requests from the Python `httpx` client to verify challenge parsing, Bearer Token virtual wallet billing, configuration version header injection, and anti-double-spend / anti-tampering conflict handling end to end.

---

## 2. Cross-Language Test Coverage Matrix

| Test Dimension | Test Suites & Files | Test Cases | Result | Key Verification Points |
| :--- | :--- | :---: | :---: | :--- |
| **Canonical Error Taxonomy** | `services/gateway/internal/x402/conformance_test.go`<br>`sdk/python/tests/unit/test_protocol_conformance_vectors.py` | 19 | **PASS** | Validates HTTP status, Domain, Message, Actionable guidance, and canonical Wire Format Envelope across all 19 Canonical Error Codes |
| **Dual-Path Protocol Vectors** | `specs/test-vectors/dual-path-protocol.json`<br>`services/gateway/internal/x402/conformance_test.go` | 6 | **PASS** | Vector-driven tests `TC_PROTO_01` – `TC_PROTO_06` covering challenges, routing, anti-downgrade, arrears, and idempotency conflicts |
| **State Machine Matrix & Guards** | `services/gateway/internal/statemachine/engine_test.go`<br>`sdk/python/tests/unit/test_state_machine_matrix.py` | 42 | **PASS** | 35 valid transitions, invalid transition rejection, terminal state immutability, dual-rail dispatch guards, unknown submit guards, and composite view mappings |
| **Config Publishing Conformance** | `specs/test-vectors/config-publishing.json`<br>`services/gateway/internal/rcu/snapshot_test.go`<br>`sdk/python/tests/unit/test_config_publishing.py` | 5 | **PASS** | `TC_CFG_01` – `TC_CFG_05` static validation matrix, Canonical SHA-256 fingerprint computation, version rollback, and fail-safe degradation |
| **Live Inter-Process HTTP Interop** | `sdk/python/tests/integration/test_cross_language_conformance.py` | 6 | **PASS** | Real Python client issuing HTTP calls against a standalone Go Gateway process: challenge parsing, wallet deduction, header tracing, idempotency, and fault injection |

---

## 3. Detailed Test Execution & Results

### 3.1 Dual-Path Protocol Vector Results (`specs/test-vectors/dual-path-protocol.json`)

| Case ID | Category | Input Characteristics | Expected Behavior | Go Gateway | Python SDK | Verdict |
| :--- | :--- | :--- | :--- | :---: | :---: | :---: |
| `TC_PROTO_01` | `ONCHAIN_X402` | Anonymous unauthenticated GET request | Returns `402` with `PAYMENT-REQUIRED` header and EIP-3009 payload structure | PASS (402, EIP-3009) | PASS (`from_header` parsed) | **Aligned** |
| `TC_PROTO_02` | `WALLET_PAYMENT` | Request carrying valid API Key | Selects virtual wallet path, acquires Hold reservation, does not issue on-chain challenge | PASS (200, balance deducted) | PASS (wallet path selected) | **Aligned** |
| `TC_PROTO_03` | `WALLET_PAYMENT` | Insufficient virtual wallet balance | Returns `402 ERR_WALLET_INSUFFICIENT_BALANCE`; **strictly omits `PAYMENT-REQUIRED`** | PASS (402, no downgrade header) | PASS (402, auto-downgrade refused) | **Aligned** |
| `TC_PROTO_04` | `WALLET_PAYMENT` | Wallet frozen in arrears (`frozen_arrears`) | Blocks request and returns `403 ERR_WALLET_FROZEN` | PASS (403, blocked) | PASS (403, blocked) | **Aligned** |
| `TC_PROTO_05` | `ROUTING` | `wallet`-only route receives on-chain signature | Blocks request and returns `403 ERR_PAYMENT_METHOD_NOT_ALLOWED` | PASS (403, blocked) | PASS (403, blocked) | **Aligned** |
| `TC_PROTO_06` | `IDEMPOTENCY` | Same idempotency key sent with different payload digest | Blocks request and returns `409 ERR_IDEMPOTENCY_CONFLICT` | PASS (409, blocked) | PASS (409, blocked) | **Aligned** |

### 3.2 Canonical Error Taxonomy Alignment (`specs/conformance/protocol-errors.json`)

Every error response emitted by the Go Gateway or parsed by the Python SDK conforms to the canonical JSON wire envelope:

```json
{
  "error": {
    "code": "ERR_WALLET_INSUFFICIENT_BALANCE",
    "message": "Wallet available balance is insufficient to cover this request.",
    "domain": "WALLET",
    "status": 402,
    "requestId": "req_01j7b8z4m0k9x8y100000001",
    "details": {}
  }
}
```

All 19 canonical error definitions are verified via `TestCanonicalErrorsIntegrityAgainstTaxonomy`:

| Error Code | HTTP Status | Domain | Canonical Message & Actionable Resolution |
| :--- | :---: | :--- | :--- |
| `ERR_PAYMENT_REQUIRED` | `402` | `PROTOCOL` | Resource requires x402 payment authorization. Parse `PAYMENT-REQUIRED` challenge and attach valid `PAYMENT-SIGNATURE`. |
| `ERR_WALLET_INSUFFICIENT_BALANCE` | `402` | `WALLET` | Wallet available balance is insufficient. Top up balance; silent downgrade to on-chain payment is strictly forbidden. |
| `ERR_WALLET_FROZEN` | `403` | `WALLET` | Wallet account is currently frozen or in arrears status. Contact admin or clear arrears balance. |
| `ERR_WALLET_AUTH_INVALID` | `401` | `WALLET` | AgentPay wallet authorization is invalid, expired, mismatched, or already consumed. Obtain a fresh challenge and retry. |
| `ERR_WALLET_AUTH_REQUIRED` | `401` | `WALLET` | Route requires an AgentPay virtual-wallet API key. Send an active API key to receive the wallet price challenge. |
| `ERR_HOLD_EXPIRED` | `408` | `WALLET` | Internal wallet hold reservation expired before upstream completion. Retry request with a fresh hold lease. |
| `ERR_PAYMENT_METHOD_NOT_ALLOWED` | `403` | `ROUTING` | Requested payment method is not permitted by route policy (`wallet`, `onchain`, or `both`). |
| `ERR_POLICY_BUDGET_EXCEEDED` | `403` | `POLICY` | 24-hour rolling or lifetime budget ceiling exhausted. Increase budget cap or await rolling window expiration. |
| `ERR_POLICY_MAX_PER_TX` | `403` | `POLICY` | Single transaction amount exceeds `max_amount_per_request` policy limit. |
| `ERR_POLICY_CIRCUIT_BROKEN` | `429` | `POLICY` | Circuit breaker triggered due to runaway agent loops or high request frequency. Cool down or await probe window. |
| `ERR_API_KEY_INVALID` | `401` | `AUTH` | Provided API Key is invalid, revoked, or expired. Verify Bearer token or issue a new active API Key. |
| `ERR_API_KEY_SCOPE_DENIED` | `403` | `AUTH` | API Key lacks required permission scope (e.g., `gateway:call`) for this operation. |
| `ERR_TENANT_SUSPENDED` | `403` | `TENANT` | Tenant organization account is suspended. Contact platform administrator. |
| `ERR_SIGNATURE_INVALID` | `401` | `CRYPTOGRAPHY` | EIP-712 / EIP-3009 payment signature verification failed or payload tampered. |
| `ERR_NONCE_REPLAY` | `409` | `SETTLEMENT` | Cryptographic nonce has already been consumed or submitted. Generate a fresh 32-byte random nonce. |
| `ERR_IDEMPOTENCY_CONFLICT` | `409` | `IDEMPOTENCY` | Idempotency key collision with mismatched payload hash or in-flight concurrent execution. |
| `ERR_UPSTREAM_TIMEOUT` | `504` | `GATEWAY` | Gateway timed out awaiting upstream API execution. Retry with the exact same `Idempotency-Key` (free replay). |
| `ERR_UPSTREAM_FAILED` | `502` | `GATEWAY` | Upstream service returned 5xx server error. Investigate upstream availability and retry with same `Idempotency-Key`. |
| `ERR_CONFIG_VERSION_EXPIRED` | `409` | `CONFIG` | Route configuration version has been superseded or rolled back during execution. Refresh snapshot and retry. |

### 3.3 Configuration Publishing & RCU Conformance (`specs/test-vectors/config-publishing.json`)

| Case ID | Category | Scenario Description | Expected Outcome | Go RCU Engine | Python Control Plane |
| :--- | :--- | :--- | :--- | :---: | :---: |
| `TC_CFG_01` | `PUBLISHING` | Valid routes & policies serialized and signed with canonical SHA-256 | `is_valid = true`, `target_status = active`, `sha256:` prefix | PASS | PASS |
| `TC_CFG_02` | `VALIDATION` | Route contains malformed upstream URL (`not_a_valid_http_uri`) | Rejected before publish with `INVALID_UPSTREAM_URL` | PASS | PASS |
| `TC_CFG_03` | `VALIDATION` | Policy `max_amount_per_request` (2,000,000) exceeds `rolling_24h_budget_cap` (1,000,000) | Rejected before publish with `MAX_AMOUNT_EXCEEDS_ROLLING_BUDGET` | PASS | PASS |
| `TC_CFG_04` | `ROLLBACK` | Rollback from `v2` to `v1` creates immutable `v3` restoring `v1`'s exact snapshot hash | `new_version_number = 3`, `active_hash == v1_hash` (`ROLLBACK_SUCCESS`) | PASS | PASS |
| `TC_CFG_05` | `DEGRADATION` | Control plane returns `500 UNAVAILABLE`; gateway continues serving from local RCU snapshot | `CONTINUE_SERVICE_WITH_LOCAL_CACHE`, `degradation_alert_logged = true` | PASS | PASS |

### 3.4 State Machine Matrix, Safety Invariants & Composite Views (`specs/conformance/state-machine.json`)

The `services/gateway/internal/statemachine` engine and Python state machine were validated against `specs/conformance/state-machine.json` (v2.1.0):

1. **Valid Transition Matrix (35 Transitions)**:
   - `PaymentIntent` (8 states, 13 valid transitions)
   - `Settlement` (8 states, 14 valid transitions covering both on-chain and virtual wallet paths)
   - `Fulfillment` (5 states, 8 valid transitions)
   - All 35 `(from, event, to)` state transitions produce identical target states in Go and Python, and all unlisted transitions are deterministically rejected.
2. **Six Formal Safety Invariants (`INV_1` – `INV_6`)**:
   - `INV_1_DISPATCH_GUARD`: Fulfillment dispatch is forbidden unless `(path == onchain AND settlement == confirmed)` or `(path == wallet AND settlement == held)`.
   - `INV_2_UNKNOWN_SUBMIT_GUARD`: When on-chain `settlement == unknown`, re-submitting a transaction is strictly prohibited; only status polling is permitted.
   - `INV_3_TERMINAL_IMMUTABLE`: Terminal states (`payment_intent: rejected/expired/completed/failed`, `settlement: confirmed/released/failed`, `fulfillment: succeeded/failed`) reject all subsequent mutation events.
   - `INV_4_UPSTREAM_5XX_UNKNOWN`: Upstream `5xx` or network timeout transitions `fulfillment` to `unknown`, never directly to `failed`.
   - `INV_5_WALLET_UNKNOWN_HOLD_GUARD`: When `path == wallet` and `fulfillment == unknown`, the wallet hold must remain in `held` state—neither automatic capture nor automatic release is permitted until recovery resolves the outcome.
   - `INV_6_NO_AUTO_DOWNGRADE`: When a virtual wallet's balance is insufficient, the gateway returns `402 ERR_WALLET_INSUFFICIENT_BALANCE` immediately without silently downgrading to an on-chain challenge.
3. **Composite Status Views**:

| `Settlement` State(s) | `Fulfillment` State(s) | Derived Composite Display State |
| :--- | :--- | :--- |
| `held` | `not_started`, `in_progress` | `WALLET_HOLD_PROCESSING` |
| `unknown`, `pending`, `submitted` | `not_started` | `SETTLEMENT_IN_PROGRESS` |
| `confirmed` | `in_progress` | `FULFILLMENT_IN_PROGRESS` |
| `confirmed` | `succeeded` | `COMPLETED` |
| `confirmed` | `failed` | `SETTLED_FULFILLMENT_FAILED_REFUND_REQUIRED` |
| `confirmed` | `unknown` | `SETTLED_FULFILLMENT_UNKNOWN_NEEDS_REVIEW` |
| `released` | `failed` | `RELEASED_UNBILLED` |
| `failed` | `not_started` | `PAYMENT_FAILED` |

---

## 4. Behavioral Discrepancies Identified & Resolved

During cross-language conformance testing, four behavioral discrepancies between the Go and Python implementations were identified and resolved:

1. **Canonical Error Taxonomy Naming & Backward-Compatible Aliases**:
   - *Discrepancy*: The Go Gateway previously defined `ERR_NONCE_ALREADY_USED` and `ERR_UPSTREAM_BAD_GATEWAY`, whereas the specification defines `ERR_NONCE_REPLAY` (409, `SETTLEMENT`) and `ERR_UPSTREAM_FAILED` (502, `GATEWAY`), and lacked `ERR_HOLD_EXPIRED`, `ERR_CONFIG_VERSION_EXPIRED`, `ERR_WALLET_AUTH_INVALID`, and `ERR_WALLET_AUTH_REQUIRED`.
   - *Resolution*: Updated `services/gateway/internal/x402/errors.go` to register the complete canonical error taxonomy with actionable metadata fields while retaining backward-compatible symbol aliases for internal callers.
2. **Dynamic Challenge Network Selection & Fallback**:
   - *Discrepancy*: The Go Gateway previously hardcoded `"eip155:84532"` when issuing 402 challenges instead of reading `route.Network` from the route definition.
   - *Resolution*: Added the `Network` field to `rcu.RouteConfig` and `config.RouteDefinition`, prioritizing `route.Network` when issuing challenges and falling back to global `cfg.Settlement.DefaultNetwork` (e.g., `eip155:421614` Arbitrum Sepolia) when unset.
3. **Case-Insensitive Headers & EVM Address Normalization**:
   - *Discrepancy*: Case-sensitive dictionary lookups in Python caused intermittent misses on `x-agentpay-config-version`, and EVM address comparisons normalized to lowercase in Python while preserving EIP-55 mixed-case checksums in Go.
   - *Resolution*: Standardized both E2E tests and client SDK logic on RFC 7230 case-insensitive header extraction and case-insensitive hexadecimal Ethereum address comparison.
4. **Pre-Configured Test Wallets & Isolation**:
   - *Resolution*: Built active, insufficient-balance, and `frozen_arrears` preset test accounts into `services/gateway/internal/server` default initialization for out-of-the-box local development and cross-language E2E testing.

---

## 5. Verification Commands & Acceptance Conclusion

You can reproduce the complete cross-language conformance suite locally with:

```bash
# 1. Validate JSON schemas and cross-language test vectors
make vectors

# 2. Run Python unit, state-machine, and live cross-language integration tests
make test

# 3. Run Go Gateway unit, RCU snapshot, and x402 conformance tests
make gateway-test
```

Full-repository verification summary:
- **Python Tests**: 373+ tests (including 6 live cross-language E2E HTTP interoperability tests) — **100% PASS**.
- **Go Tests**: All unit, state machine matrix, RCU snapshot, and x402 conformance tests — **100% PASS**.
- **Test Vector Validation**: All cross-language JSON test vectors under `specs/test-vectors` and `specs/conformance` passed.
- **Code Quality**: `ruff check` and `ruff format --check` passed with zero warnings.

**All acceptance criteria for Task T2.6 (Python/Go Conformance Testing) are fully satisfied.**
