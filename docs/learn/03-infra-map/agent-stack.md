---
title: Agent Infrastructure Layers
description: The layers behind an agent that can spend autonomously, the problem each layer solves, and where AgentPay fits.
---

# Agent Infrastructure Layers

::: warning Experimental Project
AgentPay has not undergone an independent security audit. Do not use it with real production funds.
:::

## In a nutshell

Safe agent spending does not come from one component. It requires **infrastructure layers with distinct responsibilities**. AgentPay sits in the financial-control layer: it neither thinks on the agent's behalf nor transfers funds; it decides **whether payment should be allowed** between the two.

## The whole stack at a glance

The diagram below is a simplified teaching model. Real systems may have less distinct boundaries, but this helps locate each component.

```text
┌────────────────────────────────────────────────┐
│ Application & task   User goal: "Research NVIDIA GPUs" │
├────────────────────────────────────────────────┤
│ Agent               Model + orchestration (what to do) │
├────────────────────────────────────────────────┤
│ Tool interface      Tool calls / MCP / HTTP APIs      │
├────────────────────────────────────────────────┤
│ Identity & auth     Who is this agent; what may it do? │
├────────────────────────────────────────────────┤
│ ★ Financial control Should it pay? How much? Recovery?│ ← AgentPay
├────────────────────────────────────────────────┤
│ Payment protocol   How to pay (HTTP 402 / x402)       │
├────────────────────────────────────────────────┤
│ Clearing & settle  How funds actually move            │
└────────────────────────────────────────────────┘
        Audit & reconciliation span the stack
```

| Layer | Question answered | Typical examples |
|---|---|---|
| **Application & task** | What goal should be completed? | Research reports, data analysis, operations automation |
| **Agent** | What should happen next? | LLMs and orchestration frameworks such as LangChain and CrewAI |
| **Tool interface** | How are external capabilities called? | Tool calls, MCP servers, HTTP APIs |
| **Identity & authorization** | Who is calling, and what can they do? | API keys, agent profiles, role permissions |
| **Financial control** | Should it pay, how much, and what if something goes wrong? | Budgets, limits, allowlists, circuit breakers, recovery |
| **Payment protocol** | How is payment made? | HTTP 402 and x402 |
| **Clearing & settlement** | How do funds arrive? | On-chain stablecoin transfers, platform wallets and ledgers |
| **Audit & reconciliation** | Can events be reconstructed later? | Append-only audit events, ledger reconciliation |

The first three chapters cover the layers above. This chapter shows how they fit together.

## Which layers does a paid call pass through?

Continuing the research-agent example, suppose it needs paid market data:

1. The **agent** decides it needs market data and selects a tool.
2. The **tool interface** turns that decision into an HTTP request to the seller's API.
3. The seller returns **HTTP 402 Payment Required**, with requirements such as price and recipient. This is the **payment protocol** communicating.
4. The request reaches **financial control**: Is it within the per-transaction limit? Is enough budget available? Is the recipient allowlisted? Has the request rate become abnormal?
   - If any check fails, reject the request; **it does not proceed to later steps**.
   - If all checks pass, reserve budget and allow it to continue.
5. Only after approval does the signer create an authorization in the format required by the **payment protocol**.
6. **Clearing and settlement** perform the transfer, either on-chain or by debiting a platform wallet.
7. The data is returned to the agent, while **audit** records why the request was allowed.

The key point is that **step 4 happens before step 5**. The control is applied before any signature is created.

## Why the layers cannot replace one another

It may seem that one layer could simply do more. In practice, the following assumptions cause problems:

**"Let the model control its own spending."**
The model is one of the components most likely to err: it can hallucinate or be influenced by prompt injection. Asking it to limit itself is asking the subject of control to supervise itself.

**"The payment protocol verifies things, so that should be enough."**
A protocol checks whether a signature is valid, not whether the payment should be made. An agent manipulated into paying 20 USDC can still create an authorization that is perfectly valid at the protocol level.

**"The identity layer knows which agent this is; restricting permissions is enough."**
Identity and authorization answer "who are you, and may you call this interface?" Questions such as "how much can you still spend this month?" or "how many times did you call in the last minute?" require stateful budgets and counters. That is the financial-control layer's job.

**"The clearing layer only transfers funds; business rules belong elsewhere."**
The clearing layer usually cannot see business context. It only knows that an authorization exists and may need to be submitted on-chain. Settlement can succeed, fail, or remain unknown. Handling these outcomes requires state management above it; see [Timeout Is Not Failure](../04-agentpay-concepts/timeout-is-not-failure.md).

Each layer solves its own class of problems. **If a layer is missing, its corresponding risks have no owner.**

## What happens without financial control?

| Failure scenario | Without financial control | With financial control |
|---|---|---|
| Tool retries enter an infinite loop | Each retry pays until the balance is exhausted | A rate circuit breaker blocks later payments |
| Prompt injection directs payment to an unknown address | Protocol-valid payment proceeds | Recipient is not allowlisted; reject |
| Seller quotes a much higher price than expected | Sign the requested authorization | Exceeds per-transaction cap; reject |
| Concurrent requests consume the same budget | Each checks independently; combined spend exceeds limit | Atomic reservation lets only eligible requests proceed |
| Response times out after payment | Retrying may charge twice | Query the unknown result; do not pay again blindly |

These five cases correspond to concept articles later in this learning section.

## Relationship to MCP and x402

The three are often mentioned together, but have different responsibilities:

| | Problem addressed | In a nutshell |
|---|---|---|
| **MCP** | How an agent discovers and calls tools | A standard interface at the tool layer |
| **x402** | How payment is made over HTTP | A protocol at the payment layer |
| **AgentPay** | Whether this payment should be made and what to do if it fails | Financial control layer |

When an MCP tool is paid, the three can work in sequence: call the tool → receive 402 → AgentPay evaluates and allows the request → complete payment using x402. They complement one another rather than compete.

## FAQ

**Q: Do I need to deploy every layer?**
No. The smallest setup uses the SDK in the agent's Python code, with a local firewall and signer handling payment; both financial control and the payment protocol run locally. Add the gateway and console when you need centralized management for multiple agents, a paid API gateway for providers, or platform wallets and audit timelines.

**Q: Does AgentPay custody my private key or funds?**
For on-chain payments, the private key stays on the buyer's side; the gateway and clearing service do not handle it. A platform virtual wallet is an internal ledger account whose balance changes are recorded in the ledger. The current version does not support withdrawals.

**Q: Is this layering an industry standard?**
No. It is a simplification for learning. Projects may divide and name the layers differently; what matters is understanding that **each class of problem needs an owner**.

## Next steps

- To learn what AgentPay does and does not do: [Where AgentPay Fits](./where-agentpay-fits.md)
- To understand how it differs from x402: [AgentPay vs. x402](./agentpay-vs-x402.md)
- To start with a concrete scenario: [Why We Need a Financial Firewall](../04-agentpay-concepts/financial-firewall.md)
