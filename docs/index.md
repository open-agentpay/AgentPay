---
layout: home

hero:
  name: "AgentPay"
  text: "Financial Firewall & Autonomous Payment Gateway for AI Agents"
  tagline: Empower AI agents to pay autonomously without exceeding financial authorization. Dual-path micro-settlements powered by HTTP 402 / x402 and internal virtual wallets.
  actions:
    - theme: brand
      text: 5-Minute Quickstart
      link: /guide/quickstart
    - theme: alt
      text: Explore Agent Payments
      link: /learn/
    - theme: alt
      text: Specifications
      link: /specs/
    - theme: alt
      text: GitHub
      link: https://github.com/open-agentpay/AgentPay

features:
  - title: 🛡️ Financial Firewall
    details: LLMs only propose payments; cryptographic signing and budget deductions execute behind an out-of-band policy engine. Per-tx caps, 24h rolling budgets, whitelists, and circuit breakers.
  - title: ⚡ Dual-Path Settlements
    details: On-chain x402 (EIP-3009 offline signing with zero gas token requirements for buyers) and internal virtual wallets (Hold → Fulfill → Capture/Release). Never silently downgrades on insufficient balance.
  - title: 🔄 Tri-State Orthogonal State Machine
    details: Strict separation of Payment Intent, Settlement, and Fulfillment. Network timeouts transition to explicit Unknown state, preventing double-spending and loss.
  - title: 🏢 Multitenant Governance
    details: Tenant, project, and member RBAC isolation. Key lifecycle management, draft configurations, immutable snapshot publishing, and sub-second rollbacks.
  - title: 👁️ Agent Identity & Audit Trail
    details: Cryptographic identity verified via credentials; strict separation of observed vs self-reported metadata. Append-only event logs, timeline tracking, and emergency kill-switches.
  - title: 📒 Append-Only Double-Entry Ledger
    details: Integer-based atomic currency units (micro-USDC). Immutable journal entries, multi-level automated reconciliation, and auditable accounting exports.
---

::: warning Experimental Project
AgentPay is under active development and **has not been audited by an independent security firm**. Evaluate exclusively on testnets and with test tokens. Do not use with production funds.
:::
