---
title: AgentPay vs. x402
description: x402 answers how to pay; AgentPay answers whether payment should be allowed. Learn how they divide responsibilities and work together.
---

# AgentPay vs. x402

::: warning Experimental Project
AgentPay has not undergone an independent security audit. Do not use it with real production funds.
:::

## In a nutshell

> **x402 answers “How do I pay?” AgentPay answers “Should I pay?”**

x402 is an open protocol that lets HTTP requests carry payment information. AgentPay is a control layer between an agent and that protocol: the protocol carries out the payment exchange; the control layer decides whether to allow it. They complement one another rather than replace one another.

## A brief introduction to x402

HTTP has a status code reserved long ago but rarely used: **402 Payment Required**. x402 puts it to work in a flow that broadly looks like this:

```text
1. Buyer requests a resource.
2. Seller returns 402 with requirements such as price, recipient, asset, network, and expiry.
3. Buyer signs a payment authorization, includes it in the request, and retries.
4. Seller (or its facilitator) verifies the authorization and returns the resource.
5. The authorization is submitted for settlement, immediately or asynchronously.
```

Payment happens as part of an HTTP request, without requiring an account, subscription, or prepaid balance. This can suit an agent that needs a service temporarily and pays per request without a person present.

AgentPay uses **x402 V2**. Two important headers are `PAYMENT-REQUIRED` (the seller's payment requirement) and `PAYMENT-SIGNATURE` (the buyer's signed authorization).

## An important fact: the protocol does not decide whether to pay

x402 addresses **interoperability**: buyers and sellers can exchange payment requirements and authorizations in a common format. It assumes the client knows when a payment is appropriate.

A basic protocol client may behave like this:

```text
Receive 402 → parse requirements → sign → retry
```

There is no step asking, “Wait, should I spend this money?” For example:

- The seller asks for 20 USDC, and the client signs.
- The seller changes to an unfamiliar recipient, and the client signs.
- A retry loop receives 402 repeatedly, and the client signs each time.

There is nothing wrong with the protocol; this responsibility is simply outside its scope. **That is division of responsibilities, not a protocol defect.** But an autonomous agent that can make mistakes needs a component to perform this check.

## What each one does

| | x402 | AgentPay |
|---|---|---|
| **Question answered** | How to pay | Whether to pay and what to do if it fails |
| **Layer** | Payment protocol | Financial control, with gateway, ledger, and audit capabilities around it |
| **Typical responsibilities** | Payment requirement format, authorization format, verification and settlement flow | Per-transaction caps, rolling budgets, allowlists, circuit breakers, atomic reservations, state recovery, audit |
| **Scope** | One payment exchange | An agent's spending behavior over time |
| **Stateful?** | Primarily a single exchange | Yes; tracks budgets, rates, and each payment's progress |
| **Defined by** | The open protocol community | The AgentPay project |

## How they work together

Here is where the two fit in one buyer-side x402 on-chain payment:

```text
Agent requests a resource
      │
      ▼
Seller returns 402 + payment requirements       ← x402: requirement format
      │
      ▼
┌─────────────────────────────────────┐
│ AgentPay financial firewall          │       ← AgentPay: should it pay?
│ Per-call cap · budget · allowlist     │
│ circuit breaker                      │
└────────────┬────────────────┬───────┘
          Allow              Reject
            │                  │
            ▼                  └─▶ Do not sign; explain reason to agent
Reserve budget → sign authorization             ← x402: authorization format
            │
            ▼
Retry with PAYMENT-SIGNATURE                    ← x402
            │
            ▼
Seller verifies → returns resource → async settlement
            │
            ▼
AgentPay commits/releases reservation and audits ← AgentPay: recovery and trace
```

AgentPay inserts a check **before and after** the x402 exchange: policy is evaluated before signing, and the reservation and audit are finalized after settlement progresses. The x402 format and flow remain unchanged, so the seller still sees a standard payment authorization.

## What AgentPay adds around x402

**1. Policy checks before payment.** This is the core distinction; see [Why We Need a Financial Firewall](../04-agentpay-concepts/financial-firewall.md).

**2. Budget protection under concurrency.** The protocol does not record how much you have spent or coordinate a budget across concurrent requests; see [Budget Reservation](../04-agentpay-concepts/budget-reservation.md).

**3. Handling failures afterward.** What if payment succeeded but the response timed out? What if settlement is unknown? These cases are beyond the protocol's scope; see [Timeout Is Not Failure](../04-agentpay-concepts/timeout-is-not-failure.md).

**4. A second payment rail.** x402 is for on-chain payments. AgentPay also offers a platform virtual wallet: a request carrying an API key is charged to the tenant wallet without putting each payment on-chain, reducing latency. The credential determines which rail is used. **Insufficient balance does not silently switch to an on-chain payment**, avoiding unexpected spending.

**5. A paid API gateway and management.** Turning an existing API into a paid endpoint, configuring prices and policies, publishing and rolling back changes, and reviewing usage and audit events are product capabilities around the protocol.

## Does AgentPay lock you in?

For the on-chain path, AgentPay follows x402 V2 formats and uses project test vectors to check encoding and signature consistency. Its core library does not depend strongly on a third-party x402 package; it implements the encoding and decoding according to the specification. If you also use an official library, optional interoperability utilities can convert between the two libraries' objects.

The platform virtual-wallet path does not use x402 payment headers; it uses an API key. Therefore, “AgentPay” is not synonymous with “an x402 implementation.” x402 is one of the payment methods it supports.

## FAQ

**Q: I already use x402. Why would I need AgentPay?**
If your agent runs in a controlled environment, has a small budget, and you monitor it, x402 may be enough. When an agent runs autonomously, handles concurrent requests, spends real money, or shares a budget with other agents, a control layer above the protocol becomes useful.

**Q: Can I write my own checks around an x402 client?**
Yes. A simple per-call cap is straightforward. Correct concurrent budget accounting, timeout handling after signing, circuit breaking, and audit involve many details; AgentPay aims to provide these together.

**Q: Does AgentPay replace x402?**
No. The on-chain payment path relies on x402. AgentPay adds value above the protocol rather than replacing it.

**Q: Does AgentPay interoperate with every x402 seller?**
AgentPay implements x402 V2 and validates it with protocol test vectors. Interoperability with third-party implementations still needs to be confirmed through practical testing. The project is experimental.

## Next steps

- To see AgentPay's full place in the system: [Where AgentPay Fits](./where-agentpay-fits.md)
- To understand HTTP 402 and x402: [HTTP 402 and x402](../02-agent-payments/http-402-and-x402.md)
- To see both work together: [10-Minute Demo](../05-hands-on/ten-minute-demo.md)

## Further reading

- [Protocol Consistency Contract](/specs/protocol-consistency): payment headers, dual-rail decisions, and error codes.
- [ADR-001: Reusing x402 vs. Implementing It In-House](/specs/adr-001): why the core library implements protocol encoding and decoding itself.
