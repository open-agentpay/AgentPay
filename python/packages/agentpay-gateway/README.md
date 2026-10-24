<p align="center">
  <a href="https://openagentpay.dev/">
    <img src="https://raw.githubusercontent.com/open-agentpay/AgentPay/main/docs/public/logo.png" alt="AgentPay Logo" width="120" height="120" />
  </a>
</p>

<h1 align="center">AgentPay Gateway (`agentpay-gateway`)</h1>

<p align="center">
  <strong>Autonomous Agent Payment Gateway & Reverse Proxy for API Monetization</strong>
</p>

<p align="center">
  <a href="https://pypi.org/project/agentpay-gateway/"><img src="https://img.shields.io/pypi/v/agentpay-gateway.svg" alt="PyPI Version"></a>
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

## ⚡ What is AgentPay Gateway?

`agentpay-gateway` sits directly in front of existing REST APIs, AI agent microservices, and compute endpoints. It monetizes incoming requests via standard **HTTP 402 Payment Required** challenges settled gaslessly on-chain (Circle USDC on Arbitrum Sepolia & Base Sepolia) or via virtual tenant wallets.

---

## 🚀 Quickstart

### 1. Installation

```bash
pip install agentpay-gateway
```

### 2. Configuration (`gateway.yaml`)

```yaml
version: "1.0"

server:
  host: "0.0.0.0"
  port: 8000
  db_path: "./data/gateway.db"

facilitator:
  # Demo URL (not yet live). For local development, use a local mock facilitator (e.g. http://localhost:8080/facilitator)
  url: "https://facilitator.openagentpay.dev"

settlement:
  default_network: "eip155:421614"                             # Arbitrum Sepolia
  default_asset: "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d"      # Circle USDC
  default_pay_to: "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

routes:
  - path: "/v1/market-data"
    upstream: "https://httpbin.org/get"
    price: "0.001 USDC"
```

> 💡 **Facilitator URL Note**: `https://facilitator.openagentpay.dev` is the planned public testnet demo service (not yet live). For local development, point `facilitator.url` to your local mock service (such as `http://localhost:8080/facilitator`) or use AgentPay's built-in `MockFacilitatorService`.

### 3. Running the Gateway

```bash
python3 -m agentpay_gateway.server --config ./gateway.yaml
```

---

## 🔒 Features

- **HTTP 402 x402 Protocol**: Issues standard challenges containing pricing, settlement token, network, and validity window.
- **Off-Chain Signature Verification**: Verifies EIP-712 authorizations with zero on-chain latency before forwarding upstream.
- **Idempotent Anti-Rollback**: Decoupled state machine guarantees no double-charging on network dropouts.
- **Facilitator Integration**: Coordinates with EVM facilitator nodes for asynchronous gasless settlement.

---

## 📖 Documentation & Community

- **Official Documentation**: [https://openagentpay.dev/](https://openagentpay.dev/)
- **Gateway Guide**: [https://openagentpay.dev/guide/overview.html](https://openagentpay.dev/guide/overview.html)
- **GitHub Repository**: [https://github.com/open-agentpay/AgentPay](https://github.com/open-agentpay/AgentPay)
- **Issue Tracker**: [GitHub Issues](https://github.com/open-agentpay/AgentPay/issues)

---

## 📄 License

Apache-2.0 License. See [LICENSE](https://github.com/open-agentpay/AgentPay/blob/main/LICENSE) for details.
