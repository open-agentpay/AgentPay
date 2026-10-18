# Open AgentPay (`open-agentpay`)

[![PyPI Version](https://img.shields.io/pypi/v/open-agentpay.svg)](https://pypi.org/project/open-agentpay/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Python 3.10+](https://img.shields.io/badge/python-3.10+-blue.svg)](https://www.python.org/downloads/)

**The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents.**

AgentPay equips autonomous AI agents with gasless micro-payments (Circle USDC on Arbitrum Sepolia & Base) governed by an in-process, non-custodial **Financial Firewall**.

---

## ⚡ Quickstart

### Installation
```bash
pip install open-agentpay
```

### Basic Usage: Transparent 402 Negotiation & Policy Guard
```python
from agentpay.client import AgentPayClient, LocalSigner
from agentpay.policy import SQLitePolicyEngine, PolicyConfig

# 1. Initialize local signer (non-custodial private key)
signer = LocalSigner("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")

# 2. Configure local financial firewall
policy = SQLitePolicyEngine.create_in_memory(
    PolicyConfig(
        id="agent_firewall",
        wallet_address=signer.address,
        max_per_tx_base_units=100_000,  # Max $0.10 per call
        rolling_budget_24h_base_units=1_000_000,  # Daily limit $1.00
        circuit_max_rpm=5,  # Tripped if > 5 requests/min
    )
)

# 3. Create client with automatic x402 challenge negotiation
client = AgentPayClient(signer=signer, policy=policy)

# 4. Transparent call (auto-pays 402 challenge via EIP-712 off-chain signature)
response = client.get("https://api.seller.com/v1/market-data?ticker=NVDA")
print(response.json())
```

### LangChain Paid Tool Integration
```python
from agentpay.integrations.langchain import PaidTool

market_tool = PaidTool(
    name="get_stock_quote",
    description="Fetch real-time stock price (costs $0.05 USDC per query)",
    endpoint="https://api.seller.com/v1/market-data",
    method="GET",
    client=client,
)
```

---

## 🔒 Security Guarantees
- **Non-Custodial**: Private keys never leave the agent's local trust domain.
- **Fail-Closed**: Any policy violation, budget exhaustion, or circuit trip aborts immediately with zero funds spent.
- **Gasless for Agents**: AI Agents hold only USDC; execution gas is sponsored via EIP-3009 by the seller or facilitator.

For complete documentation, visit [AgentPay GitHub](https://github.com/open-agentpay/AgentPay).
