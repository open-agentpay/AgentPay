<p align="center">
  <a href="https://openagentpay.dev/">
    <img src="https://raw.githubusercontent.com/open-agentpay/AgentPay/main/docs/public/logo.png" alt="AgentPay Logo" width="120" height="120" />
  </a>
</p>

<h1 align="center">Open AgentPay Python SDK (`open-agentpay`)</h1>

<p align="center">
  <strong>The Open-Source Financial Firewall & Micro-Payment Infrastructure for Autonomous AI Agents</strong>
</p>

<p align="center">
  <a href="https://pypi.org/project/open-agentpay/"><img src="https://img.shields.io/pypi/v/open-agentpay.svg" alt="PyPI Version"></a>
  <a href="https://openagentpay.dev/"><img src="https://img.shields.io/badge/Docs-Live-blue?logo=vitepress&logoColor=white" alt="Documentation"></a>
  <a href="https://x.com/OpenAgentPay"><img src="https://img.shields.io/badge/Follow-%40OpenAgentPay-black?logo=x&logoColor=white" alt="Follow @OpenAgentPay"></a>
  <a href="https://github.com/open-agentpay/AgentPay/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License: Apache-2.0"></a>
  <a href="https://github.com/open-agentpay/AgentPay"><img src="https://img.shields.io/badge/Protocol-x402_V2-orange.svg" alt="x402 V2"></a>
  <a href="https://www.python.org/"><img src="https://img.shields.io/badge/Python-3.10%2B-3776AB.svg?logo=python&logoColor=white" alt="Python 3.10+"></a>
  <a href="https://www.circle.com/usdc"><img src="https://img.shields.io/badge/Settlement-Circle_USDC-2775CA.svg" alt="Circle USDC"></a>
</p>

<p align="center">
  <a href="https://openagentpay.dev/">📚 Documentation</a> •
  <a href="https://github.com/open-agentpay/AgentPay">💻 GitHub Repository</a> •
  <a href="https://x.com/OpenAgentPay">🐦 Follow on X (@OpenAgentPay)</a> •
  <a href="https://github.com/open-agentpay/AgentPay/issues">🐛 Report Issue</a>
</p>

---

## 💡 What is AgentPay?

**AgentPay** equips autonomous AI agents with micro-payments (Circle USDC on Arbitrum Sepolia & Base Sepolia) governed by an in-process, non-custodial **Financial Firewall**. It transparently intercepts `HTTP 402 Payment Required` responses, validates policy limits, and signs off-chain authorizations—protecting agent wallets against runaway loops and LLM hallucinations.

> 📦 **Package Name Note**: The canonical PyPI distribution is [`open-agentpay`](https://pypi.org/project/open-agentpay/) (official aliases: `agentpay-core`, `agentpay-firewall`). Regardless of which package you install, you import `agentpay` in Python code.

---

## ⚡ Quickstart

### 1. Installation

```bash
pip install open-agentpay
```

*(Optional LangChain support)*:
```bash
pip install "open-agentpay[langchain]"
```

### 2. Basic Usage: On-Chain Mode with Financial Firewall

Transparently intercepts HTTP 402 challenges, enforces local policy limits, and signs off-chain EIP-712/EIP-3009 authorizations:

```python
import os
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import SQLitePolicyEngine, PolicyConfig

# 1. Initialize local signer (non-custodial private key, 0 gas required)
# ⚠️ TESTNET DEVELOPMENT ONLY: This fallback key is Hardhat/Anvil's public Account 0 key.
# NEVER transfer mainnet funds to this address! In production, inject via environment variable:
private_key = os.getenv("AGENT_PRIVATE_KEY", "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
signer = LocalSigner(private_key)

# 2. Configure local financial firewall
policy = SQLitePolicyEngine.create_in_memory(
    PolicyConfig(
        id="agent_firewall",
        wallet_address=signer.address,
        max_per_tx_base_units=100_000,  # Max $0.10 per call
        rolling_budget_24h_base_units=1_000_000,  # Daily cap $1.00
        circuit_max_rpm=5,  # Trip circuit breaker if > 5 calls/min
    )
)

# 3. Create client with automatic x402 challenge negotiation
client = AgentPayClient(signer=signer, policy=policy)

# 4. Transparent call (auto-pays 402 challenge via EIP-712 off-chain signature)
response = client.get("https://api.seller.com/v1/market-data?ticker=NVDA")
print(response.json())
```

### 3. Virtual Wallet Mode (API Key Path)

For sub-millisecond execution using a pre-funded virtual ledger:

```python
import os
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import SQLitePolicyEngine, PolicyConfig

private_key = os.getenv("AGENT_PRIVATE_KEY", "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
client = AgentPayClient(
    signer=LocalSigner(private_key),
    policy=SQLitePolicyEngine.create_in_memory(PolicyConfig(...)),
    headers={"Authorization": "Bearer agp_live_your_api_key"},
)

response = client.get("https://gateway.seller.com/v1/compute")
```

### 4. LangChain Tool Integration (`@paid_tool`)

Convert any Python function into a paid LangChain tool protected by the financial firewall:

```python
from agentpay.integrations.langchain import paid_tool


@paid_tool(name="fetch_stock_quote", description="Fetch real-time stock prices", client=client)
def fetch_stock_quote(ticker: str) -> dict:
    resp = client.get(f"https://api.seller.com/v1/market-data?ticker={ticker}")
    return resp.json()


# Invoked seamlessly by LangChain agents
result = fetch_stock_quote.invoke({"ticker": "NVDA"})
```

### 5. Async Client (`AsyncAgentPayClient`)

For high-concurrency agent workflows:

```python
import asyncio
from agentpay import AsyncAgentPayClient, LocalSigner
from agentpay.policy import SQLitePolicyEngine, PolicyConfig


async def main():
    async with AsyncAgentPayClient(signer=signer, policy=policy) as client:
        response = await client.get("https://api.seller.com/v1/market-data?ticker=NVDA")
        print(response.json())


asyncio.run(main())
```

---

## 🛡️ Core Capabilities

| Pillar | Description |
|---|---|
| **Zero-Trust Financial Firewall** | Enforces per-transaction caps, 24-hour rolling budgets, lifetime limits, and payee whitelists outside the LLM prompt. |
| **Autonomous Circuit Breaker** | Automatically halts agent loops and freezes spending when request frequency exceeds safety thresholds (`circuit_max_rpm`). |
| **Gasless Off-Chain Settlements** | AI Agents hold only Circle USDC; gas is sponsored on Arbitrum Sepolia & Base Sepolia via EIP-3009 off-chain authorizations. |
| **Agent Session & Audit Observability** | Automatic tracking headers (`X-Agent-Session-ID`, `X-Agent-Task-ID`) connect tool spending to tasks in the AgentPay Console. |
| **Fail-Closed Guarantees** | Any policy violation, budget overrun, or upstream failure aborts safely with zero funds deducted. |

---

## 📖 Documentation & Community

- **Official Documentation**: [https://openagentpay.dev/](https://openagentpay.dev/)
- **GitHub Repository**: [https://github.com/open-agentpay/AgentPay](https://github.com/open-agentpay/AgentPay)
- **Twitter / X**: [@OpenAgentPay](https://x.com/OpenAgentPay)
- **Issue Tracker**: [GitHub Issues](https://github.com/open-agentpay/AgentPay/issues)

---

## 📄 License

Apache-2.0 License. See [LICENSE](https://github.com/open-agentpay/AgentPay/blob/main/LICENSE) for details.
