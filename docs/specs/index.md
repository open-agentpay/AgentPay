# Specifications Overview

AgentPay protocols, state machines, data models, and security designs are formally defined as specifications, backed by executable cross-language conformance test vectors (verified identically against Python and Go implementations).

::: info Single Source of Truth
Specifications on this site are automatically synchronized from the repository root [`specs/`](https://github.com/open-agentpay/AgentPay/tree/main/specs).
Machine-readable JSON Schemas and test vectors are maintained at [`specs/conformance`](https://github.com/open-agentpay/AgentPay/tree/main/specs/conformance) and [`specs/test-vectors`](https://github.com/open-agentpay/AgentPay/tree/main/specs/test-vectors).
:::

## Core Contracts & Architecture

| Topic | Description |
|---|---|
| [Payment State Machine](./state-machine) | Intent / Settlement / Fulfillment tri-state orthogonal machine |
| [Wallet & Ledger](./wallet-ledger) | Hold / Capture / Release, append-only double-entry ledger & reconciliation |
| [Protocol Consistency](./protocol-consistency) | Request headers, normalized error schemas & dual-path dispute resolution |
| [Config Publishing](./config-publishing) | Drafts, immutable snapshot publishing, RCU hot-swapping & rollbacks |
| [Audit Events](./audit-events) | Event envelope, data sanitization, WORM retention & CSV/JSON export |
| [Agent Audit & Identity](./agent-audit) | Cryptographic identity extraction, observed vs reported dimensions, kill-switch |
| [Multitenant Data Model](./multitenant-data-model) | PostgreSQL 16 schema, row-level isolation & foreign key topology |
| [Architecture Specification](./architecture) | End-to-end dual-plane architecture, component boundaries & data flows |
| [Gas & Settlement Models](./gas-models) | EIP-3009 gasless settlement economics & Facilitator batching |
| [Security Model](./security-model) | Trust boundaries, STRIDE threat catalog & firewall mitigations |

## Architecture Decision Records (ADR)

| ADR | Decision Summary |
|---|---|
| [ADR-001: x402 V2 Reuse Strategy](./adr/001) | Layered adaptation with custom Pydantic V2 firewall core vs. raw third-party SDK |
| [ADR-002: Database Upgrade to PostgreSQL](./adr/002) | Upgrading from Phase 0 SQLite to PostgreSQL 16 with row-level `FOR UPDATE` locking & `JSONB` snapshots |

## Conformance & Security Reports

| Report | Coverage Summary |
|---|---|
| [Python / Go Protocol Conformance Report](./reports/protocol-conformance-report) | Cross-language parity across 19 canonical errors, dual-path vectors, 35 state transitions & live HTTP interop |
| [Multi-Tenant Isolation & Penetration Report](./reports/tenant-isolation-test-report) | IDOR/BOPLA, vertical RBAC escalation, path/payload spoofing, RCU routing & wallet ledger isolation |
