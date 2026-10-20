---
title: Policy Configuration Guide
description: Configure the AgentPay financial firewall - per-transaction caps, 24-hour rolling budgets, whitelists, rate limits, circuit breakers and policy inheritance - with recipes, sizing advice and troubleshooting.
---

# Policy Configuration Guide

Policies are the core of AgentPay. They decide **whether an agent may pay, how much, to whom, and how fast**, and they are enforced by deterministic code *outside* the model. An agent can propose a payment, but it can never change, read or talk its way past a policy.

::: tip The one rule to remember
Policies are **fail-closed**. A rule violation, a lock timeout, a parsing error or any unexpected exception all result in the request being **rejected**, never silently allowed.
:::

## Where policies are enforced

AgentPay enforces policy at two points. They are complementary: the first protects the buyer's own wallet, the second protects the platform and its tenants.

| Layer | Runs in | Protects | Configured by |
| --- | --- | --- | --- |
| **Local firewall** | The buyer's SDK process (`MemoryPolicyEngine` / SQLite-backed engine) | Your own signing key and budget from loops, injected prompts and inflated quotes | Python code or a YAML policy file |
| **Gateway policy** | The AgentPay Gateway, in the request path | The tenant wallet and platform: per-agent budgets, rate limits, route access, kill switch | [Management Console](/guide/console) (draft → validate → publish) |

Both layers evaluate **before money moves**. If either rejects, nothing is signed, held or charged.

---

## Quick example

A minimal, conservative policy for an agent that calls a ~0.05 USDC API:

```yaml
policy_id: "pol_weather_default"
tenant_id: "ten_default"

max_amount_per_tx: 500000          # 0.5 USDC per request
rolling_budget_24h: 5000000        # 5.0 USDC per rolling 24 hours

allowed_payees:
  - "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

circuit_breaker:
  consecutive_failures_limit: 3
  cooldown_seconds: 60
```

The same limits applied directly in the Python SDK:

```python
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

policy = MemoryPolicyEngine(
    max_per_tx=500_000,          # 0.5 USDC
    daily_budget=5_000_000,      # 5.0 USDC
    whitelist_payees=["0x742d35Cc6634C0532925a3b844Bc454e4438f44e"],
)
client = AgentPayClient(signer=LocalSigner(...), policy=policy)
```

---

## Amounts and units

**All amounts are integers in the smallest unit.** Floating-point numbers are never accepted or used for money. USDC has 6 decimals:

| Human value | Base units |
| --- | --- |
| 0.001 USDC | `1000` |
| 0.01 USDC | `10000` |
| 0.10 USDC | `100000` |
| 1 USDC | `1000000` |
| 100 USDC | `100000000` |

::: warning
`500000` means **0.5 USDC**, not 500,000 USDC. A value that is a factor of 10 too large is the most common configuration mistake, so double-check the zeros.
:::

---

## Rule reference

### Per-transaction cap

| | |
| --- | --- |
| **YAML key** | `max_amount_per_tx` |
| **SDK parameter** | `max_per_tx` |
| **Gateway field** | `max_amount_per_request` |
| **Type** | integer, base units, `> 0` |

The maximum price the agent will pay for a **single** request. It defends against rogue or compromised upstreams that suddenly quote $500 instead of $0.05. If a challenge or route price exceeds the cap, the request is rejected immediately with `ERR_POLICY_MAX_PER_TX` and **nothing is signed**.

### 24-hour rolling budget

| | |
| --- | --- |
| **YAML key** | `rolling_budget_24h` |
| **SDK parameter** | `daily_budget` |
| **Gateway field** | `rolling_24h_budget_cap` |
| **Type** | integer, base units, `> 0` |

A **sliding window**, not a calendar day: at any moment the engine sums what was spent or reserved in the last 24 hours and rejects a request that would push the total over the limit (`ERR_POLICY_BUDGET_EXCEEDED`). Reserved amounts count immediately, so concurrent requests cannot jointly overspend. Released reservations stop counting.

The per-transaction cap must be **less than or equal to** the rolling budget; a policy where the cap exceeds the budget is invalid and cannot be published.

### Lifetime limit

| | |
| --- | --- |
| **YAML key** | `lifetime_limit` |
| **Type** | integer, base units, `> 0` |

An absolute ceiling on total spend over the lifetime of the ledger, regardless of the rolling window. Useful as a final backstop for experimental agents.

### Payee, domain and network whitelists

| | |
| --- | --- |
| **YAML keys** | `allowed_payees`, `allowed_networks` |
| **SDK parameter** | `whitelist_payees` |
| **Gateway fields** | `whitelisted_payees`, `whitelisted_networks` |
| **Type** | list of strings |

Only pay recipients, and only on networks, that you have explicitly allowed.

- **Payees** are EVM addresses (`0x` + 40 hex characters).
- **Networks** use [CAIP-2](https://github.com/ChainAgnostic/CAIPs/blob/master/CAIPs/caip-2.md) identifiers such as `eip155:421614` (Arbitrum Sepolia).
- An **empty** list means "no restriction" for that dimension in most development setups. For anything beyond local experiments, always set `allowed_payees` explicitly so an attacker cannot redirect payments to their own address.

A request whose payee, asset or network is not allowed is rejected before signing.

### Route allow-list (gateway, per agent)

| | |
| --- | --- |
| **Gateway fields** | `whitelisted_routes` on the agent or policy (path patterns or route IDs) |

Restricts which routes an agent may call at all. An agent calling a route outside its allow-list receives `403 ERR_ROUTE_NOT_ALLOWED`, which is useful for giving a research agent access to data APIs but not to expensive compute routes.

### Rate limit

| | |
| --- | --- |
| **Gateway fields** | per-agent and per-route limits (requests per minute / per second); route default `rate_limit_rpm` |

Caps request frequency. Excess requests receive `429 ERR_RATE_LIMIT_EXCEEDED`. Rate limits complement budgets: a loop of very cheap calls can do damage through volume long before it exhausts a budget.

### Circuit breaker

| | |
| --- | --- |
| **YAML keys** | `circuit_breaker.consecutive_failures_limit`, `circuit_breaker.cooldown_seconds` |
| **Gateway field** | `circuit_breaker_max_rpm` |

The breaker detects runaway behavior such as **high-frequency payments**, **repeated payments to the same payee and resource** and **repeated failures**. When it trips it moves to `OPEN` and blocks further payments (`429 ERR_POLICY_CIRCUIT_BROKEN`) until the `cooldown_seconds` period has passed. It exists specifically for the "agent stuck in a tool-calling loop" failure mode; see it trip in the [Quickstart](/guide/quickstart) demo.

<!-- TODO(author): confirm whether the breaker has a half-open probe state, and how the loop-detection thresholds are exposed in YAML. -->

### Allowed payment methods (gateway, per route)

| | |
| --- | --- |
| **Gateway field** | `allowed_payment_methods` on the route: `wallet`, `onchain` or `both` |

Decides which payment path a route accepts. A request using a method the route does not allow is rejected with `403 ERR_PAYMENT_METHOD_NOT_ALLOWED`. There is **no automatic fallback** between methods: if a wallet-path request has insufficient balance it fails with `402 ERR_WALLET_INSUFFICIENT_BALANCE` rather than switching to an on-chain charge.

### Project budget cap

A project may carry a hard budget ceiling (`budget_cap_base_units`). `0` means "inherit from the tenant, no separate cap". Use it to stop one project from consuming the whole tenant wallet.

### Agent status and the kill switch

An agent in the `suspended` or `revoked` state is blocked at the gateway within seconds (`403 ERR_AGENT_SUSPENDED` / `ERR_AGENT_REVOKED`). Suspending an agent also disables its API keys. This is the fastest way to stop a misbehaving agent; it requires no policy change or publish.

---

## Policy inheritance

On the gateway, limits can be set at several levels. The effective policy for a request is resolved from the broadest to the narrowest level, and **the most restrictive value always wins**:

```text
Tenant  →  Project  →  Agent  →  Route  →  Policy rules
(broadest)                                   (narrowest)
```

| Level | Typical use |
| --- | --- |
| **Tenant** | Organization-wide ceiling and wallet |
| **Project** | Budget per team or business line |
| **Agent** | Limits for one specific agent (budget, rate limit, route allow-list) |
| **Route** | Limits tied to one paid endpoint and its allowed payment methods |

Example: if the tenant allows 100 USDC/day but an agent policy allows 5 USDC/day, the agent is limited to 5. A narrower level can never widen what a broader level forbids, and for allow-lists a request must be permitted at every level.

---

## How budget is reserved

Checking a budget and *then* paying would let two concurrent requests both see "enough budget" and overspend. AgentPay therefore uses **atomic reservation**.

**Buyer-side (on-chain path)**

```text
evaluate  →  RESERVE (atomic)  →  sign  →  send  →  COMMIT   (success)
                                                  →  RELEASE  (definitive failure)
```

- Reservation runs inside an exclusive database transaction, so concurrent reservations serialize.
- The signer is only called for an **approved reservation**.
- Once a signature has been sent, a network error or `5xx` does **not** release the budget. It is released only after the authorization has expired (`now > validBefore + 60s`) **and** it is confirmed unspent on-chain. This prevents a double spend.

**Gateway-side (wallet path)**

```text
HOLD (row-locked)  →  upstream  →  2xx: CAPTURE   |   4xx: RELEASE   |   5xx/timeout: stay HELD + needs_review
```

A hold expires after the upstream timeout plus 60 seconds (default 120 s) and is then released by a background sweeper. If the upstream outcome is unknown, funds stay held and the payment is flagged for review rather than being charged or released automatically.

---

## Ways to configure policies

### A. Python SDK (local firewall)

Best for development and for protecting an agent's own key.

```python
from agentpay.policy import MemoryPolicyEngine

policy = MemoryPolicyEngine(
    max_per_tx=100_000,        # 0.10 USDC
    daily_budget=5_000_000,    # 5.00 USDC
    whitelist_payees=["0x742d35Cc6634C0532925a3b844Bc454e4438f44e"],
)
```

`MemoryPolicyEngine` keeps state in memory and is ideal for tests and short-lived agents. For state that survives restarts, use the SQLite-backed policy engine, which uses atomic transactions for reservation.

::: warning One key, one ledger
A given private key must map to exactly **one** SQLite ledger file. Running several processes with the same key but separate ledgers multiplies your effective budget, which defeats the firewall.
:::

### B. YAML policy file

Keep limits in version control and load them into the SDK's `PolicyConfig` model, which validates them with Pydantic. Validate a file before deploying it:

```python
import yaml
from agentpay.policy import PolicyConfig

with open("policy.yaml") as f:
    config = PolicyConfig.model_validate(yaml.safe_load(f))
print(config)
```

A malformed YAML file, an invalid address or a negative amount raises a validation error here, long before the agent runs.

<!-- TODO(author): confirm the exact import path / loader for PolicyConfig and that the YAML keys above match the model field names. -->

### C. Management Console (gateway policies)

Gateway policies follow a **draft → validate → publish → rollback** lifecycle:

1. **Edit a draft.** Changes are saved as a draft and have **no effect** on live traffic.
2. **Validate.** The console runs the checks below and shows a diagnostic report for any error.
3. **Review the diff** between your draft and the active version.
4. **Publish.** An immutable, hash-fingerprinted snapshot is created and gateways swap to it atomically with no downtime. Invalid drafts cannot be published.
5. **Roll back** to any earlier version in one step if something goes wrong.

Every request records the config version that served it (`X-Config-Version`), so you can always tell which policy applied.

**Publish-time validation**

| Check | Rule |
| --- | --- |
| Per-tx vs budget | `max_amount_per_request` ≤ `rolling_24h_budget_cap` |
| Payee address | Matches `^0x[a-fA-F0-9]{40}$` |
| Price | `price_base_units` ≥ 0 |
| Payment methods | One of `wallet`, `onchain`, `both` |
| Route paths | Unique within a tenant |
| Upstream URL | Starts with `http://` or `https://` |
| Rate / breaker values | Non-negative |

::: tip YAML import / export
Importing and exporting full gateway configuration as YAML is planned. Imported configuration will go through the same validation and permission checks as the console.
:::

---

## Error reference

When a policy blocks a request, the gateway returns a standard JSON error envelope with a canonical code. Use the **code**, not the message, in your client logic.

| Code | HTTP | Meaning | What to do |
| --- | --- | --- | --- |
| `ERR_POLICY_MAX_PER_TX` | 403 | Price exceeds the per-transaction cap | Raise the cap if the price is legitimate; otherwise treat the upstream as suspicious |
| `ERR_POLICY_BUDGET_EXCEEDED` | 403 | The rolling budget would be exceeded | Wait for the window to roll, or raise the budget |
| `ERR_POLICY_CIRCUIT_BROKEN` | 429 | Circuit breaker is open | Find the loop; retry after the cooldown |
| `ERR_RATE_LIMIT_EXCEEDED` | 429 | Request rate limit hit | Slow down or raise the limit |
| `ERR_ROUTE_NOT_ALLOWED` | 403 | Agent may not call this route | Add the route to the agent's allow-list |
| `ERR_PAYMENT_METHOD_NOT_ALLOWED` | 403 | Route does not accept this payment method | Use an allowed method or change the route |
| `ERR_WALLET_INSUFFICIENT_BALANCE` | 402 | Wallet balance too low (no fallback to on-chain) | Top up the wallet |
| `ERR_WALLET_FROZEN` | 403 | Wallet frozen, including arrears | Resolve the freeze in the console |
| `ERR_AGENT_SUSPENDED` / `ERR_AGENT_REVOKED` | 403 | Agent has been stopped | Re-enable the agent, or issue a new one |

Note that not every policy violation is a `403`: circuit-breaker and rate-limit rejections return `429` so clients can back off.

---

## Recipes

Starting points to adapt. Values are in base units with the human value in the comment.

### Research agent (a few paid data APIs)

```yaml
max_amount_per_tx: 200000          # 0.20 USDC
rolling_budget_24h: 5000000        # 5.00 USDC
allowed_payees: ["0x...dataProviderA", "0x...dataProviderB"]
circuit_breaker: { consecutive_failures_limit: 3, cooldown_seconds: 60 }
```

### High-frequency micro-API agent

```yaml
max_amount_per_tx: 5000            # 0.005 USDC
rolling_budget_24h: 20000000       # 20.00 USDC
allowed_payees: ["0x...apiVendor"]
circuit_breaker: { consecutive_failures_limit: 5, cooldown_seconds: 30 }
```

Pair this with a gateway **rate limit** tuned to normal peak traffic.

### Locked-down experiment ("I just want to try it")

```yaml
max_amount_per_tx: 50000           # 0.05 USDC
rolling_budget_24h: 500000         # 0.50 USDC
lifetime_limit: 2000000            # 2.00 USDC total, ever
allowed_payees: ["0x...onlyThisOne"]
circuit_breaker: { consecutive_failures_limit: 2, cooldown_seconds: 300 }
```

---

## Choosing the numbers

1. **Start from the price.** Set `max_amount_per_tx` to about **2–5×** the highest legitimate route price. Much lower and legitimate calls are blocked; much higher and a price-gouging upstream gets through.
2. **Budget from expected volume.** `rolling_budget_24h ≈ expected calls per day × average price × 2` as a safety margin. Worked example: 200 calls/day at 0.02 USDC is 4 USDC of normal spend, so an 8 USDC budget leaves headroom without allowing runaway.
3. **Rate limit from normal peak.** Set the requests-per-minute limit to about twice your observed peak.
4. **Keep the cap below the budget.** Typically the budget is at least 10× the per-tx cap, so a single request can never consume the whole day.
5. **Tighten after observing.** Start strict in a sandbox, watch the audit timeline for `DENY` decisions on legitimate traffic, then loosen deliberately.

---

## Testing your policy

Test limits on purpose, before they matter.

- **Over the cap.** Call a route whose price is above `max_amount_per_tx`. Expect `ERR_POLICY_MAX_PER_TX` and no ledger entry.
- **Budget exhaustion.** Set a tiny budget (for example two calls' worth) and make three calls. The third should return `ERR_POLICY_BUDGET_EXCEEDED`.
- **Wrong payee.** Point a route at an address not on the whitelist and confirm the request is rejected before signing.
- **Loop.** Run `make demo` to watch a repeating payment trip the circuit breaker.
- **Kill switch.** Suspend an agent in the console and confirm new requests are refused within seconds.

Because a rejected request creates no signature and no charge, these tests are safe to run repeatedly.

---

## Monitoring and auditing

- Every decision is recorded as an `agent.policy_decision` event with the outcome (`ALLOW` / `DENY`), the rule that matched and the reason code. These appear in the console's **agent timeline** and **payment audit** pages.
- `agent.spend` events record actual charges, tied to the same request, so spend and decisions can be reconciled.
- Policy and configuration changes are recorded with who made them and the config version.
- Watch for **repeated `DENY`s** from one agent: that is either a mis-sized policy or an agent in trouble. Consider suspending it.

---

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| Every request is rejected with `ERR_POLICY_MAX_PER_TX` | Cap is smaller than the route price, or amounts were entered in whole USDC instead of base units. |
| `ERR_POLICY_BUDGET_EXCEEDED` earlier than expected | The window is rolling, and **reserved** (not yet confirmed) amounts count. Check for in-flight or stuck holds. |
| Budget looks "reset" or doubled after a restart | An in-memory engine lost state, or several processes use separate ledgers with one key. Use the SQLite engine and one ledger per key. |
| Draft saved but traffic behaves the same | Drafts are not live. Validate and **publish** to take effect. |
| Publish blocked | A validation rule failed, often `max_amount_per_request` greater than the budget or a malformed address. Read the diagnostic report. |
| `429 ERR_POLICY_CIRCUIT_BROKEN` | The breaker is open. Fix the loop first, then wait for the cooldown. |
| Policy change not visible in a request | Check `X-Config-Version` on the response to see which snapshot served it. |

---

## Best practices

1. **Always set `allowed_payees`** outside local experiments. It is the single strongest defence against payment redirection.
2. **Use layered limits.** Combine a per-tx cap, a rolling budget, a rate limit and the circuit breaker; each catches failures the others miss.
3. **Prefer least privilege.** Give each agent its own key, budget and route allow-list rather than sharing a broad one.
4. **Keep policy out of the model's reach.** Never expose policy editing as an LLM tool or let prompts influence limits.
5. **Version and review.** Keep YAML policies in Git and review changes like code; use publish and rollback for gateway policies.
6. **Stay on testnet.** AgentPay is experimental and unaudited. Use test tokens only.

## Next steps

| I want to… | Read |
| --- | --- |
| See policies block a runaway agent | [5-Minute Quickstart](/guide/quickstart) |
| Use the policy engine in code | [Python SDK](/guide/python-sdk) |
| Attach policies to routes and prices | [Gateway Configuration](/guide/gateway) |
| Manage policies in a UI | [Management Console](/guide/console) |
| Understand every guarantee | [State Machine](/specs/state-machine) |