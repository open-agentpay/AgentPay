# Specifications Overview

AgentPay protocols, state machines, data models, and security designs are formally defined as specifications, backed by executable cross-language conformance test vectors (verified identically against Python and Go implementations).

::: info Single Source of Truth
Specifications on this site are automatically synchronized from the repository root [`specs/`](https://github.com/open-agentpay/AgentPay/tree/main/specs).
Machine-readable JSON Schemas and test vectors are maintained at [`specs/conformance`](https://github.com/open-agentpay/AgentPay/tree/main/specs/conformance) and [`specs/test-vectors`](https://github.com/open-agentpay/AgentPay/tree/main/specs/test-vectors).
:::

| Topic | Description |
|---|---|
| [Payment State Machine](./state-machine) | Intent / Settlement / Fulfillment tri-state orthogonal machine |
| [Wallet & Ledger](./wallet-ledger) | Hold / Capture / Release, append-only double-entry ledger & reconciliation |
| [Protocol Consistency](./protocol-consistency) | Request headers, normalized error schemas & dual-path dispute resolution |
| [Config Publishing](./config-publishing) | Drafts, immutable snapshot publishing, RCU hot-swapping & rollbacks |
| [Audit Events](./audit-events) | Event envelope, data sanitization, WORM retention & CSV/JSON export |
| [Agent Audit & Identity](./agent-audit) | Cryptographic identity extraction, observed vs reported dimensions, kill-switch |
| [Multitenant Data Model](./multitenant-data-model) | PostgreSQL 16 schema, row-level isolation & foreign key topology |
| [Security Model](./security-model) | Trust boundaries, STRIDE threat catalog & firewall mitigations |
