---
title: Glossary
description: Essential terminology and definitions across AgentPay and AI Agent payment architectures.
---

# Glossary

### Agent API Key
A scoped credential issued by a tenant to an agent instance formatted as `apk_live_<hex>`. The gateway resolves the associated wallet, policy, and tenant boundaries via this key.

### Financial Firewall
An out-of-band proxy layer placed between autonomous agents and paid upstream services that enforces transaction caps, rolling budgets, payee whitelists, and circuit breaking.

### Hold (Balance Reservation)
An atomic reservation on a virtual wallet balance placed before an upstream request is forwarded. If the call succeeds, the hold is captured; if the call fails with a deterministic client error, the hold is released.

### Hold TTL (Time-To-Live)
The maximum duration a balance hold remains active before being eligible for automated cleanup. AgentPay defaults to `upstream_timeout + 60s` (default 120 seconds).

### Capture (Settlement Execution)
The irreversible transition converting held funds into settled revenue once upstream delivery has been confirmed.

### x402 Protocol
The open machine-readable payment protocol extending HTTP 402 Payment Required for automated web micro-payments.

### Facilitator
The off-chain service node responsible for verifying signatures, sponsoring miner gas fees, and batching on-chain settlements.
