# AgentPay Gateway (`agentpay-gateway`)

[![PyPI Version](https://img.shields.io/pypi/v/agentpay-gateway.svg)](https://pypi.org/project/agentpay-gateway/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Python 3.10+](https://img.shields.io/badge/python-3.10+-blue.svg)](https://www.python.org/downloads/)

**Autonomous Agent Payment Gateway & Reverse Proxy for API Monetization.**

`agentpay-gateway` sits directly in front of existing REST APIs and AI microservices, monetizing requests via HTTP 402 Payment Required challenges settled gaslessly on-chain (Arbitrum Sepolia & Base).

---

## ⚡ Quickstart

### Installation
```bash
pip install agentpay-gateway
```

### Configuration (`gateway.yaml`)
```yaml
version: "1.0"

server:
  host: "0.0.0.0"
  port: 8000
  db_path: "./data/gateway.db"

facilitator:
  url: "http://127.0.0.1:8001"

settlement:
  default_network: "eip155:421614"                        # Arbitrum Sepolia
  default_asset: "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d" # Circle USDC
  default_pay_to: "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

routes:
  - path: "/v1/market-data"
    upstream: "https://httpbin.org/get"
    price: "0.001 USDC"
```

### Running the Gateway
```bash
python3 -m agentpay_gateway.server --config ./gateway.yaml
```

For complete documentation, visit [AgentPay Gateway Guide](https://github.com/july-whj/AgentPay/blob/main/specs/gateway-guide.md).
