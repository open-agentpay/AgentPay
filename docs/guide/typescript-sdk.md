---
title: TypeScript SDK Guide
description: Use the AgentPay TypeScript / Node.js SDK - a native fetch-based client with a local SQLite financial firewall, two payment modes (API-key wallet and on-chain x402), agent audit metadata, idempotent retries, and integrations for LangChain.js, Vercel AI SDK, and MCP.
---

# TypeScript SDK Guide

The `open-agentpay` package (`sdk/typescript/packages/agentpay`) is the official TypeScript / Node.js SDK for autonomous AI agents and tool developers. It provides a drop-in HTTP client and `fetch` interceptor that handles paid APIs automatically **while a local financial firewall decides whether each payment is allowed**.

::: warning Experimental
AgentPay is unaudited and intended for testnet and sandbox use only. Never put real funds on a key used with it.
:::

## What the SDK gives you

| Capability | Description |
| --- | --- |
| **`AgentPayClient` & `createAgentPayFetch`** | A native `fetch`-based HTTP client (`get`, `post`, `put`, `delete`, `request`) and a drop-in `typeof globalThis.fetch` adapter (`client.asFetch()`). |
| **Two payment modes** | *Price-aware wallet mode* (API key, x402-style `agentpay-wallet` challenge, tenant virtual wallet) and *on-chain mode* (local signer via `viem`, x402 + EIP-3009). |
| **Local financial firewall** | A `better-sqlite3`-backed policy engine (per-tx cap, 24h rolling budget, lifetime budget, whitelist matrix, circuit breaker). With wallet challenge mode enabled, it checks the quoted price before the gateway places a hold. |
| **3-state reconciliation** | `BuyerCoordinator` and `Reconciler` in `open-agentpay/state` enforce post-dispatch wire safety so network timeouts never release reserved budgets prematurely. |
| **Audit metadata** | Optional headers (`X-Session-ID`, `X-Task-ID`, `X-Tool-Name`) that group an agent's calls into sessions and tasks in the console timeline. |
| **AI Framework Integrations** | `createPaidTool` for LangChain.js (`open-agentpay/langchain`), `paidVercelTool` for Vercel AI SDK (`open-agentpay/vercel-ai`), and `createPaidMcpToolHandler` / `createPaidMcpFetch` for Model Context Protocol (`open-agentpay/mcp`). |

## Installation

Requires **Node.js 20.0+**.

```bash
# Install canonical SDK package from npm
npm install open-agentpay
# or with pnpm / yarn
pnpm add open-agentpay
yarn add open-agentpay
```

For local development from the repository root:

```bash
pnpm --filter open-agentpay build
```

---

## Subpath exports

The package provides modular subpath exports so you only import what you need:

| Import path | Main exports |
| --- | --- |
| `open-agentpay` | `AgentPayClient`, `createAgentPayFetch`, `LocalSigner`, plus all core, policy, state, and integration exports |
| `open-agentpay/client` | `AgentPayClient`, `AgentPayTransport`, `createAgentPayFetch`, `LocalSigner` |
| `open-agentpay/policy` | `PolicyConfig`, `SQLitePolicyEngine`, `WhitelistMode`, `BreakerState`, `createBuyerConnection`, `initBuyerDatabase`, `PolicyViolationError`, `PolicyErrorCode` |
| `open-agentpay/core` | `PaymentChallenge`, `PaymentAuthorization`, `WalletAuthorization`, x402 header codecs, `X402Error` |
| `open-agentpay/evm` | EIP-712 / EIP-3009 typed data builders, CAIP-2 network constants |
| `open-agentpay/state` | `BuyerCoordinator`, `Reconciler`, `IntentTransitionValidator` |
| `open-agentpay/langchain` | `createPaidTool`, `PaidStructuredTool`, `ToolException` |
| `open-agentpay/vercel-ai` | `paidVercelTool`, `createVercelPaidTool` |
| `open-agentpay/mcp` | `createPaidMcpToolHandler`, `createPaidMcpFetch` |

---

## The client

```typescript
import { AgentPayClient, LocalSigner } from 'open-agentpay';
import { PolicyConfig, SQLitePolicyEngine } from 'open-agentpay/policy';

const client = new AgentPayClient({
  signer: new LocalSigner(process.env.AGENT_PRIVATE_KEY!), // signs on-chain authorizations
  policy: SQLitePolicyEngine.createInMemory(new PolicyConfig({ ... })), // local financial firewall
  defaultHeaders: { ... },                                 // default headers, e.g. Authorization
  walletChallengeMode: true,                               // price-check API-key wallet calls before retry
});
```

| Option | Required | Purpose |
| --- | --- | --- |
| `signer` | yes | Signs EIP-712 / EIP-3009 authorizations (`ISigner`) for the **on-chain path** using `viem`. In wallet mode no on-chain signature is created, so it can be a placeholder key. |
| `policy` | yes | The local policy engine (`IPolicyEngine`). Evaluated before any payment. See [Policy Configuration](/guide/policy). |
| `defaultHeaders` | no | Default request headers sent on every call. `Authorization: Bearer <api_key>` authenticates the wallet request. |
| `baseUrl` | no | Optional base URL prepended to relative request paths. |
| `walletChallengeMode` | no | Set `true` for AgentPay Gateway wallet calls. The SDK receives the 402 quote, evaluates local policy, then retries with `PAYMENT-SIGNATURE`. Defaults to `false`. |
| `raiseOnPolicyRejection` | no | When `true`, throws `PolicyViolationError` immediately if the local firewall rejects a 402 challenge. When `false` (default), returns the 402 `Response`. |
| `baseFetch` | no | Custom `fetch` implementation (defaults to `globalThis.fetch`). |

### Drop-in `fetch` adapter

If an existing SDK or library expects a standard `fetch` function, use `client.asFetch()` or `createAgentPayFetch()`:

```typescript
import { createAgentPayFetch } from 'open-agentpay';

const paidFetch = client.asFetch();
// or: const paidFetch = createAgentPayFetch({ signer, policy });

const response = await paidFetch('https://gate.openagentpay.dev/v1/market-data?ticker=NVDA');
```

---

## Choosing a payment mode

The mode is chosen by the credentials on the request. There is no automatic fallback between modes.

| | **Wallet mode** | **On-chain mode** |
| --- | --- | --- |
| **Credential** | API key in `Authorization` header | Local private key (`LocalSigner`) |
| **Account needed** | Yes: tenant, project, agent, key from the [console](/guide/console) | No |
| **How it pays** | Gateway returns an `agentpay-wallet` price challenge; SDK policy approves before wallet Hold → upstream → Capture | Client signs an EIP-3009 authorization; Facilitator settles on-chain |
| **Latency / cost** | Internal ledger, no gas | Network confirmation, gas sponsored by the Facilitator |
| **Best for** | Fleets of agents inside one organization | Cross-organization calls, no prior relationship |

::: tip
If a wallet-mode request has insufficient balance, you get `402 ERR_WALLET_INSUFFICIENT_BALANCE`. The SDK does **not** retry on-chain. Switching modes is always an explicit decision in your code.
:::

### Price-aware wallet mode (API key)

Enable `walletChallengeMode: true` when using an AgentPay Gateway API key. The gateway's first response is a `402` with the exact route price; no wallet hold or upstream call happens yet. The SDK verifies `requestBinding` and `resourceUri`, checks the quote against its local policy, and reserves the local budget before retrying with a one-time `PAYMENT-SIGNATURE` wallet authorization.

```typescript
import { AgentPayClient, LocalSigner } from 'open-agentpay';
import { PolicyConfig, SQLitePolicyEngine } from 'open-agentpay/policy';

// Wallet mode makes no on-chain signature, so the signer is a placeholder.
// ⚠️ TESTNET DEVELOPMENT ONLY: This is a well-known public Hardhat key. NEVER deposit real funds!
const devKey =
  process.env.AGENT_PRIVATE_KEY ??
  '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';
const signer = new LocalSigner(devKey);

const policy = SQLitePolicyEngine.createInMemory(
  new PolicyConfig({
    id: 'wallet_firewall',
    name: 'Wallet Firewall',
    walletAddress: signer.address,
    maxPerTxBaseUnits: 100_000n,         // 0.10 USD per request (wallet minor units)
    rollingBudget24hBaseUnits: 5_000_000n, // 5.00 USD per rolling 24h
    whitelistDomains: ['gateway.example.com'],
  })
);

const client = new AgentPayClient({
  signer,
  policy,
  defaultHeaders: {
    Authorization: `Bearer ${process.env.AGENTPAY_API_KEY}`,
  },
  walletChallengeMode: true,
});

const resp = await client.get('https://gateway.example.com/v1/market-data', {
  params: { ticker: 'NVDA' },
});
console.log(resp.status, await resp.json());
```

### On-chain mode (x402)

```typescript
import { AgentPayClient, LocalSigner } from 'open-agentpay';
import { PolicyConfig, SQLitePolicyEngine, WhitelistMode } from 'open-agentpay/policy';

const signer = new LocalSigner(process.env.AGENT_WALLET_KEY!);
const policy = SQLitePolicyEngine.createInMemory(
  new PolicyConfig({
    id: 'onchain_firewall',
    name: 'On-Chain Firewall',
    walletAddress: signer.address,
    whitelistMode: WhitelistMode.SIMPLE,
    maxPerTxBaseUnits: 1_000_000n,          // 1.00 USDC
    rollingBudget24hBaseUnits: 10_000_000n, // 10.00 USDC
    whitelistPayees: ['0x70997970C51812dc3A010C7d01b50e0d17dc79C8'],
  })
);

const client = new AgentPayClient({ signer, policy }); // no API key needed

const resp = await client.get('https://gate.openagentpay.dev/v1/solana/smart-money', {
  params: { timeframe: '24h' },
});
console.log(await resp.json());
```

When the server answers `402` with a `PAYMENT-REQUIRED` challenge, the client performs the following steps transparently:

```text
1. Decode the challenge (price, payee, asset, network, expiry, scheme)
2. Policy: evaluate → atomically RESERVE budget in SQLite (reject here = no authorization is sent)
3. Select by scheme: sign EIP-3009 via viem, or create an `agentpay-wallet` authorization
4. Replay the same request with the `PAYMENT-SIGNATURE` header
5. On 2xx: COMMIT the local reservation; retain it on ambiguous timeout / `5xx`
```

::: warning Private keys & redaction
`LocalSigner` redacts private keys in `toString()`, `toJSON()`, and `nodejs.util.inspect.custom`. Always load keys from environment variables or a secrets manager, never from source code or prompts. Never expose a signer or policy object to the model as a tool.
:::

---

## Local financial firewall

The `policy` option accepts `SQLitePolicyEngine`, which supports both in-memory and durable file-backed SQLite databases via `better-sqlite3`:

| Mode | State | Use for |
| --- | --- | --- |
| `SQLitePolicyEngine.createInMemory(config)` | In memory (`:memory:`, lost on restart) | Tests, demos, short-lived agents |
| `new SQLitePolicyEngine(db, config)` | Durable SQLite file with WAL mode and `BEGIN IMMEDIATE` atomic reservations | Long-running agents where budget must survive restarts |

### Durable file-backed ledger example

```typescript
import {
  createBuyerConnection,
  initBuyerDatabase,
  PolicyConfig,
  SQLitePolicyEngine,
  WhitelistMode,
} from 'open-agentpay/policy';

const db = createBuyerConnection('./agentpay-buyer.sqlite');
initBuyerDatabase(db);

const policy = new SQLitePolicyEngine(
  db,
  new PolicyConfig({
    id: 'agent_firewall',
    name: 'Agent Firewall',
    walletAddress: signer.address,
    maxPerTxBaseUnits: 500_000n,          // 0.50 USDC
    rollingBudget24hBaseUnits: 5_000_000n, // 5.00 USDC
    lifetimeBudgetBaseUnits: 50_000_000n,  // Optional lifetime cap (50.00 USDC)
    circuitMaxRpm: 15,
    circuitConsecutiveFailures: 5,
    circuitCooldownSeconds: 300,
    whitelistMode: WhitelistMode.SIMPLE,
    whitelistPayees: ['0x742d35Cc6634C0532925a3b844Bc454e4438f44e'],
  })
);
```

All amounts are **integers in the smallest unit** (`bigint`, `number`, or integer `string`; floats and booleans are rejected with a `TypeError`). USDC has 6 decimals, so `500_000n` is 0.50 USDC. See [Policy Configuration](/guide/policy) for every rule.

---

## Agent identity and audit metadata

The gateway derives `agent_id` from the API key and strips any client-supplied `X-Agent-ID` header. You can attach optional, self-reported metadata headers to group calls in the console timeline:

| Header | Purpose |
| --- | --- |
| `X-Session-ID` | Group calls belonging to one workflow session |
| `X-Task-ID` | Group calls belonging to one task |
| `X-Tool-Name` | Name of the tool making the call |

```typescript
const client = new AgentPayClient({
  signer,
  policy,
  defaultHeaders: {
    Authorization: `Bearer ${apiKey}`,
    'X-Session-ID': 'session_nvda_analysis_001',
    'X-Task-ID': 'task_market_data_fetch',
    'X-Tool-Name': 'fetch_stock_quote',
  },
  walletChallengeMode: true,
});
```

---

## Idempotency and safe retries

1. **Send an `Idempotency-Key` for any paid request**, and reuse the **same key** when retrying the same logical operation.
2. The same key with the same request replays the stored result (`X-Cache: HIT`) at **no additional cost**.
3. **A timeout is not a failure.** After a payment signature has been sent on the wire, a network timeout or `5xx` does **not** release the local budget reservation prematurely. Use `Reconciler` from `open-agentpay/state` to safely unlock expired authorizations after verifying they were not spent on-chain.

```typescript
import { randomUUID } from 'node:crypto';

const idempotencyKey = randomUUID();

const resp = await client.post(
  'https://gateway.example.com/v1/report',
  { ticker: 'NVDA' },
  { headers: { 'Idempotency-Key': idempotencyKey } }
);
```

---

## Handling errors

By default (`raiseOnPolicyRejection: false`), a local policy rejection returns the `402` response without signing. Set `raiseOnPolicyRejection: true` if you want the client to throw `PolicyViolationError` directly:

```typescript
import { PolicyViolationError } from 'open-agentpay/policy';

try {
  const resp = await client.get('https://gateway.example.com/v1/market-data');
  if (resp.status === 200) {
    const data = await resp.json();
  } else if (resp.status === 402) {
    console.error('Payment refused or challenge rejected:', await resp.text());
  } else if (resp.status === 403 || resp.status === 429) {
    console.error('Blocked by gateway policy:', await resp.json());
  }
} catch (err) {
  if (err instanceof PolicyViolationError) {
    console.error(`Blocked by local firewall [${err.code}]:`, err.message);
  } else {
    throw err;
  }
}
```

---

## AI Framework Integrations

### LangChain.js (`open-agentpay/langchain`)

Wrap any tool function in `createPaidTool` (or `PaidStructuredTool`). Policy and x402 errors are automatically caught and converted into LangChain `ToolException` messages so the agent can handle rejection gracefully without crashing:

```typescript
import { createPaidTool } from 'open-agentpay/langchain';

const fetchStockQuote = createPaidTool({
  name: 'fetch_stock_quote',
  description: 'Fetch a real-time US stock quote (charged per call)',
  client,
  func: async ({ ticker }: { ticker: string }, context) => {
    const resp = await context!.client!.get('https://gateway.example.com/v1/market-data', {
      params: { ticker },
    });
    return await resp.json();
  },
});

const result = await fetchStockQuote.invoke({ ticker: 'NVDA' });
```

### Vercel AI SDK (`open-agentpay/vercel-ai`)

Wrap Vercel AI SDK tool definitions with `paidVercelTool`. When the local firewall rejects a call, it returns a structured `{ error: true, code, message }` object to the model:

```typescript
import { paidVercelTool } from 'open-agentpay/vercel-ai';
import { z } from 'zod';

const marketTool = paidVercelTool({
  description: 'Fetch DEX market data with financial firewall protection',
  parameters: z.object({ chain: z.string() }),
  client,
  execute: async ({ chain }, context) => {
    const res = await context!.client!.get(
      'https://gate.openagentpay.dev/v1/free/defi/dex-volume',
      { params: { chain } }
    );
    return await res.json();
  },
});
```

### Model Context Protocol (`open-agentpay/mcp`)

Use `createPaidMcpToolHandler` on an MCP server or `createPaidMcpFetch` on an MCP client transport:

```typescript
import { createPaidMcpFetch, createPaidMcpToolHandler } from 'open-agentpay/mcp';

// 1. Wrap an MCP server tool handler (returns McpToolResponse with isError: true on policy rejection)
const mcpHandler = createPaidMcpToolHandler(client, async (args: { ticker: string }, context) => {
  const res = await context!.client.get('https://gateway.example.com/v1/market-data', {
    params: { ticker: args.ticker },
  });
  return await res.json();
});

// 2. Or create a fetch function for MCP HTTP/SSE client transports
const mcpFetch = createPaidMcpFetch(client);
```

---

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `PolicyViolationError: UNMETERED_CREDENTIAL` | Request carries an `apk_...` API key header without `walletChallengeMode: true` or `unmeteredCredentialPolicy: 'allow' \| 'meter'`. Enable `walletChallengeMode: true` for gateway wallet routes. |
| `TypeError: maxPerTxBaseUnits must be an integer, got 0.05` | Amounts must be integer base units (`50_000n` or `50000` for 0.05 USDC), never floats. |
| `InvalidPrivateKeyError` | Private key must be a 32-byte hex string (with or without `0x` prefix) or 32-byte `Uint8Array`. |
| Every call blocked with `SINGLE_TX_LIMIT_EXCEEDED` | `maxPerTxBaseUnits` is below the route price. |
| Budget resets on process restart | `SQLitePolicyEngine.createInMemory` uses `:memory:`. Use `createBuyerConnection('./buyer.sqlite')` + `initBuyerDatabase(db)` for persistence. |

## Next steps

| I want to… | Read |
| --- | --- |
| Run everything locally | [5-Minute Quickstart](/guide/quickstart) |
| Use the Python SDK | [Python SDK](/guide/python-sdk) |
| Set spending limits | [Policy Configuration](/guide/policy) |
| Put an API behind a price | [Gateway Configuration](/guide/gateway) |
| Create agents and API keys | [Management Console](/guide/console) |
| See the wire protocol | [API Specs](/guide/api-specs) |
