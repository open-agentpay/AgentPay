---
title: Budget Reservation
description: 'Why "check budget then pay" fails under concurrency, and how AgentPay prevents overspending with atomic reservation.'
---

# Budget Reservation

::: warning Experimental Project
AgentPay has not undergone an independent security audit. Do not use with production funds. All examples on this page are based on testnets and test tokens.
:::

## In a Nutshell

**Budget check and budget deduction must be executed as a single indivisible action.** Checking first and paying later inevitably results in overspending under concurrent load. AgentPay adopts a three-phase pattern: "Reserve first, pay second, commit or release third."

## A Concrete Scenario

You allocate a 24-hour budget of 10 USDC to an agent. At a certain moment, the agent concurrently dispatches two requests for 6 USDC each:

| Time | Request A | Request B |
|---|---|---|
| t1 | Reads remaining budget: 10 | Reads remaining budget: 10 |
| t2 | 10 ≥ 6, check passed | 10 ≥ 6, check passed |
| t3 | Pays 6 USDC | Pays 6 USDC |
| Outcome | | **Total spent: 12 USDC, overspent by 2 USDC** |

Both requests were "checked", but both decisions were based on the same stale balance. This is common in agent architectures: a single complex task routinely parallelizes multiple tool invocations, or multiple agents share a single collective budget.

The defect lies in the time window between t1 and t3: **checking and deducting are decoupled.**

## Common Approaches and Why They Fall Short

### Approach 1: Check First, Pay Later

Described in the scenario above. The race window between check and payment allows concurrent requests to slip through simultaneously.

### Approach 2: Deduct Budget Only After Successful Payment

```text
Check → Make Payment → Deduct on Success
```

The race window expands dramatically: it now spans the entire duration of network payment execution, during which all concurrent requests read an outdated balance.

### Approach 3: In-Memory Mutex Counter

An in-memory lock works for a single-threaded process, but agents commonly run across multiple processes or distributed containers, and processes restart. In-memory counters are neither shared nor persistent; restarting resets the available budget back to maximum.

### Approach 4: Deduct Immediately on Request, Never Refund on Failure

This avoids overspending, but introduces another critical problem: when requests fail, get rejected upstream, or time out, budget remains permanently consumed. Over time, the budget "leaks away" until the agent is starved and incapacitated. Reserved funds must have a deterministic lifecycle.

## The AgentPay Approach: Reserve, Commit, Release

AgentPay breaks the budget impact of a payment into three distinct lifecycle operations:

```text
       Check Limit + Deduct Available      ← Single Atomic Operation
                     │
             Reserve Succeeded
                     │
    ┌────────────────┼────────────────┐
    ▼                ▼                ▼
Payment Success  Explicit Failure  Unknown Result
    │                │                │
  Commit          Release         Hold Active
(Accounted)   (Quota Restored) (Awaiting Reconciliation)
```

**Step 1: Reserve.** Check available quota and place a hold on that exact amount within a single atomic database transaction. No external request can observe an intermediate inconsistent state.

**Step 2: Pay.** The signing and payment workflow begins only after reservation succeeds. If reservation fails, the request is rejected immediately without invoking the signer.

**Step 3: Commit or Release.**
- Payment confirmed successful → **Commit**: the reservation converts into an irrevocable debit.
- Explicit failure (e.g., upstream 4xx client rejection) → **Release**: the hold is released and quota is restored.
- Unknown result (e.g., network timeout) → **Hold Active**: reservation remains locked until reconciled, aligning strictly with [Timeout Is Not Failure](./timeout-is-not-failure.md).

### Returning to the Concurrency Scenario

```text
Request A: Reserve 6 → Succeeded, remaining available: 4
Request B: Reserve 6 → Remaining 4 < 6, rejected immediately
```

Only one request passes; overspending is structurally impossible.

### How Atomicity Is Guaranteed

In AgentPay, reservation occurs at two architectural layers with equivalent semantics:

| Layer | Purpose | Implementation |
|---|---|---|
| **Buyer Local Firewall** | Restricts local agent spending | SQLite transaction with exclusive table locking before writes |
| **Gateway Platform Wallet** | Governs tenant virtual wallet debits | PostgreSQL row-level locks (`SELECT ... FOR UPDATE`), strictly queuing requests per wallet while keeping distinct wallets non-blocking |

On the platform wallet path, the core debit query uses conditional atomic updates:

```sql
UPDATE ap_wallet
SET available_cache = available_cache - :amount,
    held_cache      = held_cache + :amount
WHERE wallet_id = :wallet_id
  AND available_cache >= :amount;
-- Affected rows = 0 implies insufficient balance; rejected immediately.
```

Checking and deducting occur within a single SQL statement; the database guarantees isolation against concurrent transactions. In gateway concurrency test suites, 100 concurrent workers competing for a limited pool of funds consistently demonstrate that available balance never drops below zero.

### Rolling 24-Hour Budget Windows

AgentPay employs a **sliding rolling window** rather than calendar midnight resets:

- Fixed calendar window: Spending 10 USDC at 23:59 and another 10 USDC at 00:01 allows 20 USDC to be spent within two minutes.
- Rolling window: At any given point in time $t$, total spend over the preceding interval $[t - 24\text{h}, t]$ cannot exceed the ceiling.

This eliminates calendar boundary arbitrage.

### Reservation TTL and Expiration

To prevent upstream hangs from permanently locking quota, holds carry a TTL (default: `upstream_timeout + 60s`). Expired reservations are processed by background recovery workers.

## Configuration Guidelines

**One private key per ledger.** Running multiple agent processes on the same private key with independent local SQLite ledgers gives each process a full budget replica, multiplying effective spend by the number of instances. The same wallet should share a centralized ledger; for multi-agent clusters, the Gateway platform wallet is the recommended model.

**Atomic integer amounts only.** All numbers are stored and compared as integers in minimal token units (USDC uses 6 decimals). No floating-point rounding errors can accumulate.

**Fail-Closed on uncertainty.** Transaction lock timeouts, deadlocks, and unexpected errors fail closed. It is always safer to decline a transaction than risk overspending.

## Frequently Asked Questions

**Q: If a reservation is made but payment never happens, is quota locked forever?**  
No. Explicit upstream rejections trigger immediate release. Uncertain outcomes remain held until timeout expiration, where background reconciliation safely unblocks the quota.

**Q: What happens when an agent exhausts its budget?**  
Subsequent payments are immediately rejected with an explicit error code without generating signatures. You can adjust limits in the console or wait for older transactions to roll out of the 24-hour window.

**Q: Do row-level locks create bottlenecks under high concurrency?**  
Requests on the same wallet serialize momentarily, but the transaction duration is minimal: select, update, insert audit event, and immediate commit—upstream network I/O is never executed while holding database locks. Distinct wallets execute completely in parallel.

**Q: Why separate single-transaction caps from 24-hour budgets?**  
They address different threats: single-transaction caps prevent large unauthorized debits (e.g., prompt-injected price hikes), while budgets prevent runaway death spirals of many tiny micropayments.

## Next Steps

- Understand why uncertain timeouts require active holds: [Timeout Is Not Failure](./timeout-is-not-failure.md)
- See policy interception in action: [Policy in Action](../05-hands-on/policy-in-action.md)
- Explore dual-rail wallet mechanics: [Dual-Rail Architecture](./dual-rail.md)

## Further Reading

- [Wallet & Ledger Specification](/specs/wallet-ledger): Hold, capture, release, and reconciliation contracts
- [Multitenant Data Model](/specs/multitenant-data-model): Database schema for wallets and reservations
- [Threat Model v1](/specs/threat-model-v1): Double-spending and negative-balance mitigation
- [Security Model](/specs/security-model): Concurrency budget competition and single-ledger rules
