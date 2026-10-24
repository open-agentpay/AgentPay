<p align="center">
  <a href="https://openagentpay.dev/">
    <img src="https://raw.githubusercontent.com/open-agentpay/AgentPay/main/docs/public/logo.png" alt="AgentPay Logo" width="120" height="120" />
  </a>
</p>

<h1 align="center">AgentPay Python Monorepo</h1>

<p align="center">
  <strong>The Open-Source Financial Firewall & Micro-Payment Infrastructure for Autonomous AI Agents</strong>
</p>

<p align="center">
  <a href="https://pypi.org/project/open-agentpay/"><img src="https://img.shields.io/pypi/v/open-agentpay.svg" alt="PyPI open-agentpay"></a>
  <a href="https://pypi.org/project/agentpay-gateway/"><img src="https://img.shields.io/pypi/v/agentpay-gateway.svg" alt="PyPI agentpay-gateway"></a>
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

## 📦 Packages Overview

This workspace contains the Python components of the AgentPay ecosystem:

| Package | Directory | Description | PyPI Distribution |
|---|---|---|---|
| **Buyer SDK** | [`packages/agentpay`](./packages/agentpay) | Core client, in-process financial firewall, EIP-712/3009 signer, and LangChain integration | [`open-agentpay`](https://pypi.org/project/open-agentpay/) |
| **Seller Gateway** | [`packages/agentpay-gateway`](./packages/agentpay-gateway) | Reverse proxy, x402 challenge negotiation, signature verification, and settlement coordination | [`agentpay-gateway`](https://pypi.org/project/agentpay-gateway/) |

> **Note on Importing**: Whether installed via `pip install open-agentpay` or alias packages (`agentpay-core`, `agentpay-firewall`), the import path in Python is always `import agentpay`.

---

## ⚡ Quickstart

### 1. Installation

#### From PyPI:
```bash
pip install open-agentpay agentpay-gateway
```

#### From Local Source (Development Mode):
```bash
# From the repository root
python3 -m venv .venv && source .venv/bin/activate
pip install -e python/packages/agentpay -e python/packages/agentpay-gateway
```

### 2. Basic Client Usage

```python
import os
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import SQLitePolicyEngine, PolicyConfig

# 1. Initialize local signer (non-custodial private key)
# ⚠️ TESTNET DEVELOPMENT ONLY: This fallback key is Hardhat's publicly known default test key.
# NEVER send real mainnet funds to this address! In production, inject via environment variable:
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

### 3. LangChain Paid Tool Integration

```python
from agentpay.integrations.langchain import paid_tool


@paid_tool(name="fetch_stock_quote", description="Fetch real-time stock prices", client=client)
def fetch_stock_quote(ticker: str) -> dict:
    resp = client.get(f"https://api.seller.com/v1/market-data?ticker={ticker}")
    return resp.json()


# Invoked seamlessly by LangChain agents
result = fetch_stock_quote.invoke({"ticker": "NVDA"})
```

---

## 🛠️ Testing & Development

Run test suites and linters from the repository root:

```bash
# Run all unit, integration, and e2e tests
make test

# Run code style & lint checks
make lint

# Validate protocol conformance test vectors
make vectors

# Run the terminal runaway circuit breaker demo
make demo
```

To build all PyPI distribution packages locally:

```bash
python3 scripts/build_pypi_dist.py
```

Built artifacts (`.whl` and `.tar.gz`) are output to `dist_all/`.

---

## 📖 Documentation & Community

- **Official Documentation**: [https://openagentpay.dev/](https://openagentpay.dev/)
- **GitHub Repository**: [https://github.com/open-agentpay/AgentPay](https://github.com/open-agentpay/AgentPay)
- **Twitter / X**: [@OpenAgentPay](https://x.com/OpenAgentPay)
- **Issue Tracker**: [GitHub Issues](https://github.com/open-agentpay/AgentPay/issues)

---

## 📄 License

Apache-2.0 License. See [LICENSE](https://github.com/open-agentpay/AgentPay/blob/main/LICENSE) for details.
