---
title: 5-Minute Quickstart
description: Run the AgentPay gateway locally, make your first policy-governed payment with the Python SDK, and watch the financial firewall stop a runaway agent.
---

# 5-Minute Quickstart

In this guide you will run the AgentPay gateway on your own machine, make a paid API call from a Python agent, and then see the firewall in action. No wallet, testnet funds or real money are required.

::: warning Sandbox only
AgentPay is experimental and has not been independently audited. Everything in this guide runs locally against development data. Never point it at mainnet funds.
:::

<!--
  VIDEO PLACEHOLDER — drop the embed here once recorded, e.g.:
  <iframe src="…" width="100%" height="420" allowfullscreen></iframe>
-->

## What you will do

| # | Step | Time | What you will see |
| --- | --- | --- | --- |
| 1 | Get the code and start PostgreSQL | ~1 min | A running database container |
| 2 | Start the gateway | ~1 min | Health checks returning `ok` |
| 3 | See the `402 Payment Required` challenge | ~30 s | The raw x402 challenge from the gateway |
| 4 | Install the Python SDK | ~1 min | `agentpay` importable |
| 5 | Make your first governed request | ~30 s | A `200` response, paid from the tenant wallet |
| 6 | Retry safely with an idempotency key | ~30 s | A replayed response at zero extra cost |
| 7 | Watch the firewall stop a runaway loop | ~1 min | Payments approved, then blocked |
| 8 | Clean up | ~10 s | Everything stopped |

## Prerequisites

| Tool | Version | Check with |
| --- | --- | --- |
| Git | any recent | `git --version` |
| Docker + Compose v2 | Docker 24+ | `docker compose version` |
| Go | 1.21+ | `go version` |
| Python | 3.10+ | `python3 --version` |
| curl | any | `curl --version` |

Make sure ports **5432** (PostgreSQL), **8080** (gateway data plane) and **8081** (admin API) are free. On Windows, use WSL 2.

---

## 1. Get the code and start PostgreSQL

```bash
git clone https://github.com/open-agentpay/AgentPay.git
cd AgentPay

docker compose up -d postgres
```

Confirm the container is up and healthy:

```bash
docker compose ps
```

You should see the `postgres` service with a status of `running` (or `healthy`). The schema is created from `infra/postgres/schema.sql`.

## 2. Start the gateway

Open a **second terminal** (the gateway runs in the foreground) and start it:

```bash
cd AgentPay/gateway-go
go run ./cmd/gateway
```

The first run downloads Go modules and compiles, so it may take a minute. When it is ready it logs that it is listening:

| Port | Purpose |
| --- | --- |
| `127.0.0.1:8080` | Data plane — the traffic your agents send |
| `127.0.0.1:8081` | Admin API — used by the management console |

Back in your first terminal, check that the gateway is alive:

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/readyz
curl -s http://127.0.0.1:8080/version
```

`/healthz` means the process is up; `/readyz` means it has loaded its configuration and can serve requests; `/version` prints build metadata.

::: details Where does the gateway get its routes?
On start it loads a configuration snapshot. The development configuration includes a sample paid route, `/weather/current`, which is the one used below. Gateways always report which snapshot served a request in the `X-Config-Version` response header. See [Gateway Configuration](/guide/gateway) to define your own routes.
:::

## 3. See the `402 Payment Required` challenge

Before using the SDK, look at what an agent sees when it calls a paid route **without any credentials**:

```bash
curl -i "http://127.0.0.1:8080/weather/current?city=SanFrancisco"
```

Expected result:

```text
HTTP/1.1 402 Payment Required
PAYMENT-REQUIRED: eyJ2ZXJzaW9uIjoiMi4wIiwi...
```

The `PAYMENT-REQUIRED` header is a Base64URL-encoded JSON challenge: price, payee, asset, network and an expiry. Decode it to read it:

```bash
curl -si "http://127.0.0.1:8080/weather/current?city=SanFrancisco" \
  | grep -i '^payment-required:' | cut -d' ' -f2 | tr -d '\r' \
  | python3 -c "import sys,base64,json; s=sys.stdin.read().strip(); s+='='*(-len(s)%4); print(json.dumps(json.loads(base64.urlsafe_b64decode(s)),indent=2))"
```

You will see fields such as `network`, `asset`, `amount` (in the smallest unit — USDC has 6 decimals, so `100000` is 0.1 USDC), `payee`, `validBefore` and `scheme`. This is the **on-chain path**: a buyer would sign an EIP-3009 authorization for exactly this amount. In the next step you will use the **wallet path** instead, which needs no signing at all.

## 4. Install the Python SDK

From the repository root, in a virtual environment:

```bash
python3 -m venv .venv
source .venv/bin/activate          # Windows (WSL): same; PowerShell: .venv\Scripts\Activate.ps1

pip install -e python/packages/agentpay
python -c "import agentpay; print('agentpay OK')"
```

## 5. Make your first governed request

Create `first_request.py`:

```python
import os

from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

# 1. Local financial firewall: protects you from an agent looping and burning budget.
#    Amounts are integers in the smallest unit (USDC has 6 decimals).
policy = MemoryPolicyEngine(
    max_per_tx=1_000_000,       # at most 1 USDC per request
    daily_budget=10_000_000,    # at most 10 USDC per 24 hours
)

# 2. Signer. In wallet mode no on-chain signature is made, so this is only a placeholder.
#    This is a well-known public development key: never put funds on it.
signer = LocalSigner(
    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
)

# 3. API key issued for your agent (development key seeded with a test wallet balance).
api_key = os.getenv("AGENTPAY_API_KEY", "apk_live_valid123")

# 4. The client behaves like a standard httpx.Client. The Authorization header selects
#    the wallet path: the gateway debits the tenant wallet and no signing is needed.
client = AgentPayClient(
    signer=signer,
    policy=policy,
    headers={"Authorization": f"Bearer {api_key}"},
)

# The gateway authenticates the key, checks policy, freezes the price in the tenant
# wallet (Hold), calls the upstream, then confirms the charge (Capture).
response = client.get("http://127.0.0.1:8080/weather/current?city=SanFrancisco")
print("Response status:", response.status_code)
print("Data:", response.json())
```

`AgentPayClient` extends `httpx.Client`, so `get`, `post`, timeouts and the rest of the `httpx` API work as usual. It takes three arguments:

| Argument | Role |
| --- | --- |
| `policy` | The **local** firewall (per-request cap, rolling budget, payee whitelist). Evaluated before anything is paid. |
| `signer` | Signs EIP-712 / EIP-3009 authorizations for the **on-chain path**. Unused when you authenticate with an API key. |
| `headers` | Default headers. `Authorization: Bearer <api_key>` selects the **wallet path**. |

Prefer async? Use the async client from the same object:

```python
import asyncio

async def main():
    async with client.async_client() as aclient:
        res = await aclient.get("http://127.0.0.1:8080/weather/current?city=SanFrancisco")
        print(res.status_code, res.json())

asyncio.run(main())
```

Run it:

```bash
python first_request.py
```

Expected output:

```text
Response status: 200
Data: {...weather data...}
```

That one call did a lot. In order, the gateway:

1. **Authenticated** the API key and resolved the tenant, project and agent from it.
2. **Evaluated policy** — per-request cap, rolling budget, rate limit, route allow-list.
3. **Held** the route price in the tenant wallet (the funds are reserved, not yet spent).
4. **Forwarded** the request to the upstream, injecting the upstream credentials itself.
5. **Captured** the held amount after the upstream returned `2xx`, appending an immutable ledger entry.

If the upstream had returned a `4xx`, the hold would have been **released** and nothing charged. If it had returned a `5xx` or timed out, the hold would stay in place and the payment would be marked for review. It is never auto-charged and never auto-released.

::: tip Sandbox key
`apk_live_valid123` is a **development seed key** that only exists in the local development configuration. In a real deployment you create agents and keys in the [Management Console](/guide/console); keys are shown once and stored only as hashes.
:::

## 6. Retry safely with an idempotency key

Networks fail. The danger is a retry that charges twice. Send the same request twice with an `Idempotency-Key`:

```bash
for i in 1 2; do
  curl -si "http://127.0.0.1:8080/weather/current?city=SanFrancisco" \
    -H "Authorization: Bearer apk_live_valid123" \
    -H "Idempotency-Key: quickstart-demo-001" \
    | grep -iE '^(HTTP/|x-cache|x-config-version)'
  echo "---"
done
```

Expected result:

```text
HTTP/1.1 200 OK
X-Config-Version: <n>
---
HTTP/1.1 200 OK
X-Cache: HIT
X-Config-Version: <n>
---
```

The second response is replayed from the first (`X-Cache: HIT`) and **costs nothing extra**. Re-sending the same key with a *different* request body is rejected with `409 ERR_IDEMPOTENCY_CONFLICT`, so a key can never be reused to smuggle in a different purchase.

## 7. Watch the firewall stop a runaway loop

The most important property of AgentPay is that a misbehaving agent gets cut off. The repository ships a one-command demo that shows three scenarios: a normal payment succeeds, an over-limit payment is refused, and a repeating payment loop trips the circuit breaker.

From the repository root:

```bash
make demo
```

The demo can run against a local mock chain (no wallet or testnet funds needed) or against Arbitrum Sepolia; see the repository README for how to switch. Look for these three outcomes in the output:

| Scenario | Expected outcome |
| --- | --- |
| Normal call within limits | Approved and paid |
| Single payment above the per-transaction cap | Refused — nothing is signed or charged |
| Same payment repeated in a tight loop | Approved at first, then **circuit breaker opens** and further payments are blocked |

To set your own limits, see [Policy Configuration](/guide/policy).

## 8. Clean up

Stop the gateway with `Ctrl+C` in its terminal, then:

```bash
docker compose down          # stop PostgreSQL (data is kept in its volume)
docker compose down -v       # …or also delete the data for a fully clean slate
```

---

## What just happened

```text
                    ┌─ wallet path (steps 5–6) ─┐
Agent ── API key ──▶│  Gateway: auth → policy → │──▶ Upstream API
                    │  HOLD → forward → CAPTURE │
                    └────────────┬──────────────┘
                                 ▼
                       PostgreSQL ledger + audit trail

Agent ── no key ───▶ Gateway ──▶ 402 + PAYMENT-REQUIRED   (on-chain path, step 3)
```

- Agents authenticate with a **revocable API key**. They never hold a private key on the wallet path.
- Every charge goes through **Hold → (fulfil) → Capture or Release**, so you are only billed for successful deliveries.
- Retries are made safe by **idempotency keys**.
- Spending limits and the circuit breaker are enforced **outside the model**, so a prompt cannot talk its way past them.

## Troubleshooting

| Symptom | Likely cause and fix |
| --- | --- |
| `port is already allocated` on `docker compose up` | Something else is using 5432. Stop it, or change the published port in `docker-compose.yml`. |
| Gateway exits with a database connection error | PostgreSQL is not ready yet. Wait a few seconds, check `docker compose ps`, then restart the gateway. |
| `bind: address already in use` on 8080 / 8081 | Another process holds the port. Find it with `lsof -i :8080` and stop it. |
| `go: go.mod requires go >= …` | Your Go version is too old. Upgrade to Go 1.21 or newer. |
| `curl: (7) Failed to connect` | The gateway is not running. Check the terminal where you ran `go run ./cmd/gateway`. |
| Python raises `ConnectError` | Wrong `gateway_url`, or the gateway is still compiling on first run. |
| `401 ERR_API_KEY_INVALID` | The key is not known to the gateway. Use the seeded development key, or create one in the console. |
| `402 ERR_WALLET_INSUFFICIENT_BALANCE` | The wallet lacks funds for the route price. The gateway will **not** fall back to on-chain payment. Top up the wallet in the console. |
| `403 ERR_POLICY_MAX_PER_TX` / `ERR_POLICY_BUDGET_EXCEEDED` | The firewall did its job. Raise the limits in your policy if the request is legitimate. |
| `pip install` fails on Python 3.9 or older | Use Python 3.10+. |

If you are still stuck, [open an issue](https://github.com/open-agentpay/AgentPay/issues) with the gateway log and the output of `curl -i` for the failing request.

## Next steps

| I want to… | Read |
| --- | --- |
| Give my agent spending limits | [Policy Configuration](/guide/policy) |
| Use AgentPay from LangChain or my own code | [Python SDK](/guide/python-sdk) |
| Put my own API behind a price | [Gateway Configuration](/guide/gateway) |
| Create tenants, agents, keys and routes in a UI | [Management Console](/guide/console) |
| Understand the wire protocol | [API Specs](/guide/api-specs) |
| Deploy beyond localhost | [Deployment](/guide/deployment) |