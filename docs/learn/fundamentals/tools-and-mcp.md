---
title: Tool Calling & Model Context Protocol (MCP)
description: How Tool Calling and the open Model Context Protocol (MCP) standard interact with machine payments.
---

# Tool Calling & Model Context Protocol (MCP)

The Model Context Protocol (MCP) standardizes how autonomous agents discover tools and query external context. Once an agent determines an action, it triggers external services via Tool Calling.

In paid environments, tool integrations fall into two paradigms:
- **Explicit Payment Tools**: The agent explicitly calls a financial tool (e.g. `pay_for_dataset(resource_id, max_quote)`).
- **Transparent HTTP Interception**: The agent queries standard REST/RPC endpoints, while the underlying HTTP transport catches `HTTP 402 Payment Required` responses, validates local budget policies, and negotiates payment automatically.

::: tip In AgentPay
AgentPay provides an async Python SDK and native MCP server tools supporting both transparent interception and explicit tool execution. See the [Python SDK Guide](/guide/python-sdk).
:::
