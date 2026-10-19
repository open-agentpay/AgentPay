---
title: Python SDK 指南
description: AgentPay Python SDK 客户端使用说明、身份绑定与异常处理。
---

# Python SDK 指南

`agentpay` 是专为 Python 编写的异步客户端 SDK，支持透明 x402 协商、Agent 身份绑定与 MCP 工具集成。

## 安装

```bash
pip install -e python/packages/agentpay
```

## 初始化客户端

```python
from agentpay import AgentPayClient

client = AgentPayClient(
    gateway_url="http://127.0.0.1:8080",
    api_key="apk_live_your_agent_key",
    agent_id="ag_01j7xyz890abcdef12345678"
)
```

## 主要功能

- **透明 x402 拦截**：遇到 HTTP 402 状态码时，SDK 自动解析支付报价头并驱动网关执行资金冻结与扣费。
- **Agent 上下文追踪**：在请求头中自动注入 `X-Agent-ID` 与 `X-Correlation-ID`，保证每笔微支付与对话上下文精准关联。
- **重试与断路器**：内置幂等性 Key（`Idempotency-Key`）生成与上游超时保护。
