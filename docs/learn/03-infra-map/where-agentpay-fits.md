---
title: Where AgentPay Fits
description: What AgentPay does and does not do in agent infrastructure, and which components buyers, sellers, and administrators use.
---

# Where AgentPay Fits

::: warning Experimental Project
AgentPay has not undergone an independent security audit. Do not use it with real production funds.
:::

## In a nutshell

AgentPay is the **financial-control layer** between an agent and funds: it decides whether to allow payment before it happens and makes failures recoverable and activity traceable afterward. It is not an agent framework, a payment protocol, or a blockchain.

## What it does

Returning to the diagram in [Agent Infrastructure Layers](./agent-stack.md), AgentPay covers financial control and extends slightly into the adjacent layers:

| Capability | Description |
|---|---|
| **Pre-payment policy evaluation** | Per-transaction caps, rolling budgets, recipient and network allowlists, rate circuit breakers, and loop detection |
| **Atomic budget reservation** | Prevents concurrent requests from exceeding the budget (see [Budget Reservation](../04-agentpay-concepts/budget-reservation.md)) |
| **Failure and unknown-result handling** | Tracks payment, settlement, and fulfillment separately; avoids blind retries when the outcome is unknown (see [Timeout Is Not Failure](../04-agentpay-concepts/timeout-is-not-failure.md)) |
| **Two payment rails** | On-chain payments (x402) and a platform virtual wallet; insufficient balance does not silently switch to the other rail |
| **Seller payment gateway** | Turns an existing API into a pay-per-request endpoint without changing its business code |
| **Tenant and agent management** | Tenant, project, and member permissions; agent and API key lifecycles; one-click disablement |
| **Audit and reconciliation** | Records requests, payments, settlement, fulfillment, and policy decisions for review by agent and session |

## What it does not do

Clear boundaries are as important as clear capabilities:

| Not responsible for | Reason and guidance |
|---|---|
| **Agent reasoning and orchestration** | These belong to models and frameworks such as LangChain or CrewAI. AgentPay integrates as a tool wrapper. |
| **Defining the payment protocol** | The on-chain path follows x402 V2. AgentPay uses the protocol; it does not define it (see [AgentPay vs. x402](./agentpay-vs-x402.md)). |
| **Judging whether a service is valuable** | A firewall cannot tell that an allowlisted service returned a useless answer after being paid within budget. |
| **Custody of your private key** | On the on-chain path, the key stays with the buyer; the gateway and clearing service do not handle it. |
| **Withdrawals and transfers** | The current platform virtual wallet is for closed-loop spending; it does not support withdrawals or transfers between users. |
| **Operating a public clearing network** | The project provides a reference implementation, not a public clearing service. |
| **Surviving a compromised host** | If the execution environment is fully compromised, a local private key may be exposed. Stronger isolation is planned for the future. |

## Three roles, three ways to use it

### Buyer: safely call paid services from an agent

You build an agent that needs paid APIs but should not have unlimited spending authority.

- Use the **Python SDK** and configure policy on the client.
- To the agent, this is an ordinary HTTP call; when a payment requirement appears, the firewall evaluates it and the signer handles authorization.
- Wrap the client as a paid tool for frameworks such as LangChain.

The smallest setup **does not require deploying a service**; the SDK and local firewall are enough.

### Seller: turn an API into a paid endpoint

You provide an API and want to charge per request without changing its existing code.

- Deploy the **AgentPay Gateway** as a proxy in front of the upstream service.
- Configure routes, prices, payment methods, and recipient addresses in the gateway.
- The gateway issues payment requirements, verifies payment, forwards requests, and avoids duplicate charges when the upstream service fails.

The gateway keeps the upstream service's real credentials; they are not exposed to callers.

### Administrator: manage multiple agents and their funds

Your team or enterprise needs to manage many agents and a shared budget.

- Use the **Web Console** to configure routes and policies, edit drafts, validate and publish them, and roll back if needed.
- Manage tenants, projects, members, and API keys; assign each agent its own budget and limits.
- Use audit timelines to inspect what each agent did, how much it spent, and why a request was allowed or rejected.
- Disable an agent with one action if its behavior becomes abnormal.

## Components

| Component | Purpose | Users |
|---|---|---|
| **Python SDK** | Client interceptor, local policy engine, signer | Buyers |
| **Go Gateway** | Reverse proxy, payment requirements and verification, wallet debits, idempotency and recovery | Sellers and administrators |
| **Web Console** | Route and policy configuration, publish and rollback, wallets, agents, and audit | Administrators |
| **Clearing service (reference implementation)** | Validates signatures and submits on-chain settlement; includes a local simulation | Development and demos |
| **Specifications and test vectors** | Authoritative definitions for protocol, state machine, error codes, and audit events | Integrators and developers building extensions |

AgentPay can run in two forms:

- **Embedded local mode:** all components run in one process without networking, suitable for development, testing, and demos (`make demo` uses this form).
- **Distributed mode:** the buyer runs in its own environment while the gateway and clearing service run in the cloud and communicate over HTTPS, matching a typical deployment.

## Current status

| Area | Status |
|---|---|
| Python SDK and local policy engine | Complete |
| Go Gateway and wallet payment path | Complete |
| Web Console (routes, policies, publish/rollback, wallets, agents, audit) | Complete |
| TypeScript SDK | Paused; planned |
| Independent security audit | Not performed |
| Real funds in production | Not recommended |

The project is experimental. Evaluate it on testnets with test tokens first.

## FAQ

**Q: I only want to stop my agent from overspending. Do I need to deploy the gateway?**
No. The local SDK firewall can already enforce per-transaction limits, budgets, request rates, and allowlists. Use the gateway and console when you need centralized management, paid API access, or platform wallets.

**Q: Is AgentPay a wallet product?**
No. The on-chain path uses the buyer's own wallet and private key. The platform virtual wallet is an internal gateway ledger account for low-cost, low-latency small charges; it is not a public wallet application.

**Q: Can it work with my existing agent framework?**
Yes. AgentPay integrates through HTTP client interceptors and tool wrappers; you do not need to change frameworks.

## Next steps

- To understand how it relates to x402: [AgentPay vs. x402](./agentpay-vs-x402.md)
- To see it in action: [10-Minute Demo](../05-hands-on/ten-minute-demo.md)
- To learn the design rationale: [Why We Need a Financial Firewall](../04-agentpay-concepts/financial-firewall.md)
