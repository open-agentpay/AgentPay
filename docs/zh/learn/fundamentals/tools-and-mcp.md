---
title: Tool Calling 与 MCP
description: 了解 Tool Calling 与 Model Context Protocol（MCP）如何在支付场景中协同工作。
---

# Tool Calling 与 MCP

Model Context Protocol (MCP) 是当前主流的智能体工具与上下文连接标准。智能体在感知环境并做出决策后，通过 Tool Calling 调用外部工具完成操作。

在支付场景下，工具调用分为两类：
- **主动支付工具**：Agent 自主决定向某个服务商购买资源（例如 `pay_for_resource(endpoint, amount)`）。
- **透明 HTTP 握手代理**：Agent 正常调用数据查询工具，底层的通信中间件自动捕获 `HTTP 402 Payment Required` 状态并完成代扣。

::: tip 在 AgentPay 中
AgentPay 提供了原生 Python SDK 与 MCP Server 适配，支持透明代理与显式工具调用两种模式。详见 [Python SDK 指南](/zh/guide/python-sdk)。
:::
