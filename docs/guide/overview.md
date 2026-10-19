---
title: Project Overview
description: What AgentPay is, the core problems it solves, architecture topology, and payment settlement modes.
---

# Project Overview

::: warning Experimental Project
AgentPay is in active development and has not undergone an independent third-party security audit. It is strictly intended for testnet and sandbox environments. Do not connect to production mainnet funds.
:::

## What is AgentPay?

**AgentPay** is an open-source **financial firewall and autonomous payment gateway** purpose-built for AI Agents.

As Large Language Models interface with external tools, APIs, and data sources via Tool Calling and the Model Context Protocol (MCP), agents increasingly need to invoke paid APIs and compute resources. Traditional credit card or static API key models face two critical risks:
1. **Runaway Loops & Fraud Risks**: Prompt injection attacks or unexpected infinite loops can exhaust account quotas or incur massive bills within seconds.
2. **Private Key Exposure**: Exposing blockchain private keys or master payment secrets directly inside the LLM prompt context leads to catastrophic loss via jailbreaks or model hallucinations.

AgentPay acts as a **controlled intermediary barrier** between autonomous agents and upstream paid services, enforcing fine-grained budgets, policy boundaries, and automated settlements based on the [x402 protocol specification](/specs/state-machine).

---

## Architectural Topology

```
+------------------+         +-------------------------+         +--------------------+
|     AI Agent     |         |    AgentPay Gateway     |         |  Upstream Service  |
| (No Private Key) | ------->|   (Financial Firewall)  | ------->|   (HTTP 402/x402)  |
+------------------+         +-------------------------+         +--------------------+
                                | Policy & Budget Guard
                                | Wallet Hold & Capture
                                v
                     +---------------------+
                     | PostgreSQL / Ledger |
                     +---------------------+
```

- **Zero-Trust Private Key Isolation**: The agent only holds an API key (`apk_live_...`) issued by the tenant. Cryptographic signing and on-chain interactions are delegated to the gateway Facilitator or the internal ledger.
- **Financial Firewall Guardrails**: Real-time evaluation of per-transaction caps, 24-hour rolling budgets, destination whitelists, rate limits, and circuit breakers.
- **Immutable Ledger & Audit Trail**: Nanosecond-precision double-entry accounting state machine ensuring absolute consistency between balance reservation, upstream fulfillment, and final capture.

---

## Dual-Path Payment Settlement

AgentPay supports a dual-path settlement architecture to balance micro-payment throughput with decentralized verification:

| Mode | Use Case | Settlement Mechanism | Latency & Cost |
|---|---|---|---|
| **Custodial Virtual Wallet (Wallet Path)** | High-frequency micro-payments, intra-organization agent fleets | Atomic reservation (Hold) and capture (Capture) within gateway, PostgreSQL row-level locking | Sub-millisecond latency, zero gas fees |
| **On-Chain Settlement (On-Chain Path)** | Cross-organizational invocations, decentralized service protocols | Smart contracts on Arbitrum Sepolia / Base, asynchronous receipt confirmation | Trustless on-chain finality |

---

## Current Status & Roadmap

- **Current Implementation (v0.1.x Baseline)**:
  - **Core Gateway**: High-performance Go reverse proxy (`gateway-go`) with embedded policy engine and virtual wallet.
  - **Client SDK**: Async Python SDK with transparent HTTP 402 handshake and native MCP tool integration.
  - **Storage**: PostgreSQL 16 with multitenant row-level isolation schema.
  - **Testnet**: Arbitrum Sepolia (`eip155:421614`).
- **Roadmap (2027+)**:
  - TypeScript / Node.js SDK (Browser, Bun, MCP server integrations).
  - Java / Spring Boot Starter for enterprise SaaS integrations.
  - Multi-node distributed gateway clusters and full enterprise console.
