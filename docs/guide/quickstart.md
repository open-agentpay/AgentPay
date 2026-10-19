---
title: 5-Minute Quickstart
description: Get up and running with the AgentPay gateway and Python SDK to execute your first policy-governed payment.
---

# 5-Minute Quickstart

This guide walks you through running the AgentPay gateway locally and making a controlled paid tool invocation.

## 1. Start Infrastructure & Gateway

Start PostgreSQL and the Go gateway using Docker Compose:

```bash
docker compose up -d postgres
# Run the local Go gateway
cd gateway-go
go run ./cmd/gateway
```

The gateway listens at `http://127.0.0.1:8080` for data-plane traffic and `:8081` for the admin API.

## 2. Install Python SDK

```bash
pip install -e python/packages/agentpay
```

## 3. Make Your First Governed Request

```python
import asyncio
from agentpay import AgentPayClient

async def main():
    async with AgentPayClient(
        gateway_url="http://127.0.0.1:8080",
        api_key="apk_live_valid123"
    ) as client:
        # Request passes through the gateway firewall and completes x402 payment negotiation automatically
        response = await client.get("http://127.0.0.1:8080/weather/current?city=SanFrancisco")
        print("Response status:", response.status_code)
        print("Data:", response.json())

if __name__ == "__main__":
    asyncio.run(main())
```

::: tip Sandbox Notice
AgentPay is currently intended for sandbox and testnet evaluation. For detailed configuration, refer to the [Python SDK Guide](/guide/python-sdk) and [Gateway Configuration](/guide/gateway).
:::
