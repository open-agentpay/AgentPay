---
title: Agent Identity & Behavior Auditing
description: Building immutable event timelines linking transactions directly to prompts, agent IDs, and tool execution traces.
---

# Agent Identity & Behavior Auditing

When suspicious financial transactions occur, security and compliance teams must answer three critical questions:
1. **Who authorized the transaction?** Which agent instance and tenant initiated the payment?
2. **Why was it executed?** What prompt, user session, and tool invocation motivated the spend?
3. **Where did funds go?** What was the destination payee address, blockchain tx hash, and upstream HTTP status code?

Traditional web server access logs record HTTP endpoints but lose agent context. An append-only audit stream binding cryptographic credentials to execution timelines is essential for enterprise compliance.

::: tip In AgentPay
AgentPay emits immutable `AgentTimelineEvent` and `AuditEvents`, enabling end-to-end trace correlation from user prompt to ledger journal. See the [Agent Identity & Audit Specification](/specs/agent-audit) and [Audit Events Specification](/specs/audit-events).
:::
