---
title: 5 分钟快速开始
description: 快速上手 AgentPay 网关与 Python SDK，完成首次受控微支付调用。
---

# 5 分钟快速开始

本文将指导你在本地快速启动 AgentPay 网关，并运行一个模拟的 Agent 付费工具调用。

## 1. 启动基础设施与网关

使用 Docker Compose 启动 PostgreSQL 与 Go 网关：

```bash
docker compose up -d postgres
# 启动本地开发网关
cd gateway-go
go run ./cmd/gateway
```

网关默认在 `http://127.0.0.1:8080` 启动，管理接口在 `:8081`。

## 2. 安装 Python SDK

```bash
pip install -e python/packages/agentpay
```

## 3. 发起首次受控请求

```python
import asyncio
from agentpay import AgentPayClient

async def main():
    async with AgentPayClient(
        gateway_url="http://127.0.0.1:8080",
        api_key="apk_live_valid123"
    ) as client:
        # 请求经过网关金融防火墙过滤，自动完成 x402 扣费协商
        response = await client.get("http://127.0.0.1:8080/weather/current?city=Beijing")
        print("Response status:", response.status_code)
        print("Data:", response.json())

if __name__ == "__main__":
    asyncio.run(main())
```

::: tip 生产环境注意
当前项目仅限在测试网与沙箱环境中运行。更多配置请阅读 [Python SDK](/zh/guide/python-sdk) 与 [网关配置](/zh/guide/gateway)。
:::
