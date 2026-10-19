---
title: Python SDK Guide
description: Documentation for the AgentPay Python asynchronous client SDK, agent identity bindings, and interceptor patterns.
---

# Python SDK Guide

`agentpay` is the official Python async SDK providing transparent x402 protocol negotiation, agent identity tracing, and native MCP tool adapters.

## Installation

```bash
pip install -e python/packages/agentpay
```

## Client Initialization

```python
from agentpay import AgentPayClient

client = AgentPayClient(
    gateway_url="http://127.0.0.1:8080",
    api_key="apk_live_your_agent_key",
    agent_id="ag_01j7xyz890abcdef12345678"
)
```

## Core Features

- **Transparent x402 Interception**: Upon encountering `HTTP 402 Payment Required`, the client transparently decodes challenges and coordinates settlement with the gateway.
- **Agent Context Tracing**: Automatically populates `X-Agent-ID` and `X-Correlation-ID` headers to bind each micro-transaction to dialogue sessions.
- **Idempotency & Retries**: Generates deterministic `Idempotency-Key` headers and handles network timeouts safely without duplicating spend.
