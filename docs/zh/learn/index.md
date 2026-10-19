---
title: Agent 支付知识库
description: 面向开发者的 AI Agent 支付与资金安全入门：x402、链上与预付余额、防火墙、失控循环与审计。
---

# Agent 支付知识库

这里讲的是 **Agent 如何安全地花钱**，内容独立于 AgentPay 产品，可以单独阅读。
每篇文章末尾会用“在 AgentPay 中”提示框，说明对应的实现与配置。

## 从哪里开始

| 你想了解 | 建议阅读 |
|---|---|
| Agent 为什么会需要付款 | [Agent 为什么需要花钱](./fundamentals/why-agents-pay) |
| HTTP 402 / x402 是怎么工作的 | [HTTP 402 与 x402](./payments/x402) |
| 链上支付和预付余额怎么选 | [链上支付 vs 预付余额](./payments/onchain-vs-wallet) |
| 为什么超时不能当作失败 | [超时不等于失败](./payments/timeout-is-not-failure) |
| 如何防止 Agent 乱花钱 | [金融防火墙](./safety/financial-firewall) |
| 术语速查 | [术语表](./glossary) |

::: tip 在 AgentPay 中
知识库里的每个概念，在 [规范](/zh/specs/) 中都有严格定义，在 [使用文档](/zh/guide/overview) 中有配置方法。
:::

::: warning 实验性
涉及资金与安全的内容仅供学习与评估，AgentPay 尚未经过独立安全审计。
:::
