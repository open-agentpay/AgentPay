---
title: Agent Payments Knowledge Base
description: "A practical guide for developers exploring AI Agent payments and fund safety: x402, on-chain vs virtual balances, financial firewalls, runaway loops, and audit trails."
---

# Agent Payments Knowledge Base

This section covers **how autonomous AI agents can safely spend money**. The articles discuss universal industry concepts and tradeoffs independently of the AgentPay implementation.
At the end of each article, a dedicated callout explains how the concept maps to AgentPay configurations and specifications.

## Where to Start

| What You Want to Understand | Recommended Reading |
|---|---|
| Why AI agents need purchasing capabilities | [Why Agents Need to Pay](./fundamentals/why-agents-pay) |
| How HTTP 402 / x402 works under the hood | [HTTP 402 & x402](./payments/x402) |
| Tradeoffs between on-chain payments and prepaid balances | [On-Chain vs Virtual Wallet](./payments/onchain-vs-wallet) |
| Why network timeouts must never be treated as failures | [Timeout Is Not Failure](./payments/timeout-is-not-failure) |
| How to prevent runaway agents from draining budgets | [Financial Firewalls](./safety/financial-firewall) |
| Key terminology quick lookup | [Glossary](./glossary) |

::: tip In AgentPay
Every concept discussed here is formally specified in the [Specifications](/specs/) and actionable via our [Guides](/guide/overview).
:::

::: warning Experimental Notice
Payment and security architectures discussed here are for educational and architectural evaluation. AgentPay has not undergone an independent third-party security audit.
:::
