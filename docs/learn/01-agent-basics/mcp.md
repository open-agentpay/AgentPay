# MCP: A Standard Way to Connect Agents with Tools

The Model Context Protocol (MCP) defines a common way for AI applications to connect to external context and capabilities. It addresses the integration problem where each model host and tool provider would otherwise need a custom connector. MCP is an integration protocol; it does not decide what an agent should do or whether an action is authorized.

## Roles and capabilities

An MCP **host** is the AI application. An MCP **client** in the host communicates with an MCP **server**, which may expose **tools** (operations the model may request), **resources** (data the client can read), and **prompts** (reusable templates). The host decides which servers and capabilities are available.

## Typical interaction

```text
Host connects to server → discovers tools/resources
Model requests a tool → host/client sends request
Server returns result → host provides it to model
```

Local and remote servers have different operational and security considerations. Treat each server as an integration with a trust boundary: review what data it receives, what operations it can perform, and how credentials are supplied. Apply least privilege and validate inputs and outputs.

Tool output may contain prompt injection. Treat it as untrusted data, not instructions that override application policy. Enforce high-impact actions outside the model.

MCP is not a payment protocol or financial control plane. A payment-enabled tool still needs recipient and amount validation, budgets, protected signing authority, and safe retry and settlement handling.
