---
title: Agent Payments
description: Why agents need payment capabilities, how machine payments work, and the controls required to keep spending safe.
---

# Agent Payments

Agents increasingly call paid APIs, purchase data, rent compute, and use services provided by other agents. This chapter explains the payment landscape and the safety problems that appear when software can spend autonomously.

## Suggested reading order

1. [Why Agents Need to Pay](./why-agents-pay.md) — how autonomous work creates new spending needs.
2. [Payment Options Compared](./payment-options.md) — where cards, invoices, and on-chain payments fit.
3. [HTTP 402 and x402](./http-402-and-x402.md) — how a web request can ask for payment.
4. [Stablecoins and USDC](./stablecoins-and-usdc.md) — the role of stable-value assets and integer accounting.
5. [Gasless Authorization with EIP-712 and EIP-3009](./gasless-eip-3009.md) — how a buyer can authorize a transfer without holding gas tokens.
6. [Micropayment Economics](./micropayment-economics.md) — why transaction costs matter for small payments.
7. [The Five Trust Problems in Agent Payments](./trust-problems.md) — the core authorization, limit, recipient, recovery, and accountability questions.
8. [Agent Payment Challenges](./agent-payment-challenges.md) — model errors, loops, concurrency, timeouts, delivery, and signing boundaries.
9. [The Agent Payment Flow](./agent-payment-flow.md) — an end-to-end view from resource request to settlement and recovery.
10. [Payment State, Idempotency, and Recovery](./payment-state-and-idempotency.md) — how to represent uncertain outcomes and prevent duplicate effects.
11. [Spending Policies](./spending-policy.md) — limits, budgets, allowlists, rate controls, and approvals.
12. [Agent Payment Ecosystem](./ecosystem-landscape.md) — how adjacent protocols address different layers.

## Articles

| Topic | What it covers |
|---|---|
| [Why Agents Need to Pay](./why-agents-pay.md) | Paid resources, high-frequency usage, and why per-call human approval does not scale. |
| [Payment Options Compared](./payment-options.md) | The trade-offs among common payment approaches for machine workloads. |
| [HTTP 402 and x402](./http-402-and-x402.md) | HTTP payment requests and the x402 payment flow. |
| [Stablecoins and USDC](./stablecoins-and-usdc.md) | Stablecoin basics, USDC, testnet boundaries, and integer amounts. |
| [Gasless Authorization](./gasless-eip-3009.md) | EIP-712 signatures, EIP-3009 authorizations, and relayed transactions. |
| [Micropayment Economics](./micropayment-economics.md) | How fees, batching, and settlement choices affect small payments. |
| [Trust Problems](./trust-problems.md) | Five questions a safe agent payment system must answer. |
| [Agent Payment Challenges](./agent-payment-challenges.md) | Failure modes unique to autonomous and concurrent payments. |
| [Payment Flow](./agent-payment-flow.md) | Request, policy evaluation, reservation, authorization, delivery, and settlement. |
| [Payment State and Idempotency](./payment-state-and-idempotency.md) | State machines, retries, idempotency keys, and recovery. |
| [Spending Policy](./spending-policy.md) | Deterministic controls applied before payment execution. |
| [Ecosystem Landscape](./ecosystem-landscape.md) | Protocols and products grouped by the problem layer they address. |

## Key distinction

A payment protocol can describe **how** a payment is presented or settled. It does not by itself decide **whether** a particular agent should spend, how much budget remains, or how to recover from an uncertain outcome. Those decisions require independent controls and explicit state management.

Amounts in implementation examples should use integer base units rather than floating-point arithmetic. Examples involving real assets should be treated as illustrative; use testnets and test tokens for experiments.
