# AgentPay Python / Go Protocol Conformance Report (T2.6 Conformance Report)

> **Report Version**: v2.0.0  
> **Execution Date**: 2026-10-18  
> **Test Status**: **100% PASSED**  
> **Environment**: macOS (Darwin arm64), Python 3.12.7 (pytest 9.1.1), Go 1.24  
> **Core Specifications Covered**:
> - [`specs/protocol-consistency-spec.md`](https://github.com/open-agentpay/AgentPay/blob/main/specs/protocol-consistency-spec.md) (T1.3 Unified Protocol Contract)
> - [`specs/conformance/protocol-errors.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/conformance/protocol-errors.json) (Canonical Error Taxonomy & Schema)
> - [`specs/test-vectors/dual-path-protocol.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/test-vectors/dual-path-protocol.json) (Dual-Path Protocol Conformance Test Vectors)
> - [`specs/conformance/state-machine.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/conformance/state-machine.json) (Unified State Machine Model)

---

## 1. Overview & Testing Objectives

To guarantee 100% behavioral parity between AgentPay's **Python Buyer SDK** and **Cloud-Native Go Gateway** across data-plane and control-plane interactions, **Task T2.6 (Python/Go Conformance Testing)** verifies cross-language equivalence across four dimensions:

1. **Canonical Error Taxonomy & Wire Envelope**: Verifies 1:1 alignment across all 17 standardized error codes, HTTP status codes, domain classifications, and JSON response envelopes between the Go Gateway and Python client.
2. **Dual-Path Protocol Test Vectors**: Executes `TC_PROTO_01` through `TC_PROTO_06` across both languages to verify deterministic arbitration for x402 on-chain challenges, virtual wallet deductions, insufficient-balance handling without silent fallback, arrears freezes, and idempotency conflicts.
3. **State Machine Transition Matrix & Invariant Guards**: Implements the state machine engine in Go and verifies 100% parity with Python across transition rules, terminal state immutability, dispatch guards (`INV_1_DISPATCH_GUARD`), unknown re-submission guards (`INV_2_UNKNOWN_SUBMIT_GUARD`), and composite view derivation.
4. **Live HTTP Cross-Language Interoperability**: Spawns an independent Go Gateway process alongside a simulated Python upstream backend and issues real HTTP requests from the Python `httpx` client to verify challenge parsing, Bearer Token virtual wallet billing, configuration version header injection, and anti-double-spend / anti-tampering conflict handling end to end.

---

## 2. Cross-Language Test Coverage Matrix

| Test Dimension | Test Suites & Files | Test Cases | Result | Key Verification Points |
|---|---|---|---|---|
| **Full Error Code Alignment** | `gateway-go/internal/x402/conformance_test.go`<br>`python/tests/unit/test_protocol_conformance_vectors.py` | 18 | **PASS** | Validates HTTP status, Domain, Message, and canonical Wire Format Envelope across all 17 Canonical Error Codes |
| **Dual-Path Protocol Vectors** | `specs/test-vectors/dual-path-protocol.json`<br>`gateway-go/internal/x402/conformance_test.go` | 6 | **PASS** | Vector-driven tests `TC_PROTO_01` – `TC_PROTO_06` covering challenges, routing, anti-downgrade, arrears, and idempotency conflicts |
| **State Machine Matrix & Guards** | `gateway-go/internal/statemachine/engine_test.go`<br>`python/tests/unit/test_state_machine_matrix.py` | 42 | **PASS** | 35 valid transitions, invalid transition rejection, terminal state immutability, dual-rail dispatch guards, unknown submit guards, and composite view mappings |
| **Config Publishing Conformance** | `specs/test-vectors/config-publishing.json`<br>`gateway-go/internal/rcu/snapshot_test.go`<br>`python/tests/unit/test_config_publishing.py` | 5 | **PASS** | `TC_CFG_01` – `TC_CFG_05` static validation matrix, Canonical SHA-256 fingerprint computation, version rollback, and fail-safe degradation |
| **Live Inter-Process HTTP Interop** | `python/tests/integration/test_cross_language_conformance.py` | 6 | **PASS** | Real Python client issuing HTTP calls against a standalone Go Gateway process: challenge parsing, wallet deduction, header tracing, idempotency, and fault injection |

---

## 3. Detailed Test Execution & Results

### 3.1 Dual-Path Protocol Vector Results

| Case ID | Category | Input Characteristics | Expected Behavior | Go Gateway | Python SDK | Verdict |
|---|---|---|---|---|---|---|
| `TC_PROTO_01` | ONCHAIN_X402 | Anonymous unauthenticated GET request | Returns 402 with `PAYMENT-REQUIRED` header and EIP-3009 payload structure | PASS (402, EIP-3009) | PASS (`from_header` parsed) | **Aligned** |
| `TC_PROTO_02` | WALLET_PAYMENT | Request carrying valid API Key | Selects virtual wallet path, acquires Hold reservation, does not issue on-chain challenge | PASS (200, balance deducted) | PASS (wallet path selected) | **Aligned** |
| `TC_PROTO_03` | WALLET_PAYMENT | Insufficient virtual wallet balance | Returns 402 `ERR_WALLET_INSUFFICIENT_BALANCE`; **strictly omits `PAYMENT-REQUIRED`** | PASS (402, no downgrade header) | PASS (402, auto-downgrade refused) | **Aligned** |
| `TC_PROTO_04` | WALLET_PAYMENT | Wallet frozen in arrears (`frozen_arrears`) | Blocks request and returns 403 `ERR_WALLET_FROZEN` | PASS (403, blocked) | PASS (403, blocked) | **Aligned** |
| `TC_PROTO_05` | ROUTING | `wallet`-only route receives on-chain signature | Blocks request and returns 403 `ERR_PAYMENT_METHOD_NOT_ALLOWED` | PASS (403, blocked) | PASS (403, blocked) | **Aligned** |
| `TC_PROTO_06` | IDEMPOTENCY | Same idempotency key sent with different payload digest | Blocks request and returns 409 `ERR_IDEMPOTENCY_CONFLICT` | PASS (409, blocked) | PASS (409, blocked) | **Aligned** |

### 3.2 Full Error Taxonomy Alignment (17 Error Codes)

Verified automatically via `TestCanonicalErrorsIntegrityAgainstTaxonomy` against `specs/conformance/protocol-errors.json` and the Go Gateway's `CanonicalErrorRegistry`:

- `ERR_PAYMENT_REQUIRED` (402, PROTOCOL)
- `ERR_WALLET_INSUFFICIENT_BALANCE` (402, WALLET)
- `ERR_WALLET_FROZEN` (403, WALLET)
- `ERR_PAYMENT_METHOD_NOT_ALLOWED` (403, ROUTING)
- `ERR_POLICY_BUDGET_EXCEEDED` (403, POLICY)
- `ERR_POLICY_MAX_PER_TX` (403, POLICY)
- `ERR_POLICY_CIRCUIT_BROKEN` (429, POLICY)
- `ERR_API_KEY_INVALID` (401, AUTH)
- `ERR_API_KEY_SCOPE_DENIED` (403, AUTH)
- `ERR_TENANT_SUSPENDED` (403, TENANT)
- `ERR_SIGNATURE_INVALID` (401, CRYPTOGRAPHY)
- `ERR_NONCE_REPLAY` (409, SETTLEMENT)
- `ERR_IDEMPOTENCY_CONFLICT` (409, IDEMPOTENCY)
- `ERR_HOLD_EXPIRED` (408, WALLET)
- `ERR_UPSTREAM_TIMEOUT` (504, GATEWAY)
- `ERR_UPSTREAM_FAILED` (502, GATEWAY)
- `ERR_CONFIG_VERSION_EXPIRED` (409, CONFIG)

All 17 error codes achieve 100% cross-language parity in HTTP status, domain classification, and wire format envelope.

### 3.3 State Machine Matrix & Safety Guard Alignment

The `gateway-go/internal/statemachine` engine was validated against `specs/conformance/state-machine.json`:
1. **Valid Transition Matrix**: All 35 `(from, event, to)` state transitions produce identical target states in Go and Python.
2. **Terminal Immutability**: `settlement: confirmed/failed/released`, `fulfillment: succeeded/failed`, and `payment_intent: settled/failed` strictly reject any subsequent mutation events.
3. **Dual-Rail Dispatch Guard (`INV_1_DISPATCH_GUARD`)**:
   - On-chain path: Requires `settlement == confirmed` before dispatching upstream fulfillment.
   - Wallet path: Requires `settlement == held` (quota frozen) before dispatching upstream fulfillment.
4. **Unknown Re-Submission Guard (`INV_2_UNKNOWN_SUBMIT_GUARD`)**:
   - Strictly forbids blind re-submission when `settlement == unknown`; submission is permitted only from `not_started`.
5. **Composite Views**: All 9 state combinations map to identical user-facing display states (`WALLET_HOLD_PROCESSING`, `SETTLEMENT_IN_PROGRESS`, `FULFILLMENT_IN_PROGRESS`, `COMPLETED`, `SETTLED_FULFILLMENT_FAILED_REFUND_REQUIRED`, `SETTLED_FULFILLMENT_UNKNOWN_NEEDS_REVIEW`, `RELEASED_UNBILLED`, `PAYMENT_FAILED`) across Go and Python.

---

## 4. Behavioral Discrepancies Identified & Resolved

During conformance testing, the following discrepancies were identified and fixed:

1. **Canonical Error Taxonomy Naming & Backward-Compatible Aliases**:
   - *Discrepancy*: The Go Gateway previously defined `ERR_NONCE_ALREADY_USED` and `ERR_UPSTREAM_BAD_GATEWAY`, whereas the specification defines `ERR_NONCE_REPLAY` (409, SETTLEMENT) and `ERR_UPSTREAM_FAILED` (502, GATEWAY), and lacked `ERR_HOLD_EXPIRED` and `ERR_CONFIG_VERSION_EXPIRED`.
   - *Fix*: Updated `gateway-go/internal/x402/errors.go` to include all 17 canonical error codes and actionable metadata fields while retaining backward-compatible aliases for existing modules.
2. **Dynamic Challenge Network Selection & Fallback**:
   - *Discrepancy*: The Go Gateway previously hardcoded `"eip155:84532"` when issuing 402 challenges instead of reading `route.Network` from the route definition.
   - *Fix*: Added the `Network` field to `rcu.RouteConfig` and `config.RouteDefinition`, prioritizing `route.Network` when issuing challenges and falling back to global `cfg.Settlement.DefaultNetwork` (e.g., `eip155:421614` Arbitrum Sepolia) when unset.
3. **Case-Insensitive Headers & EVM Address Normalization**:
   - *Discrepancy*: Case-sensitive dictionary lookups in Python caused intermittent misses on `x-agentpay-config-version`, and EVM address comparisons normalized to lowercase in Python while preserving mixed case in Go.
   - *Fix*: Standardized both E2E tests and client SDK logic on RFC 7230 case-insensitive header extraction and case-insensitive Ethereum address comparison.
4. **Pre-Configured Test Wallets & Isolation**:
   - *Improvement*: Built active, insufficient-balance, and `frozen_arrears` preset test accounts into `gateway-go/internal/server` default initialization for out-of-the-box local development and cross-language E2E testing.

---

## 5. Acceptance Conclusion

Full-repository verification results:
- **Python Tests**: 373 tests (including 6 live cross-language E2E tests) — 100% PASS.
- **Go Tests**: All unit, state machine matrix, and integration tests — 100% PASS.
- **Test Vector Validation**: All cross-language JSON test vectors under `specs/test-vectors` passed.
- **Code Quality**: `ruff check` and `ruff format --check` passed with zero warnings.

**All acceptance criteria for Task T2.6 (Python/Go Conformance Testing) are fully satisfied.**
