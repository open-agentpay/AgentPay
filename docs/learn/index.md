---
title: Learning Roadmap
description: "From Agent basics to payments and AgentPay: Understanding how autonomous AI agents safely spend money."
---

# Learning Roadmap

::: warning Experimental Project
AgentPay is currently experimental and **has not undergone an independent security audit. Do not use with production funds.** Testing on testnets with test tokens is strongly recommended.
:::

Agents are evolving from "answering questions" to "executing tasks": calling paid APIs, purchasing data, renting compute, and even hiring other agents. When agents begin to spend money, a fundamental question emerges:

> **Without human sign-off on every transaction, under what conditions is an agent authorized to spend money?**

This knowledge base explains the background of the problem and how AgentPay addresses it. No prior blockchain or payment expertise is required.

## AgentPay in a Nutshell

**LLMs can propose payments, but must never approve payments.**

AgentPay is an independent control layer placed between agents and real funds. Per-transaction limits, rolling budgets, payee whitelists, circuit breakers, failure recovery, and audit trails are all enforced by deterministic code, never left to model discretion.

Division of responsibility with the x402 protocol:

| | Question Answered |
|---|---|
| **x402** | How to pay? |
| **AgentPay** | Should it be paid? How much? What happens if it fails? |

## Where Should I Start?

### 🚀 Want to see results first: 10-Minute Experience

Walk through three scenarios in one script: **Normal payment success → Over-budget rejection → Runaway loop tripped by circuit breaker**.

→ [10-Minute Demo](./05-hands-on/ten-minute-demo.md)

### Choose Your Learning Path by Role

#### I am an Agent Developer (Buyer)

I want my agent to safely call paid APIs without draining funds.

1. [Common Agent Failure Modes](./01-agent-basics/agent-failure-modes.md)
2. [Why We Need a Financial Firewall](./04-agentpay-concepts/financial-firewall.md)
3. [Budget Reservation: Why "Check Then Pay" Fails](./04-agentpay-concepts/budget-reservation.md)
4. [Timeout Is Not Failure](./04-agentpay-concepts/timeout-is-not-failure.md)
5. [10-Minute Demo](./05-hands-on/ten-minute-demo.md)

#### I am an API Provider (Seller)

I want to monetize my API into a pay-per-request endpoint.

1. [HTTP 402 & x402](./02-agent-payments/http-402-and-x402.md)
2. [Complete Payment Lifecycle](./04-agentpay-concepts/payment-lifecycle.md)
3. [Configure Your First Paid Route in the Console](./05-hands-on/first-paid-route.md)
4. [Draft, Publish & Rollback](./05-hands-on/draft-publish-rollback.md)

#### I am an Enterprise Administrator

I manage multiple agents and am accountable for all outgoing expenditures.

1. [Agent Infrastructure Landscape](./03-infra-map/agent-stack.md)
2. [Tenants, Projects, Agents & API Keys](./04-agentpay-concepts/multi-tenant.md)
3. [Dual-Rail Architecture: Virtual Wallets vs On-Chain](./04-agentpay-concepts/dual-rail.md)
4. [Agent Identity: Observation vs Self-Reporting](./04-agentpay-concepts/agent-identity.md)
5. [Troubleshooting with Audit Timelines](./05-hands-on/audit-timeline.md)

## Complete Table of Contents

Organized from broad to specific: first universal agent and payment knowledge, then where AgentPay fits in the infrastructure stack, followed by core architectural concepts and hands-on tutorials.

### Chapter 1: Agent Development Basics

Universal knowledge independent of AgentPay, explaining why agents require financial boundaries.

[What is an Agent](./01-agent-basics/what-is-an-agent.md) · [Tool Calling](./01-agent-basics/tool-calling.md) · [MCP Standard](./01-agent-basics/mcp.md) · [Agent Frameworks](./01-agent-basics/agent-frameworks.md) · [Failure Modes](./01-agent-basics/agent-failure-modes.md) · [Autonomy Levels](./01-agent-basics/autonomy-levels.md)

### Chapter 2: Agent Payments

Why agents need to spend money, limitations of current payment methods, and what x402 and stablecoins solve.

[Why Agents Need to Pay](./02-agent-payments/why-agents-pay.md) · [Payment Options Compared](./02-agent-payments/payment-options.md) · [HTTP 402 & x402](./02-agent-payments/http-402-and-x402.md) · [Stablecoins & USDC](./02-agent-payments/stablecoins-and-usdc.md) · [Gasless Auth (EIP-3009)](./02-agent-payments/gasless-eip-3009.md) · [Micropayment Economics](./02-agent-payments/micropayment-economics.md) · [Five Trust Problems](./02-agent-payments/trust-problems.md) · [Ecosystem Landscape](./02-agent-payments/ecosystem-landscape.md)

### Chapter 3: Infrastructure Map

Connecting earlier concepts to understand where AgentPay sits in the agent infrastructure stack and what it deliberately does not do.

[Agent Stack Layers](./03-infra-map/agent-stack.md) · [Where AgentPay Fits](./03-infra-map/where-agentpay-fits.md) · [AgentPay vs x402](./03-infra-map/agentpay-vs-x402.md)

### Chapter 4: AgentPay Core Concepts

The "why" behind the design, with each article starting from a real failure scenario.

[Financial Firewall](./04-agentpay-concepts/financial-firewall.md) · [Payment Lifecycle](./04-agentpay-concepts/payment-lifecycle.md) · [Timeout Is Not Failure](./04-agentpay-concepts/timeout-is-not-failure.md) · [Three-State Model](./04-agentpay-concepts/three-state-model.md) · [Budget Reservation](./04-agentpay-concepts/budget-reservation.md) · [Dual-Rail Architecture](./04-agentpay-concepts/dual-rail.md) · [Agent Identity](./04-agentpay-concepts/agent-identity.md) · [Idempotency & Free Replays](./04-agentpay-concepts/idempotency.md) · [Multi-Tenant Model](./04-agentpay-concepts/multi-tenant.md) · [Threat Model Lite](./04-agentpay-concepts/threat-model-lite.md)

### Chapter 5: Hands-On Tutorials

Practical guides based on Python SDK, Go Gateway, and Web Management Console.

[10-Minute Demo](./05-hands-on/ten-minute-demo.md) · [First Paid Route](./05-hands-on/first-paid-route.md) · [Policy in Action](./05-hands-on/policy-in-action.md) · [Read Payment Trace](./05-hands-on/read-payment-trace.md) · [Audit Timeline & Kill Switch](./05-hands-on/audit-timeline.md) · [Draft, Publish & Rollback](./05-hands-on/draft-publish-rollback.md)

### Chapter 6: Quick Reference

[Glossary](./06-resources/glossary.md) · [ID Prefixes](./06-resources/id-prefixes.md) · [Safety Redlines](./06-resources/safety-redlines.md) · [FAQ](./06-resources/faq.md) · [Further Reading](./06-resources/further-reading.md)

## Differences Between Learn, Guide, and Specs

| Section | Question Answered | When to Read |
|---|---|---|
| **Learn** (Here) | Why is it designed this way? | Understanding concepts, threat models, and rationale |
| **Guide** | How do I do it? | Hands-on deployment, coding, and configuration |
| **Specs** | What is the exact contract? | Implementation, protocol integration, or auditing details |

Learn focuses on design concepts without repeating raw specification text. Each concept page links to corresponding Specs for deep dives.

## Conventions

- Amounts are represented in **atomic integer units** (USDC uses 6 decimals: `1 USDC = 1_000_000`). No floating-point numbers are used.
- Code samples default to the **Python SDK**.
- Sample networks default to **testnets** (such as Arbitrum Sepolia) with test tokens.
