---
title: 10-Minute Demo
description: "Run three scenarios in a single command: normal payment success, over-budget rejection, and runaway loop tripped by circuit breaker."
---

# 10-Minute Demo

::: warning Experimental Project
AgentPay has not undergone an independent security audit. Do not use with production funds. This walkthrough uses testnets and test tokens only.
:::

## What You Will See

A single demo script runs locally on your machine, walking sequentially through three real-world scenarios:

| Scenario | What Happens | Firewall Response |
|---|---|---|
| **1. Normal Payment** | Agent purchases an NVDA market quote priced at $0.05 | Approved → Signed → Settled, agent receives data |
| **2. Over-Budget Attempt** | Agent calls a heavy computing endpoint priced at $2.50 | Single cap is $0.10; rejected immediately, **no signature generated** |
| **3. Runaway Loop** | Agent gets trapped in an infinite retry loop | Limit is 5 RPM; circuit breaker trips and blocks all subsequent calls |

The demo firewall is configured with:

```text
Single Cap: $0.10  |  24-Hour Budget: $1.00  |  Max 5 Requests / Minute
```

## Prerequisites

- Python 3.10+, `make`, and `git`
- Clone the repository:

```bash
git clone https://github.com/july-whj/AgentPay.git
cd AgentPay
```

- Refer to the repository README for dependency installation.

<!-- TODO: Add exact dependency installation commands -->

## Step 1: Run In-Memory Simulation (Recommended First)

```bash
make demo
```

This mode simulates the complete payment chain in memory—**no network calls, zero gas spent, no test tokens required**. It completes in seconds with 100% deterministic results.

### Phase 1: Normal Micro-Payment

```text
>>> [PHASE 1] Normal Micro-Payment with On-Chain Settlement <<<
Agent requests paid market data for 'NVDA' ($0.05 USDC).
✓ Tool Response Received: {'ticker': 'NVDA', 'price': 142.5, ...}
[Facilitator] ON_CHAIN_TRANSACTION_CONFIRMED
```

The agent executed a standard tool call, while under the hood:

1. Upstream returned an HTTP 402 "Payment Required" challenge.
2. The firewall evaluated: $0.05 is within single cap, budget available → **Approved**.
3. Local signer generated an off-chain authorization transfer signature.
4. Client retried with the payment header and received data.
5. The facilitator submitted the authorization for settlement.

Notice this line in the log:

```text
"gasChargedToBuyer": "0.0000 ETH (Free EIP-3009 Transfer)"
```

The buyer's wallet only holds USDC; it does not need ETH for gas (see [Gasless Auth (EIP-3009)](../02-agent-payments/gasless-eip-3009.md)).

### Phase 2: Exceeding Single-Transaction Cap

```text
[BuyerFirewall] POLICY_VIOLATION_INTERCEPTED
  "errorCode": "SINGLE_TX_CAP_EXCEEDED",
  "requestedAmountUsdc": "2.50",
  "maxAllowedPerTxUsdc": "0.10",
  "action": "PAYMENT_ABORTED_SAFELY",
  "walletFundsDrained": "0.00 USDC"
```

The endpoint asked for $2.50, but the single cap is $0.10. The firewall rejected the request **before reaching the signer**. With no signature generated, zero funds can leave the wallet.

Notice the `toolObservation` line in the log: the rejection reason is returned directly to the agent. The agent understands *why* it was blocked and can choose an alternative strategy, but has no mechanism to bypass or modify the policy.

### Phase 3: Runaway Loop Intercepted by Circuit Breaker

```text
[BuyerFirewall] CIRCUIT_BREAKER_TRIPPED
  "iteration": 5,
  "breakerState": "OPEN",
  "reason": "CIRCUIT_MAX_RPM_EXCEEDED (Max 5 RPM exceeded)",
  "cooldownSeconds": 300
✗ Loop iteration #5 BLOCKED by Circuit Breaker
```

The agent enters a runaway loop, firing 8 rapid invocations. The first 4 succeeded; combined with Phase 1, 5 payments occurred within one minute. On iteration 5, the circuit breaker tripped to `OPEN`, immediately blocking all subsequent invocations with a 300-second cooldown.

This is how the "$1,800/hour runaway loop" described in [Why We Need a Financial Firewall](../04-agentpay-concepts/financial-firewall.md) is halted in seconds.

### Final Audit Summary

```text
Calls Attempted:             10
Successful Paid Calls:       5
Over-Budget Blocks:          1
Runaway Loop Blocks:         4
Total Spend Attempted:       $2.63 USDC
Actual Funds Debited:        $0.09 USDC
Total Funds Saved by AgentPay: $2.54 USDC
```

Reconciling the numbers:

| Metric | Details |
|---|---|
| 10 calls attempted | Phase 1 (1) + Phase 2 (1) + Phase 3 (8) |
| 5 successful paid calls | Phase 1 (1) + First 4 loop iterations |
| 5 blocked calls | Over-cap (1) + Circuit breaker (4) |
| Actual debited: $0.09 | Total of the 5 authorized micro-payments |
| Saved: $2.54 | $2.50 over-cap call + 4 blocked loop iterations |

Important note: **The 4 micro-payments executed before the circuit breaker tripped are non-refundable.** The firewall caps maximum loss; it does not reduce authorized spending to zero.

## Step 2 (Optional): Real On-Chain Settlement

To see transactions broadcast and confirmed on a public blockchain explorer:

```bash
make demo-live
```

This mode broadcasts genuine transactions to the Arbitrum Sepolia testnet.

### Additional Prerequisites

- **Facilitator wallet**: Funded with a small amount of Arbitrum Sepolia testnet ETH for gas.
- **Buyer wallet**: Funded with Circle testnet USDC.
- Configure private keys and RPC in your environment according to README instructions.

<!-- TODO: Document exact environment variable names and faucets -->

::: danger Do Not Use Public Keys
Many public tutorials and local dev frameworks embed publicly known private keys. Bots constantly monitor those addresses. Always generate **brand-new test keys** for live mode and fund only with testnet faucet tokens. Never use test keys on mainnet.
:::

### Verify on Arbiscan

Upon completion, Phase 1 outputs a transaction hash and Arbiscan explorer link:

![Phase 1 Transaction on Arbiscan](./images/arbiscan-phase1-tx.png)

How to interpret the explorer fields:

| Arbiscan Field | Value | Meaning |
|---|---|---|
| **Status** | Success | Transaction executed successfully on-chain |
| **ERC-20 Tokens Transferred** | 0.05 USDC from buyer to seller | Actual amount spent, matching demo logs |
| **From** | Facilitator wallet address | Transaction broadcast and **gas paid by Facilitator, not Buyer** |
| **Transaction Fee** | ~0.0000018 ETH | Gas absorbed by the facilitator; 0 ETH for buyer |
| **Value** | 0 ETH | Native ETH transferred is zero; only USDC moved |
| **Timestamp** | Matches terminal log | Real-time on-chain confirmation |

This confirms the zero-gas model: the buyer authorizes the transfer with an off-chain signature, while the facilitator broadcasts the transaction and pays the network gas fee.

::: details Live Mode Notice: "nonce too low"
<!-- TODO: Remove once relayer queue is updated -->
During live mode settlement, you may occasionally observe:

```text
Failed to broadcast settlement tx: RPC error -32000: nonce too low ...
Settlement submission failed ... Facilitator error 409: Nonce has already been submitted for settlement
```

`nonce too low` indicates that the facilitator transaction nonce was preempted on-chain. This happens when the same wallet broadcasts multiple transactions concurrently or the address is used by multiple processes.

In such cases, affected settlements await reconciliation rather than immediate broadcast, illustrating [Timeout Is Not Failure](../04-agentpay-concepts/timeout-is-not-failure.md): **"Submission failure" does not mean "transaction does not exist"; state must be verified prior to re-transmission.**
:::

## Verification Checklist

After running the demo, verify that:

- [ ] Phase 1: Market data returned; buyer gas charged is 0
- [ ] Phase 2: Intercepted with `SINGLE_TX_CAP_EXCEEDED`; zero funds left wallet
- [ ] Phase 3: Blocked on iteration 5 with circuit breaker state `OPEN`
- [ ] Audit summary: Total funds saved exceeds actual debited funds
- [ ] (Live mode): Phase 1 transaction verified on Arbiscan

## Try Experimenting

Open `examples/runaway-agent/demo.py` and tweak the firewall parameters (single cap, 24-hour budget, max RPM):

1. Increase single cap to $3.00 and re-run. Does Phase 2 still get blocked? If so, which rule triggers?
2. Increase max RPM to 20. At which iteration does the circuit breaker trip?
3. Reduce 24-hour budget to $0.05. How does Phase 3 behave?

Observe the `errorCode` in terminal logs to see which policy rule intercepted each request.

## Next Steps

- Understand firewall architecture: [Why We Need a Financial Firewall](../04-agentpay-concepts/financial-firewall.md)
- Learn how concurrent budgets avoid overspending: [Budget Reservation](../04-agentpay-concepts/budget-reservation.md)
- Configure paid routes in Web Console: [First Paid Route](./first-paid-route.md)
