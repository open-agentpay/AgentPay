# Project Overview

::: warning Experimental Project
AgentPay is in active development and has **not** undergone an independent third-party security audit. It is intended for testnet and sandbox environments only. Do not connect it to production mainnet funds.
:::

## What is AgentPay?

**AgentPay** is an open-source **financial firewall and autonomous payment gateway** for AI Agents.

As LLM agents call external tools, APIs and data sources (Tool Calling, MCP, LangChain, CrewAI…), more and more of those resources are paid. The x402 protocol answers the question *"how does an agent pay for an HTTP resource?"* AgentPay answers the harder question that comes right after it:

> **Should this agent be allowed to pay, how much, to whom, and what happens when something goes wrong?**

The guiding principle is simple: **an LLM may *propose* a payment, but it can never *authorize* one.** Every payment passes through deterministic, non-LLM code that enforces budgets and policies before any money moves.

### Who is it for?

| You are… | AgentPay gives you… |
| --- | --- |
| **An agent developer** (buyer) | A drop-in Python client that handles `HTTP 402`, enforces local spending limits, and never lets the model touch a private key. |
| **An API / tool provider** (seller) | A reverse-proxy gateway that turns any existing HTTP API into a paid API with a YAML or console-managed route — no changes to your upstream code. |
| **A platform / enterprise team** | Multi-tenant isolation, RBAC, per-agent API keys, a virtual wallet, an audit timeline and a one-click kill switch. |

---

## The Problems It Solves

| Risk | What goes wrong | How AgentPay mitigates it |
| --- | --- | --- |
| **Runaway loops** | A buggy tool-calling loop re-pays the same API hundreds of times in seconds. | Rolling 24h budget, per-request cap, request-rate limits and a loop-detecting **circuit breaker**. |
| **Prompt injection & hallucination** | A poisoned prompt convinces the agent to pay an attacker or an inflated price. | Payee / network / asset **whitelists** and a hard `max_per_tx` cap, evaluated outside the model. |
| **Key exposure** | Private keys or master secrets leak through the prompt context. | Wallet path: the agent holds only a revocable API key. On-chain path: the key stays in the buyer's local signer / KMS and is never sent to the gateway. |
| **Double charging** | A timeout is mistaken for a failure, the request is retried, and the buyer pays twice. | A **three-state model** (intent / settlement / fulfillment), idempotency keys, and a rule that *timeout ≠ failure*. |
| **Budget races** | Two concurrent requests both see "enough budget" and together overspend. | **Atomic reservation** (`reserve → commit / release`) and row-level locking (`SELECT … FOR UPDATE`). |
| **Cross-tenant leakage** | One customer reads or modifies another customer's routes, keys or funds. | Tenant-scoped data model, server-side RBAC and dedicated isolation / penetration tests. |
| **Rogue agents** | An agent misbehaves in production. | **Kill switch**: suspend an agent or revoke its API keys and the gateway blocks new requests within seconds. |

---

## How a Payment Works

AgentPay supports two payment paths. The gateway selects one deterministically from the credentials on the request.

### Path A — Virtual Wallet (API key)

```text
Agent ── HTTP + Authorization: Bearer apk_live_… ──▶ Gateway
   1. Authenticate key  → resolve tenant, project and agent
   2. Policy check      → per-tx cap, budget, rate limit, route allow-list
   3. HOLD              → freeze the price in the tenant wallet (row-locked)
   4. Forward to upstream (upstream credentials injected by the gateway)
   5a. 2xx  → CAPTURE   → append ledger entry, return 200
   5b. 4xx  → RELEASE   → unfreeze, nothing charged
   5c. 5xx / timeout → keep HELD, mark needs_review (never auto-charge, never auto-release)
```

### Path B — On-Chain x402 (EIP-3009)

```text
Agent ── GET /resource ──▶ Gateway
Agent ◀── 402 + PAYMENT-REQUIRED (price, payee, asset, network, expiry) ──
   1. Client parses the challenge
   2. Local policy engine: evaluate + atomically RESERVE budget
   3. Local signer: sign an EIP-712 / EIP-3009 authorization
   4. Replay request with PAYMENT-SIGNATURE
   5. Gateway verifies signature, nonce and price → forwards upstream
   6. Facilitator settles on-chain (gas sponsored, buyer needs only USDC)
   7. Buyer commits (or, after the authorization expires unspent, releases) the reservation
```

::: tip No silent fallback
If a wallet-path request has insufficient balance, the gateway returns `402 ERR_WALLET_INSUFFICIENT_BALANCE`. It **never** quietly switches to an on-chain charge. Moving to the on-chain path must be an explicit, new request signed by the buyer.
:::

---

## Architecture

```text
┌──────────────────────┐        ┌───────────────────────────────┐        ┌────────────────────┐
│       AI Agent       │        │        AgentPay Gateway       │        │  Upstream Service  │
│ LangChain / CrewAI / │ ─────▶ │  Auth · Policy · x402 · Proxy │ ─────▶ │  (your existing    │
│  custom + SDK client │        │  Wallet Hold/Capture/Release  │        │   HTTP API)        │
└──────────┬───────────┘        └───────┬───────────────┬───────┘        └────────────────────┘
           │                            │               │
   Local policy engine /         PostgreSQL 15+     Facilitator ──▶ EVM chain
   local signer (on-chain)     (ledger, config,    (verify & settle, Arbitrum Sepolia)
                                 audit events)
                                        ▲
                              Management Console (Vue 3)
                              routes · policies · wallet · agents · audit
```

| Component | Role |
| --- | --- |
| **Python SDK** (`agentpay`) | Transparent `httpx` interceptor (sync + async): detects `402`, runs the local firewall, signs, replays, commits or releases. Includes a LangChain paid-tool wrapper. |
| **Financial Firewall / Policy Engine** | Per-tx cap, rolling 24h budget, lifetime limit, payee / domain / network whitelists, rate limit, circuit breaker. SQLite-backed atomic reservation on the buyer side. |
| **Gateway** (`services/gateway`) | Go reverse proxy: route matching, 402 challenge generation, signature verification, header sanitization, upstream credential isolation, idempotency, recovery, agent-level policy and kill switch. A Python reference gateway ships for the v0.1.0 baseline. |
| **Virtual Wallet & Ledger** | One wallet per tenant; append-only ledger; Hold / Capture / Release; reconciliation. |
| **Facilitator** | Verifies authorizations and submits gas-sponsored `transferWithAuthorization` transactions. A mock Facilitator is provided for local runs. |
| **Management Console** | Routes & upstream auth, config draft → validate → publish → rollback, wallet, agents & audit timeline. |
| **PostgreSQL** | System of record for tenants, projects, routes, policies, ledger, payments and audit events. |

---

## Core Concepts

### Tenancy and identity

`Tenant → Project → Agent → API Key`. Every project belongs to exactly one tenant, every API key belongs to exactly one agent, and the **`agent_id` is always derived from the credential** — client-supplied headers such as `X-Agent-ID` are stripped and never trusted. Optional self-reported metadata (`X-Session-ID`, `X-Task-ID`, `X-Tool-Name`) is used only to group the audit timeline; it never affects authentication, billing or policy.

### The three-state model

A single "payment succeeded / failed" flag cannot describe reality. AgentPay tracks three orthogonal state lines:

| Object | Question it answers | Example states |
| --- | --- | --- |
| **PaymentIntent** | Where is this request in its overall lifecycle? | `created`, `challenged`, `authorized`, `completed`, `rejected`, `expired`, `failed`, `needs_review` |
| **Settlement** | Did the money actually move? | `not_started`, `held`, `submitted`, `pending`, `confirmed`, `released`, `failed`, `unknown` |
| **Fulfillment** | Did the upstream actually deliver? | `not_started`, `in_progress`, `succeeded`, `failed`, `unknown` |

`unknown` is a first-class state. An ambiguous result is never treated as success **or** failure, and is resolved only by querying, never by blind retry. See the [State Machine spec](/specs/state-machine).

### Idempotency

Clients send an `Idempotency-Key`. The same key plus the same request fingerprint replays the stored result (`X-Cache: HIT`) at **zero additional cost**; the same key with a different payload returns `409 ERR_IDEMPOTENCY_CONFLICT`. After an upstream 5xx, retrying with the same key reuses the original hold instead of charging again.

### Versioned configuration

Route and policy changes are edited as drafts, validated, then published as an **immutable, hash-fingerprinted snapshot**. Gateways swap to the new snapshot atomically with no downtime, every request records the config version that served it, and a rollback is a single pointer swap. If the control plane goes down, gateways keep serving from their last good snapshot and fail closed on anything they cannot verify.

---

## Safety Invariants

These rules are not tunable settings; they are design constraints enforced in code, in the database and in tests.

1. **Integer money only.** All amounts are integers in the smallest unit (USDC = 6 decimals). No floats anywhere, and database columns carry integer / non-negative `CHECK` constraints.
2. **Fail closed.** Lock timeout, parse error, verification failure or any unhandled exception ⇒ reject.
3. **Sign only after approval.** The signer is invoked only for a reservation the policy engine has already approved.
4. **A signature in flight is never "un-spent" by a timeout.** Budget is released only after the authorization has expired *and* it is confirmed unspent on-chain.
5. **Unknown never auto-resolves.** Unknown fulfillment keeps wallet funds held and escalates to `needs_review`.
6. **Append-only ledger.** Ledger entries and audit events are never updated or deleted; corrections are new reversal / adjustment entries.
7. **The model has no financial authority.** The policy engine is not an LLM tool, and the LLM cannot read or modify it.

---

## Deployment Topologies

| | Local embedded sandbox | Cloud-native distributed |
| --- | --- | --- |
| **Use it for** | Development, CI, demos (`make demo`) | Real testnet integrations |
| **Transport** | In-memory ASGI transport (no sockets) | HTTPS between independent buyer and seller domains |
| **Chain** | Local mock EVM RPC | Arbitrum Sepolia (`eip155:421614`) |
| **Keys** | Test key | Buyer's local signer / KMS; never uploaded |
| **Gas** | Simulated | Sponsored by the Facilitator; buyer holds only USDC |

See [Deployment](/guide/deployment) for details.

---

## Scope and Known Limitations

AgentPay is explicit about what it does **not** do:

- **No withdrawals in v1.** The virtual wallet is a closed-loop spending balance; there is no withdrawal and no user-to-user transfer.
- **No semantic quality checks.** AgentPay verifies that a paid call was delivered, not that the answer was *good*. Spending your allowed budget on a useless-but-whitelisted API is out of scope.
- **Host compromise is out of scope.** If an attacker has root on the machine running the local signer, they can read the key. HSM / KMS signers are planned.
- **In-process tools are not sandboxed.** In embedded mode the SDK shares a process with the agent; a tool with arbitrary code execution could bypass the interceptor. An out-of-process sidecar is planned.
- **Single ledger per key.** One private key must map to exactly one local ledger; running several processes with separate ledgers on the same key multiplies the budget.
- **Single asset, testnet only.** v1 supports one asset (USDC, EIP-3009 compatible) on testnets.

Read the full [Threat Model](/specs/) for what is and isn't defended.

---

## Current Status and Roadmap

**Available today (v0.1.x)**

- Python SDK with `httpx` interceptor, local firewall (SQLite atomic reservation, circuit breaker), EIP-712 / EIP-3009 signer and LangChain paid-tool wrapper
- Go gateway: reverse proxy, x402 handling, wallet path, idempotency & recovery, agent-level policy, kill switch, audit timeline
- Multi-tenant control plane: tenants, projects, RBAC, agents, API keys, wallet & admin grants, routes & policies
- Management console: routes and upstream auth, config publishing and rollback, wallet, agents, payment audit
- PostgreSQL 15+ schema, Python / Go cross-language conformance vectors
- Arbitrum Sepolia testnet settlement and a one-command **runaway-loop demo**

**In progress / next**

- Ledger reconciliation (five-way balancing), usage statistics, audit retention & export
- Admin-operation audit and YAML import / export
- Performance benchmarks, clean-environment deployment verification, pilot and release candidate

**Roadmap (2027+)**

- **Gas model 2 (v0.2.0):** dynamic gas surcharge folded into the USDC amount
- **Gas model 3 (v0.3.0):** batched / state-channel settlement for per-token micro-payments
- On-chain and fiat wallet top-ups
- TypeScript / Node.js SDK and Express / Fastify middleware
- Java / Spring Boot starter
- Anomaly detection and alerting (alert-only by default)
- Multi-node distributed gateway clusters, KMS / HSM signers

---

## Where to Go Next

| I want to… | Read |
| --- | --- |
| Run it in five minutes | [5-Minute Quickstart](/guide/quickstart) |
| Wire an agent up | [Python SDK](/guide/python-sdk) |
| Monetize an API | [Gateway Configuration](/guide/gateway) |
| Set spending limits | [Policy Configuration](/guide/policy) |
| Manage tenants, keys and routes | [Management Console](/guide/console) |
| Understand the wire protocol | [API Specs](/guide/api-specs) |
| Understand the guarantees | [State Machine](/specs/state-machine) |
| Deploy it | [Deployment](/guide/deployment) |
| Contribute | [Good First Issues](/contributing/good-first-issues) |