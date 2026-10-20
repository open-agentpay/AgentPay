---
title: Python SDK Guide
description: Use the AgentPay Python SDK - an httpx-based client with a local financial firewall, two payment modes (API-key wallet and on-chain x402), agent audit metadata, idempotent retries, LangChain paid tools and a FastAPI seller middleware.
---

# Python SDK Guide

The `agentpay` package is the Python SDK for AI agents and API providers. It gives your agent a drop-in HTTP client that handles paid APIs for you **while a local financial firewall decides whether each payment is allowed**.

::: warning Experimental
AgentPay is unaudited and intended for testnet and sandbox use only. Never put real funds on a key used with it.
:::

## What the SDK gives you

| Capability | Description |
| --- | --- |
| **`AgentPayClient`** | A subclass of `httpx.Client`. Use it exactly like `httpx`: `get`, `post`, timeouts, headers. |
| **Two payment modes** | *Wallet mode* (API key, tenant virtual wallet) and *on-chain mode* (local signer, x402 + EIP-3009). |
| **Local financial firewall** | A policy engine (per-tx cap, rolling budget, whitelist, circuit breaker) that runs before anything is paid. |
| **Audit metadata** | Optional headers that group an agent's calls into sessions and tasks in the console timeline. |
| **Integrations** | `paid_tool` for LangChain / CrewAI, and a FastAPI seller middleware, `requires_payment`. |

Both sync and async usage are supported (see [Sync and async](#sync-and-async)).

## Installation

Requires **Python 3.10+**.

```bash
# from the repository root
python3 -m venv .venv
source .venv/bin/activate
pip install -e python/packages/agentpay
python -c "import agentpay; print('agentpay OK')"
```

---

## The client

```python
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

client = AgentPayClient(
    signer=LocalSigner(...),          # signs on-chain authorizations
    policy=MemoryPolicyEngine(...),   # local financial firewall
    headers={...},                    # default headers, e.g. Authorization
)
```

| Argument | Required | Purpose |
| --- | --- | --- |
| `signer` | yes | Signs EIP-712 / EIP-3009 authorizations for the **on-chain path**. In wallet mode no signature is created, so it can be a placeholder. |
| `policy` | yes | The local policy engine. Evaluated before any payment. See [Policy Configuration](/guide/policy). |
| `headers` | no | Default request headers. `Authorization: Bearer <api_key>` selects **wallet mode**. |

Because `AgentPayClient` extends `httpx.Client`, standard `httpx` options such as timeouts work as usual.

<!-- TODO(author): confirm which extra keyword arguments (timeout, transport, base_url, ...) are forwarded to httpx.Client. -->

---

## Choosing a payment mode

The mode is chosen by the credentials on the request. There is no automatic fallback between modes.

| | **Wallet mode** | **On-chain mode** |
| --- | --- | --- |
| **Credential** | API key in `Authorization` header | Local private key (`LocalSigner`) |
| **Account needed** | Yes: tenant, project, agent, key from the [console](/guide/console) | No |
| **How it pays** | Gateway debits the tenant wallet: Hold → upstream → Capture | Client signs an EIP-3009 authorization; Facilitator settles on-chain |
| **Latency / cost** | Internal ledger, no gas | Network confirmation, gas sponsored by the Facilitator |
| **Best for** | Fleets of agents inside one organization | Cross-organization calls, no prior relationship |

::: tip
If a wallet-mode request has insufficient balance, you get `402 ERR_WALLET_INSUFFICIENT_BALANCE`. The SDK does **not** retry on-chain. Switching modes is always an explicit decision in your code.
:::

### Wallet mode (API key)

```python
import os

from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

policy = MemoryPolicyEngine(
    max_per_tx=100_000,         # 0.10 USDC per request
    daily_budget=5_000_000,     # 5.00 USDC per rolling 24h
)

# Wallet mode makes no on-chain signature, so the signer is a placeholder.
# This is a well-known public development key: never use a key that holds funds.
signer = LocalSigner(
    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
)

client = AgentPayClient(
    signer=signer,
    policy=policy,
    headers={"Authorization": f"Bearer {os.environ['AGENTPAY_API_KEY']}"},
)

resp = client.get("https://gateway.example.com/v1/market-data?ticker=NVDA")
print(resp.status_code, resp.json())
```

The gateway authenticates the key, resolves tenant, project and agent from it, applies gateway policy, holds the price in the tenant wallet, calls the upstream with credentials the **gateway** holds, and captures the charge on `2xx`. Your code never sees the upstream's secret.

An alternative to the `Authorization` header is `X-API-Key`.

### On-chain mode (x402)

```python
import os

from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

signer = LocalSigner(private_key=os.environ["AGENT_PRIVATE_KEY"])
policy = MemoryPolicyEngine(
    max_per_tx=1_000_000,                    # 1 USDC
    daily_budget=10_000_000,                 # 10 USDC
    whitelist_payees=["0xSellerAddress..."],
)

client = AgentPayClient(signer=signer, policy=policy)   # no API key needed

resp = client.get("https://gateway.example.com/v1/market-data")
print(resp.json())
```

When the server answers `402` with a `PAYMENT-REQUIRED` challenge, the client does this transparently:

```text
1. Decode the challenge (price, payee, asset, network, expiry)
2. Policy: evaluate → atomically RESERVE budget      (reject here = nothing is signed)
3. Signer: create an EIP-712 / EIP-3009 authorization
4. Replay the request with the PAYMENT-SIGNATURE header
5. On success: COMMIT the reservation
```

**Failure handling matters.** After a signature has been sent, a timeout or `5xx` does **not** release your budget, because the payment may still settle. The reservation is released only once the authorization has expired and is confirmed unspent. This is what prevents double charging.

::: warning Private keys
Load keys from environment variables or a secrets manager, never from source code or prompts. Never expose a signer or policy object to the model as a tool. For production, plan on KMS / HSM-backed signing (not yet available).
:::

---

## Sync and async

The client is synchronous by default. For async code, get an async client from the same object:

```python
import asyncio

async def main():
    async with client.async_client() as aclient:
        resp = await aclient.get("https://gateway.example.com/v1/market-data")
        print(resp.status_code, resp.json())

asyncio.run(main())
```

Use `async with` so connections are closed properly.

---

## Local financial firewall

The `policy` argument accepts a policy engine. Two are provided:

| Engine | State | Use for |
| --- | --- | --- |
| `MemoryPolicyEngine` | In memory (lost on restart) | Tests, demos, short-lived agents |
| SQLite-backed engine | Durable, atomic reservation in an exclusive transaction | Long-running agents, anything where budget must survive restarts |

```python
from agentpay.policy import MemoryPolicyEngine

policy = MemoryPolicyEngine(
    max_per_tx=500_000,
    daily_budget=5_000_000,
    whitelist_payees=["0x742d35Cc6634C0532925a3b844Bc454e4438f44e"],
)
```

All amounts are **integers in the smallest unit** (USDC has 6 decimals, so `500_000` is 0.5 USDC). The engine is fail-closed: any error means the payment is rejected. See [Policy Configuration](/guide/policy) for every rule.

::: warning One key, one ledger
A private key must map to exactly one SQLite ledger. Running several processes with the same key but separate ledgers multiplies your effective budget.
:::

---

## Agent identity and audit metadata

**Identity comes from the credential, not from headers.** The gateway derives `agent_id` from the API key. Any `X-Agent-ID` (or other `X-Agent-*`) header your client sends is **stripped and ignored**, so it cannot be used to impersonate another agent. You do not need to, and should not, set it.

What you *can* add is optional, self-reported context, used only to group calls in the console timeline:

| Header | Purpose |
| --- | --- |
| `X-Session-ID` | Group calls belonging to one workflow session |
| `X-Task-ID` | Group calls belonging to one task |
| `X-Tool-Name` | Name of the tool making the call |

```python
client = AgentPayClient(
    signer=signer,
    policy=policy,
    headers={
        "Authorization": f"Bearer {api_key}",
        "X-Session-ID": "session_nvda_analysis_001",
        "X-Task-ID": "task_market_data_fetch",
        "X-Tool-Name": "fetch_stock_quote",
    },
)
```

These headers are **100% optional** and never affect authentication, billing or policy:

| | With the headers | Without them |
| --- | --- | --- |
| **Request handling** | Sanitized (allowed characters only, max 128 characters) and recorded | Treated as empty; the request proceeds normally |
| **Billing and limits** | Unaffected; based only on the API key's agent | Unaffected |
| **Console timeline** | Filter and group by session / task; shown with a "self-reported" badge | Each call shown on its own, with the gateway-observed facts |

The gateway records its own observed facts (status, latency, charge, policy decision) separately from anything self-reported, so the audit trail can always distinguish what was *observed* from what was *declared*.

---

## Idempotency and safe retries

Networks fail, and the dangerous retry is the one that **pays twice**. Rules to follow:

1. **Send an `Idempotency-Key` for any request that costs money**, and reuse the **same key** when you retry the same logical request.
2. The same key with the same request replays the stored result (`X-Cache: HIT`) at **no additional cost**, including after an upstream `5xx` (the original hold is reused).
3. The same key with a **different** body returns `409 ERR_IDEMPOTENCY_CONFLICT`. Keys are not reusable across different requests.
4. **A timeout is not a failure.** If a request times out, the outcome is unknown. Retry with the same key rather than a new one.

```python
import uuid

key = str(uuid.uuid4())          # one key per logical operation

resp = client.post(
    "https://gateway.example.com/v1/report",
    json={"ticker": "NVDA"},
    headers={"Idempotency-Key": key},
)
# On a timeout, call again with the SAME key:
# resp = client.post(..., headers={"Idempotency-Key": key})
```

<!-- TODO(author): confirm whether the SDK auto-generates an Idempotency-Key when none is given, and document its derivation. -->

---

## Handling errors

A policy or payment rejection is not a crash: the request was refused **before** any money moved. Check the HTTP status and the canonical error code in the JSON body, not the message text.

```python
resp = client.get("https://gateway.example.com/v1/market-data")

if resp.status_code == 200:
    data = resp.json()
elif resp.status_code == 402:
    # A 402 WITHOUT a PAYMENT-REQUIRED challenge is a terminal error
    # (for example insufficient wallet balance). Do not retry blindly.
    print("Payment refused:", resp.json())
elif resp.status_code in (403, 429):
    # Policy, route or rate-limit rejection
    print("Blocked by policy:", resp.json())
else:
    resp.raise_for_status()
```

<!-- TODO(author): confirm the JSON error envelope shape and the exception types raised for local firewall rejections, then replace the generic handling above. -->

| Code | HTTP | Meaning | Typical handling |
| --- | --- | --- | --- |
| `ERR_POLICY_MAX_PER_TX` | 403 | Price above the per-transaction cap | Do not retry; investigate the price |
| `ERR_POLICY_BUDGET_EXCEEDED` | 403 | Rolling budget would be exceeded | Stop; wait for the window or raise the budget |
| `ERR_POLICY_CIRCUIT_BROKEN` | 429 | Circuit breaker open | Fix the loop; retry after the cooldown |
| `ERR_RATE_LIMIT_EXCEEDED` | 429 | Too many requests | Back off |
| `ERR_ROUTE_NOT_ALLOWED` | 403 | Agent may not call this route | Change the allow-list, not the code |
| `ERR_WALLET_INSUFFICIENT_BALANCE` | 402 | Wallet balance too low | Top up; no automatic on-chain fallback |
| `ERR_WALLET_FROZEN` | 403 | Wallet frozen | Resolve in the console |
| `ERR_API_KEY_INVALID` | 401 | Unknown, revoked or expired key | Check or rotate the key |
| `ERR_AGENT_SUSPENDED` / `ERR_AGENT_REVOKED` | 403 | Agent has been stopped | Re-enable or reissue |
| `ERR_IDEMPOTENCY_CONFLICT` | 409 | Same key, different request | Use a new key for a new operation |
| `ERR_UPSTREAM_TIMEOUT` / `ERR_UPSTREAM_FAILED` | 504 / 502 | Upstream problem; funds stay held for review | Retry with the **same** idempotency key |

Full list: [API Specs](/guide/api-specs).

---

## LangChain and CrewAI paid tools

Wrap a function so each call to it is a governed, paid call:

```python
from agentpay.integrations.langchain import paid_tool

@paid_tool(
    name="fetch_stock_quote",
    description="Fetch a real-time US stock quote (charged per call)",
    client=client,
)
def fetch_stock_quote(ticker: str) -> dict:
    resp = client.get(f"https://gateway.example.com/v1/market-data?ticker={ticker}")
    return resp.json()

result = fetch_stock_quote.invoke({"ticker": "NVDA"})
```

The tool only sees the function's inputs and outputs. The model can call the tool but has no access to the client's policy, signer or API key.

---

## Seller side: FastAPI middleware

To charge for your own FastAPI endpoint:

```python
from fastapi import FastAPI
from agentpay.fastapi import requires_payment

app = FastAPI()

@app.get("/v1/market-data")
@requires_payment(
    price=100_000,                                         # 0.1 USDC
    payee="0xSellerPayoutAddress...",
    asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",    # Base Sepolia USDC
    network="eip155:84532",
    facilitator_url="https://facilitator.agentpay.dev",
)
async def get_market_data():
    return {"status": "success", "data": "Alpha Market Data"}
```

Unpaid requests get a `402` with a `PAYMENT-REQUIRED` challenge; valid signed requests are verified with the Facilitator before your handler runs. To monetize an existing API **without changing its code**, put it behind the [Gateway](/guide/gateway) instead.

---

## Testing locally

- **In-memory sandbox.** The local embedded topology routes requests through an in-memory transport to the gateway code and a mock EVM RPC, with no sockets and no testnet funds. This is how `make demo` and the test suite run in seconds.
- **Against a local gateway.** Follow the [Quickstart](/guide/quickstart) to run the gateway and Postgres in Docker and call it with a development key.
- **Use rejection tests.** A request blocked by policy creates no signature and no charge, so it is safe to test limits repeatedly.

<!-- TODO(author): document the public import for the in-memory transport (GatewayTransport) if it is meant for users. -->

---

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `TypeError: unexpected keyword argument 'gateway_url'` / `'api_key'` | The client has no such arguments. Pass `signer`, `policy` and `headers={"Authorization": ...}`. |
| `401 ERR_API_KEY_INVALID` | Wrong, revoked or expired key, or a typo in the `Bearer` header. |
| Every call blocked with `ERR_POLICY_MAX_PER_TX` | Cap below the route price, or amounts entered in whole USDC rather than base units. |
| Budget behaves oddly after restarts | `MemoryPolicyEngine` loses state; use the SQLite engine and one ledger per key. |
| `httpx.ConnectError` | Gateway not running, wrong URL or port. |
| `httpx.ReadTimeout` | The outcome is unknown. Retry with the **same** idempotency key. |
| Duplicate-looking charges | Retries used a new `Idempotency-Key`. Reuse one key per logical operation. |
| `X-Agent-ID` seems ignored | By design. Agent identity comes from the API key. |

## Next steps

| I want to… | Read |
| --- | --- |
| Run everything locally | [5-Minute Quickstart](/guide/quickstart) |
| Set spending limits | [Policy Configuration](/guide/policy) |
| Put an API behind a price | [Gateway Configuration](/guide/gateway) |
| Create agents and API keys | [Management Console](/guide/console) |
| See the wire protocol | [API Specs](/guide/api-specs) |