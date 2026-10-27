# open-agentpay

> **The Open-Source Financial Firewall & Micro-Payment Infrastructure for Autonomous AI Agents (TypeScript / Node.js SDK)**

[![npm version](https://img.shields.io/npm/v/open-agentpay.svg)](https://www.npmjs.com/package/open-agentpay)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Documentation](https://img.shields.io/badge/Docs-Live-blue)](https://openagentpay.dev/)

`open-agentpay` provides autonomous AI agents with non-custodial financial guardrails, automated **HTTP 402 Payment Required** negotiation, and zero-gas offline cryptographic authorizations (EIP-712 / EIP-3009 transfer with authorization) in native TypeScript.

---

## ⚡ Features

- 🛡️ **Non-Custodial Financial Firewall**: Local SQLite-backed circuit breakers, 24h rolling budget enforcement, and strict whitelist matrix (EVM networks, token assets, payee addresses, domains).
- 🔄 **Transparent 402 Payment Interceptor**: `AgentPayClient` and `createAgentPayFetch` automatically intercept HTTP 402 challenges, evaluate local policies, sign authorizations, and replay requests.
- 🔐 **Pre-Signing Safety Guardrails**: Zero custom cryptography (backed strictly by `viem`), automated private key redaction, and strict reservation binding before signing.
- 🔒 **Post-Dispatch Wire Safety**: Once a payment signature is transmitted on the wire, upstream 5xx errors or network timeouts never release budget quotas prematurely; automated 3-state reconciler safely unlocks after on-chain expiration.
- 🤖 **AI Framework Ecosystem Native**: Drop-in tool wrappers for **LangChain.js** (`@langchain/core`), **Vercel AI SDK** (`ai`), and **Model Context Protocol** (`@modelcontextprotocol/sdk`).

---

## 📦 Installation

```bash
npm install open-agentpay
# or
pnpm add open-agentpay
# or
yarn add open-agentpay
```

---

## 🚀 5-Minute Quickstart

```typescript
import { AgentPayClient, LocalSigner } from 'open-agentpay';
import { PolicyConfig, SQLitePolicyEngine, WhitelistMode } from 'open-agentpay/policy';

// 1. Initialize local non-custodial signer (no gas fees paid by buyer)
// Provide your 32-byte hex EVM private key via environment variable
const privateKey = process.env.AGENT_WALLET_KEY ?? '0xYOUR_EVM_PRIVATE_KEY';
const signer = new LocalSigner(privateKey);

// 2. Configure local SQLite budget policy & circuit breaker
const policy = SQLitePolicyEngine.createInMemory(
  new PolicyConfig({
    id: 'agent_firewall',
    name: 'Agent Production Firewall',
    walletAddress: signer.address,
    whitelistMode: WhitelistMode.SIMPLE,
    maxPerTxBaseUnits: 2_500_000n, // Max 2.50 USDC per transaction
    rollingBudget24hBaseUnits: 5_000_000n, // Max 5.00 USDC per 24 hours
  })
);

// 3. Create client with automatic 402 handshake & payment retry
const client = new AgentPayClient({ signer, policy });

// 4. Request paid API endpoint directly
const response = await client.get('https://gate.openagentpay.dev/v1/free/defi/dex-volume', {
  params: {
    chain: 'ethereum',
    excludeTotalDataChart: true,
  },
});

console.log('HTTP Status:', response.status); // 200 OK!
const data = await response.json();
console.log('Result:', data);
```

---

## 🤖 AI Integrations

### LangChain.js

```typescript
import { createPaidTool } from 'open-agentpay/langchain';

const paidTool = createPaidTool({
  name: 'market_data_oracle',
  description: 'Query paid market volume data',
  client,
  func: async (args: { chain: string }, context) => {
    const res = await context!.client!.get('https://gate.openagentpay.dev/v1/free/defi/dex-volume', {
      params: { chain: args.chain },
    });
    return await res.json();
  },
});
```

### Vercel AI SDK

```typescript
import { paidVercelTool } from 'open-agentpay/vercel-ai';
import { z } from 'zod';

const marketTool = paidVercelTool({
  description: 'Fetch DEX market data with financial firewall protection',
  parameters: z.object({ chain: z.string() }),
  client,
  execute: async ({ chain }, context) => {
    const res = await context!.client!.get('https://gate.openagentpay.dev/v1/free/defi/dex-volume', {
      params: { chain },
    });
    return await res.json();
  },
});
```

### Model Context Protocol (MCP)

```typescript
import { createPaidMcpFetch, createPaidMcpToolHandler } from 'open-agentpay/mcp';

// Wrap MCP Server tool handlers
const mcpHandler = createPaidMcpToolHandler(client, async (args, context) => {
  const res = await context!.client.get('https://api.example.com/paid-resource');
  return await res.json();
});

// Or use drop-in fetch for MCP Client transports
const mcpFetch = createPaidMcpFetch(client);
```

---

## 📄 License

Apache-2.0 License. See [LICENSE](https://github.com/july-whj/AgentPay/blob/main/LICENSE) for details.
