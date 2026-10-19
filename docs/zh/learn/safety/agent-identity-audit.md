---
title: Agent 身份与行为审计
description: 构建不可变时间线审计系统，追踪每笔微支付背后的 Prompt、工具调用与 Agent 归属。
---

# Agent 身份与行为审计

当一笔可疑或异常的资金流动发生后，企业审计人员需要确切回答三个问题：
1. **是谁花钱？** —— 具体是哪一个 Agent 实例、属于哪一个团队/租户？
2. **为什么花钱？** —— 这笔支付是响应哪一段对话、执行哪一个工具意图触发的？
3. **资金去了哪里？** —— 目标收款地址、链上交易 Hash 与对应服务的 HTTP 状态码是什么？

传统后端日志往往只有散落的 HTTP 访问记录，缺乏与 Agent 上下文的强关联。不可变的审计事件（Audit Events）流是追溯 Agent 行为、应对监管合规的核心基础设施。

::: tip 在 AgentPay 中
AgentPay 记录了强绑定的 `AgentTimelineEvent` 与 `AuditEvents`，支持按意图与会话全程追溯。详见 [Agent 身份与行为审计规范](/zh/specs/agent-audit) 与 [审计事件规范](/zh/specs/audit-events)。
:::
