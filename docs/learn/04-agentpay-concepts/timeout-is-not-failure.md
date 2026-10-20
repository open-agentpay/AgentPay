---
title: Timeout Is Not Failure
description: Why network timeouts must never trigger naive payment retries, and how AgentPay eliminates double charges.
---

# Timeout Is Not Failure

::: warning Experimental Project
AgentPay has not undergone an independent security audit. Do not use with production funds. All examples on this page are based on testnets and test tokens.
:::

## In a Nutshell

**A timeout merely indicates that you did not receive a response; it never proves that the event did not occur.** In financial systems, timeouts must trigger state verification before any corrective action—never naive retries.

## A Concrete Scenario

Consider our research agent querying a market data API priced at $0.10 USDC (`100_000` atomic units):

```text
1. Agent queries endpoint            → Receives HTTP 402 "Payment Required"
2. Agent signs auth & re-requests     → Payment submitted
3. On-chain settlement succeeds       ✅
4. Seller processes request and prepares payload
5. Response drops in network / Seller times out ⏱️
6. Agent receives a Client Timeout error
```

The agent observes a timeout. Conventional retry logic concludes: "Request failed, try again":

```text
7. Agent sends request again, signs new payment
8. Second transaction also succeeds   ✅
```

The outcome: **the agent was billed twice for a service that was fulfilled once.** This is the classic Double Charge trap.

The root cause stems from a false equation:

> ❌ Timeout = Failure → Safe to Retry

## Deconstructing the Failure

A payment flow comprises three decoupled phases occurring across independent systems:

| Phase | Question Answered |
|---|---|
| **Payment Intent** | Was the payment proposal accepted and is the authorization valid? |
| **Fund Settlement** | Were funds actually debited (on-chain tx confirmed, or platform wallet held)? |
| **Service Fulfillment** | Did the seller's upstream service execute and return payload? |

Because these phases run asynchronously, timeouts can occur at any interface:

- Funds were deducted, but the HTTP response dropped in transit.
- The request reached upstream, but upstream is still computing.
- The transaction was broadcast, but on-chain receipt confirmation timed out.

In each scenario, **"no response received" maps to completely different underlying system states.** Treating them all as "failure" inevitably corrupts ledgers.

## Flawed Approaches and Why They Fail

### Approach 1: Treat Timeout as Failure and Retry with a Fresh Signature

Described above. Every new signature is a legally valid authorization on-chain, and each submission executes another debit.

### Approach 2: Treat Timeout as Success

Equally disastrous. If the request was dropped before reaching the seller, the buyer marked it paid and received nothing.

### Approach 3: Release Budget Immediately on Timeout

Suppose the client holds quota for this transaction. Once an EIP-3009 or transaction signature is broadcast, **the seller or relayer can still submit it on-chain prior to expiration**. Releasing the budget prematurely allows the agent to spend the same money twice:

```text
Reserve 0.1 → Signature sent → Timeout → Release quota → Agent spends 0.1
                                                               ↑
                     Original signature mined on-chain, total spend: 0.2
```

### Approach 4: Infinite Unbounded Retries

Unchecked retry storms during upstream outages amplify system load and multiply double-charge exposure.

## The AgentPay Approach

The core tenet: **"Result Unknown" is an explicit, first-class state, not an error.** In this state, the system permits query and reconciliation—never blind payment re-submissions.

```text
           Timeout / Exception
                    │
                    ▼
       Result Unknown (First-Class State)
                    │
       ┌────────────┴────────────┐
       ▼                         ▼
Query Underlying State    Blind Retries Prohibited
       │
  ┌────┼──────────────┐
  ▼    ▼              ▼
Settled Confirmed Void  Still Indeterminate
  │    │              │
Proceed Safe Quota    Maintain Lock,
       Release        Flag for Reconciliation
```

Four fundamental rules govern this model:

**1. Once a signature is dispatched, budget is never released immediately due to network drops.**  
Two criteria must be simultaneously satisfied before releasing the reservation: the authorization has expired (plus a safety buffer margin), and verifiable confirmation confirms it was never mined on-chain.

**2. When settlement outcome is unknown, blind re-submission is strictly forbidden.**  
If settlement is in-flight or indeterminate, the gateway queries receipts and reconciles state without creating new authorizations.

**3. Idempotency keys prevent duplicate billing.**  
The client attaches an immutable `Idempotency-Key` to every distinct business action. When the gateway detects an identical key, it performs a **Free Replay**, returning existing cached results or reusing active holds without additional charges. If the same key is supplied with differing payloads, it is rejected immediately as a tampering attempt.

**4. Payment settlement and service fulfillment are tracked independently.**  
When on-chain settlement succeeds but upstream fulfillment fails, AgentPay records `Settled / Fulfillment Pending`, granting free fulfillment retries rather than writing off the transaction as lost.

### Platform Virtual Wallet Behavior

On the gateway virtual wallet path, the lifecycle is `Hold → Upstream Call → Capture or Release`:

| Upstream Status | Gateway Behavior |
|---|---|
| Success (2xx) | **Capture**: Confirm hold into irrevocable deduction |
| Client Error (4xx) | **Release**: Release hold immediately without charge |
| Server Error (5xx) / Timeout | **Maintain Hold**: Neither capture nor release; preserve hold pending reconciliation |

5xx errors and timeouts cannot be released blindly because upstream may have partially or fully executed the work. Preserving the hold is the conservative choice: funds are temporarily unavailable, but neither overbilled nor leaked. Holds carry TTLs and are safely reclaimed by background workers.

### Client-Side Best Practice

```python
import httpx

headers = {
    "Authorization": f"Bearer {api_key}",
    "Idempotency-Key": "order-20261020-001",  # Fixed per business action
}
try:
    resp = httpx.get(url, headers=headers, timeout=10)
except httpx.TimeoutException:
    # Retry with the exact same Idempotency-Key: zero duplicate charge
    resp = httpx.get(url, headers=headers, timeout=10)
```

The golden rule: **Always reuse the same Idempotency-Key when retrying an operation.** Generating a random UUID per retry informs the gateway that each attempt is a new, chargeable transaction.

## The Replay: Resolving the Opening Scenario

```text
1. Agent pays & requests, seller captures payment
2. Response drops in network, agent catches timeout
3. Agent retries using the exact same Idempotency-Key
4. Gateway identifies existing transaction → Returns cached response without re-charging
```

Result: Single debit, zero double billing, payload safely delivered.

## Frequently Asked Questions

**Q: Why doesn't the system just wait until certainty is achieved?**  
Blockchain block confirmation and upstream latencies are unpredictable. Indefinite synchronous blocking degrades agent performance and exhausts connection pools. The system returns "Processing / Indeterminate" and completes reconciliation asynchronously.

**Q: How should idempotency keys be generated?**  
Derive them deterministically from the business context: e.g., `task_id + step_number`. All retries for that step share the key; distinct steps use distinct keys.

**Q: Does maintaining holds lock up working capital indefinitely?**  
No. Holds carry a strict TTL. Once expired without capture, background sweepers release the reservation or escalate to admin console alerts.

**Q: Does this guarantee 100% prevention of double debits?**  
These mechanics substantially eliminate duplicate billing under standard network faults, validated across extensive fault-injection suites (upstream 5xx, network drops, crash recovery). However, AgentPay remains experimental open-source software and has not undergone third-party auditing.

## Next Steps

- Understand concurrency budget holds: [Budget Reservation](./budget-reservation.md)
- Learn why state must decouple intent, settlement, and fulfillment: [Three-State Model](./three-state-model.md)
- Inspect payment traces under failure: [Read Payment Trace](../05-hands-on/read-payment-trace.md)

## Further Reading

- [Unified Payment State Machine](/specs/state-machine): State transitions and recovery workflows
- [Protocol Consistency Specification](/specs/protocol-consistency): Idempotency contracts and error codes
- [Wallet & Ledger Specification](/specs/wallet-ledger): Hold, capture, release rules
- [Threat Model v1](/specs/threat-model-v1): Hold leakage and replay threat analysis
