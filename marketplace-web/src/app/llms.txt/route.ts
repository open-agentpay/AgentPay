import {
  PUBLIC_CONSOLE_URL,
  PUBLIC_GATEWAY_URL,
  PUBLIC_SITE_URL,
  formatNetworkLabel,
  formatUsdcPrice,
  getMarketplaceRoutes,
} from '@/lib/marketplace';

export const dynamic = 'force-dynamic';

export async function GET() {
  const routes = await getMarketplaceRoutes();

  const routeLines = routes
    .map((r) => {
      const priceStr = formatUsdcPrice(r.onchain_price_base_units || r.price_base_units);
      const netLabel = formatNetworkLabel(r.network, r.network_label);
      const summary =
        r.summary ||
        r.description_md.split('\n').find((line) => line.trim() && !line.trim().startsWith('#')) ||
        r.title;
      return `- **[${r.title}](${PUBLIC_SITE_URL}/hub/${r.slug})** (Slug: \`${r.slug}\`)
  - **Summary:** ${summary}
  - **Endpoint:** \`${PUBLIC_GATEWAY_URL}${r.path_pattern}\` (\`${(r.methods || ['GET', 'POST']).join('/')}\`)
  - **X402 Price:** \`${priceStr} USDC / req\` on ${netLabel} (\`${r.network}\`)
  - **Gateway 24h SLA:** \`${r.sla.success_rate_24h}% 200 OK\` | P95 \`${r.sla.p95_latency_ms}ms\` | \`${r.sla.paid_calls_24h.toLocaleString()}\` paid calls | Status: \`${r.sla.health_status.toUpperCase()}\`
  - **Machine Spec:** ${PUBLIC_SITE_URL}/hub/${r.slug}/llms.txt`;
    })
    .join('\n\n');

  const content = `# AgentPay — Financial Firewall & Autonomous Payment Infrastructure for AI Agents

> **Canonical Site:** ${PUBLIC_SITE_URL}  
> **Documentation:** ${PUBLIC_SITE_URL}/docs/  
> **Gateway Core:** ${PUBLIC_GATEWAY_URL}  
> **Management Console:** ${PUBLIC_CONSOLE_URL}  
> **License:** Apache-2.0 Open Source  
> **Protocol Standards:** HTTP 402 Payment Required (x402 V2), EIP-712, EIP-3009, Circle USDC

---

## 1. Executive Summary & Philosophy

**AgentPay** is an open-source, non-custodial financial firewall and autonomous micro-payment gateway designed specifically for AI Agents (LLMs, LangChain, CrewAI, AutoGen, MCP tools).

As autonomous agents execute complex multi-step reasoning, they increasingly invoke paid external APIs, proprietary datasets, and GPU compute. The x402 protocol answers *"how does an agent pay for an HTTP resource?"* AgentPay answers the more crucial question:

> **"Should this agent be allowed to pay, how much, to whom, and what happens when something goes wrong?"**

### The Core Architectural Rule
**An LLM may *propose* a payment, but it can never *authorize* one.**  
Every single transaction passes through a deterministic, non-LLM policy engine outside the prompt context that enforces spending caps, rolling budgets, and circuit breakers before any money moves.

---

## 2. Seven Critical Risks Solved by AgentPay

| Risk | What Goes Wrong in Production | How AgentPay Solves It |
| :--- | :--- | :--- |
| **1. Runaway Loops** | A buggy reasoning loop calls the same paid tool hundreds of times per second. | **Circuit Breaker**: Automatically trips from \`CLOSED\` to \`OPEN\` on abnormal RPM spikes; enforces strict rolling 24h budget limits. |
| **2. Prompt Injection** | An adversarial prompt tricks the agent into paying an attacker's address or an inflated fee. | **Zero-Trust Firewall**: Payee, asset, and network whitelists + hard \`max_per_tx\` cap evaluated completely outside the LLM context. |
| **3. Private Key Exposure** | Sensitive private keys or master API keys leak into logs or prompt contexts. | **Non-Custodial Local Signer**: Private keys 100% stay in the buyer's local signer/KMS and are never transmitted across the network. |
| **4. Double Charging** | Network timeouts are mistaken for failures, prompting retries and double deductions. | **Tri-State Decoupled State Machine**: Idempotency keys + independent modeling of intent, settlement, and fulfillment (*Timeout ≠ Failure*). |
| **5. Budget Race Conditions** | Concurrent agent threads check balance simultaneously and overspend beyond quota. | **Atomic Two-Phase Reservations**: SQLite/PostgreSQL ACID transactions (\`reserve -> commit\` or \`reserve -> release\`). |
| **6. Multi-Tenant Leakage** | Tenant A accesses or tampers with Tenant B's keys, routes, or balances. | **Strict Tenant Isolation**: Row-level tenant partitioning, server-side RBAC, and cryptographically verified agent identities. |
| **7. Rogue Agent Control** | An agent malfunctions or behaves unexpectedly in production. | **Instant Kill Switch**: Suspend agent or revoke keys; gateway halts downstream requests within milliseconds. |

---

## 3. Dual-Rail Architecture (Two Payment Paths)

AgentPay provides two deterministic payment rails:

### Rail A: Virtual Wallet & Ledger (Enterprise / Team Sub-Accounts)
- Ideal for organizations managing internal agent fleets, allocating department budgets, and keeping audit ledgers.
- **Workflow:** \`Agent Request (Bearer apk_live_...)\` -> \`Gateway Policy Check\` -> \`Wallet HOLD\` -> \`Forward to Upstream\` -> \`2xx: CAPTURE\` / \`4xx: RELEASE\` / \`5xx or Timeout: HOLD & Mark Needs Review\` (Never auto-double charge).

### Rail B: Native On-Chain x402 (Autonomous Inter-Agent Economy)
- Ideal for permissionless machine-to-machine trade without pre-existing accounts or credit cards.
- **Workflow:**
  1. Agent sends standard HTTP request to gateway endpoint.
  2. Gateway responds with \`HTTP 402 Payment Required\` (including price, payee address, CAIP-2 network, and nonce).
  3. Buyer client local policy engine validates budget and atomically reserves quota.
  4. Local signer signs an offline EIP-712 \`TransferWithAuthorization\` (EIP-3009) payload with **zero gas** charged to the buyer.
  5. Agent replays request with \`PAYMENT-SIGNATURE\` header.
  6. Gateway validates signature off-chain and forwards request to upstream service.
  7. Facilitator node sponsors gas and broadcasts settlement asynchronously on-chain (Base / Arbitrum).

---

## 4. Python SDK Quickstart

### Installation

\`\`\`bash
pip install open-agentpay
\`\`\`

### 5-Line LangChain & Agent Protection Example

\`\`\`python
import os
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import PolicyConfig, SQLitePolicyEngine

# 1. Initialize client with non-custodial local financial firewall
signer = LocalSigner(os.getenv("AGENT_WALLET_KEY", "0x..."))
policy = SQLitePolicyEngine.create_in_memory(
    PolicyConfig(
        id="agent_firewall",
        wallet_address=signer.address,
        max_per_tx_base_units=50000,           # Maximum allowed spend per request ($0.05 USDC)
        rolling_budget_24h_base_units=1000000, # 24-hour rolling budget ceiling ($1.00 USDC)
        circuit_max_rpm=10                     # Trip circuit breaker if tool exceeds 10 calls/minute
    )
)
client = AgentPayClient(signer=signer, policy=policy)

# 2. Invoke any paid X402 service discovered in the marketplace using standard HTTP requests
# The SDK automatically handles HTTP 402 handshake, local budget validation, and EIP-3009 signing
response = client.get(
    "${PUBLIC_GATEWAY_URL}/v1/solana/smart-money",
    params={"timeframe": "24h"}
)
print("Observation from paid tool:", response.json())
\`\`\`

---

## 5. Verified Live X402 API Directory (${routes.length} Active Verified Endpoints)

The following public endpoints are officially listed in the AgentPay Marketplace (${PUBLIC_SITE_URL}/marketplace) to test and validate end-to-end SDK payments:

${routeLines}

---

## 6. Machine & Crawler Integration Guidelines

- To fetch machine-readable Markdown for any route: send \`Accept: text/markdown\` header to \`${PUBLIC_SITE_URL}/hub/<slug>\` or access \`${PUBLIC_SITE_URL}/hub/<slug>/llms.txt\`.
- All pages embed Schema.org \`WebAPI\` and \`Product\` JSON-LD structured data with live SLA telemetry.
`;

  return new Response(content, {
    status: 200,
    headers: {
      'Content-Type': 'text/markdown; charset=utf-8',
      'Cache-Control': 'public, s-maxage=30, stale-while-revalidate=60',
      'X-Robots-Tag': 'all',
    },
  });
}
