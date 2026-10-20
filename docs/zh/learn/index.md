---
title: 学习地图
description: 从 Agent 基础到 Agent 支付，再到 AgentPay：理解自主 AI Agent 如何安全地花钱
---

# 学习地图

::: warning 实验性项目
AgentPay 目前处于实验阶段，**尚未经过独立安全审计，请勿用于生产环境的真实资金**。建议仅使用测试网和测试代币。
:::

Agent 正在从"回答问题"变成"执行任务"：调用 API、购买数据、使用算力，甚至雇佣其他 Agent。当 Agent 开始花钱，一个新问题随之出现：

> **在没有人逐笔确认的情况下，Agent 在什么条件下才有资格花钱？**

这个学习区先讲清楚问题的背景，再讲 AgentPay 是怎么回答它的。你不需要有区块链或支付领域的基础。

## 一句话认识 AgentPay

**LLM 可以提议付款，但不能批准付款。**

AgentPay 是放在 Agent 与真实资金之间的一层独立控制：单笔上限、滚动预算、白名单、频率熔断、失败恢复与审计，全部由确定性代码执行，而不是交给模型判断。

它与 x402 协议的分工是：

| | 回答的问题 |
|---|---|
| **x402** | 怎么付？ |
| **AgentPay** | 该不该付？付多少？失败了怎么办？ |

## 我该从哪里开始？

### 🚀 想先看效果：10 分钟体验

跑通三个场景：**正常付款成功 → 超额被拒绝 → 失控循环被熔断**。

→ [10 分钟 Demo](./05-hands-on/ten-minute-demo.md)

### 按你的角色选择学习路径

#### 我是 Agent 开发者（买方）

我想让自己的 Agent 能安全地调用付费 API。

1. [Agent 的常见失控方式](./01-agent-basics/agent-failure-modes.md)
2. [为什么需要金融防火墙](./04-agentpay-concepts/financial-firewall.md)
3. [预算预留：为什么不能"先检查后支付"](./04-agentpay-concepts/budget-reservation.md)
4. [超时不等于失败](./04-agentpay-concepts/timeout-is-not-failure.md)
5. [10 分钟 Demo](./05-hands-on/ten-minute-demo.md)

#### 我是 API 提供方（卖方）

我想把自己的 API 变成按次收费的接口。

1. [HTTP 402 与 x402](./02-agent-payments/http-402-and-x402.md)
2. [一次支付的完整生命周期](./04-agentpay-concepts/payment-lifecycle.md)
3. [在控制台配置第一个付费路由](./05-hands-on/first-paid-route.md)
4. [草稿、发布与回滚](./05-hands-on/draft-publish-rollback.md)

#### 我是企业管理员

我要管理多个 Agent，并对花出去的每一笔钱负责。

1. [Agent 基础设施全景](./03-infra-map/agent-stack.md)
2. [租户、项目、Agent 与 API Key](./04-agentpay-concepts/multi-tenant.md)
3. [虚拟钱包与链上支付：双轨架构](./04-agentpay-concepts/dual-rail.md)
4. [Agent 身份：观测与自报](./04-agentpay-concepts/agent-identity.md)
5. [用审计时间线排查问题](./05-hands-on/audit-timeline.md)

## 全部内容

学习区按"由大到小"组织：先了解 Agent 和 Agent 支付的通用知识，再看 AgentPay 在整套基础设施中的位置，最后深入概念和动手实践。

### 第 1 章：Agent 开发基础

不涉及 AgentPay 的通用知识，帮助你理解 Agent 为什么需要"财务边界"。

[什么是 Agent](./01-agent-basics/what-is-an-agent.md) · [工具调用](./01-agent-basics/tool-calling.md) · [MCP](./01-agent-basics/mcp.md) · [Agent 框架](./01-agent-basics/agent-frameworks.md) · [常见失控方式](./01-agent-basics/agent-failure-modes.md) · [自主度分级](./01-agent-basics/autonomy-levels.md)

### 第 2 章：Agent 支付

Agent 为什么要花钱、现有支付方式有什么局限、x402 和稳定币解决了什么。

[为什么 Agent 需要花钱](./02-agent-payments/why-agents-pay.md) · [支付方式对比](./02-agent-payments/payment-options.md) · [HTTP 402 与 x402](./02-agent-payments/http-402-and-x402.md) · [稳定币与 USDC](./02-agent-payments/stablecoins-and-usdc.md) · [免 Gas 授权](./02-agent-payments/gasless-eip-3009.md) · [微支付的成本](./02-agent-payments/micropayment-economics.md) · [五个信任问题](./02-agent-payments/trust-problems.md) · [生态概览](./02-agent-payments/ecosystem-landscape.md)

### 第 3 章：基础设施全景

把前两章的知识串起来，看 AgentPay 在整套 Agent 基础设施中处于哪一层、不做什么。

[Agent 技术栈分层](./03-infra-map/agent-stack.md) · [AgentPay 的位置](./03-infra-map/where-agentpay-fits.md) · [AgentPay 与 x402](./03-infra-map/agentpay-vs-x402.md)

### 第 4 章：AgentPay 核心概念

设计背后的"为什么"，每篇都从一个具体的故障场景讲起。

[金融防火墙](./04-agentpay-concepts/financial-firewall.md) · [支付生命周期](./04-agentpay-concepts/payment-lifecycle.md) · [超时不等于失败](./04-agentpay-concepts/timeout-is-not-failure.md) · [三态模型](./04-agentpay-concepts/three-state-model.md) · [预算预留](./04-agentpay-concepts/budget-reservation.md) · [双轨支付](./04-agentpay-concepts/dual-rail.md) · [Agent 身份](./04-agentpay-concepts/agent-identity.md) · [幂等与免费重放](./04-agentpay-concepts/idempotency.md) · [多租户](./04-agentpay-concepts/multi-tenant.md) · [通俗版威胁模型](./04-agentpay-concepts/threat-model-lite.md)

### 第 5 章：动手实践

基于 Python SDK、Go 网关与 Web 控制台的操作教程。

[10 分钟 Demo](./05-hands-on/ten-minute-demo.md) · [第一个付费路由](./05-hands-on/first-paid-route.md) · [配置策略并观察拦截](./05-hands-on/policy-in-action.md) · [读懂支付链路](./05-hands-on/read-payment-trace.md) · [审计时间线](./05-hands-on/audit-timeline.md) · [草稿、发布与回滚](./05-hands-on/draft-publish-rollback.md)

### 第 6 章：速查资料

[术语表](./06-resources/glossary.md) · [ID 前缀速查](./06-resources/id-prefixes.md) · [安全红线](./06-resources/safety-redlines.md) · [常见问题](./06-resources/faq.md) · [延伸阅读](./06-resources/further-reading.md)

## Learn、Guide 与 Specs 的区别

| 栏目 | 回答的问题 | 适合什么时候读 |
|---|---|---|
| **Learn**（本区） | 为什么这样设计？ | 想理解概念和思路 |
| **Guide** | 怎么做？ | 想动手部署和配置 |
| **Specs** | 精确的定义是什么？ | 想实现、集成或核对细节 |

Learn 只讲思路，不重复规范内容。每篇概念页末尾都会给出对应 Specs 的链接，供你深入阅读。

## 约定

- 金额统一以**最小整数单位**表示（USDC 为 6 位小数，`1 USDC = 1,000,000`），示例中不使用浮点数。
- 示例代码默认使用 **Python SDK**。
- 示例网络默认使用**测试网**（如 Arbitrum Sepolia）和测试代币。