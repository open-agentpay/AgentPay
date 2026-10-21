# What Makes Agent Payments Different?

## Ordinary payments and agent payments

Traditional payment systems generally assume a person understands the transaction and confirms it before payment. Agent payments have different properties:

- Software may trigger payment automatically.
- A single task may create many transactions.
- A model may select the amount dynamically.
- The model can be misled by incorrect or malicious input.
- A network failure can leave the result uncertain.

Agent payments are therefore not simply an AI layer added to an existing payment API. They must balance autonomous execution with protection of funds.

## A model can choose incorrectly

Suppose a user asks for the cheapest database of AI companies in Singapore. An agent finds three services:

| Service | Example price |
|---|---:|
| Provider A | $0.20 |
| Provider B | $0.08 |
| Provider C | $0.12 |

The agent might choose Provider B correctly, or choose a more expensive service because descriptions are unclear, its reasoning is mistaken, or tool output is misleading. It might also misunderstand the user's budget or decide that an unnecessary purchase is required.

The solution is not to assume the model will never err. A separate program should validate each request:

```text
Agent requests a $0.20 payment
          ↓
Check per-payment limit
          ↓
Check provider allowlist
          ↓
Check remaining budget
          ↓
Check request rate
          ↓
Allow or reject
```

Rules can limit losses even when the model makes a bad choice.

## Runaway spending loops

If each search API call costs 0.01 USDC, five calls cost 0.05 USDC. A software loop that makes 1,000 calls costs 10 USDC. A tiny individual charge does not guarantee a small total bill.

A payment system should consider per-payment limits, time-window budgets, request-rate limits, duplicate detection, circuit breakers, and task-level spending caps. A per-payment cap alone cannot stop many individually valid but unnecessary purchases.

## Concurrent requests can exceed a budget

Suppose an agent has 1 USDC remaining and sends two requests for 0.70 USDC each. If both requests independently read the same balance before either records its spend, both may be approved and total spending becomes 1.40 USDC.

An atomic reservation prevents this race: the first request reserves 0.70 USDC, leaving only 0.30 available, so the second request is rejected. The check and reservation must happen within a concurrency-safe transaction or equivalent mechanism. Checking first and decrementing later is not sufficient.

## A timeout does not prove failure

An agent may submit a settlement request, the facilitator may submit the transaction, and the response may be lost on its way back. The caller sees a timeout but cannot tell whether the request never arrived, is still processing, or completed successfully.

**A timeout is not the same as a failure.** Keep the result in an unknown or pending state, query the original operation, and only then decide what to do. Blindly repeating payment can cause a duplicate charge.

## Payment and fulfillment are separate

A purchase may be paid successfully while the data provider fails to deliver the data. Systems should record payment, settlement, and fulfillment separately. A failed delivery after confirmed settlement calls for recovery of the service step, not an automatic replay of the entire purchase.

## Separate signing authority from the agent

Giving an agent a complete private key can give it more control over funds than its task requires. A safer design keeps signing in a separate signer or wallet service and lets deterministic policy checks constrain what may be signed. The model can request a payment; it should not be the authority that grants itself permission or holds unrestricted signing power.

## Summary

Agent payment safety requires independent policy enforcement, atomic budget accounting, explicit handling of uncertain outcomes, separate fulfillment state, and carefully bounded signing authority. These controls address different failure modes and should be designed together.
