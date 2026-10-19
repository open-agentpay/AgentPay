---
title: 失控循环与熔断
description: 防止 Agent 陷入代码死循环、思考递归或重复重试耗尽资金的熔断器（Circuit Breaker）设计。
---

# 失控循环与熔断

智能体在遇到异常响应、上游报错或模型逻辑进入死循环时，可能触发高频并发重试，导致资金在几分钟内被迅速消耗殆尽，这就是**失控循环（Runaway Loop）**。

## 熔断机制的层级设计

1. **频次熔断（Rate Limiting）**：限制单个 Agent 每分钟或每小时发起的支付意图次数。
2. **失败率熔断（Error Ratio Breaker）**：如果连续多次调用上游均返回履约失败或未知状态，系统自动触发熔断，拒绝后续支付请求。
3. **全局紧急停机（Kill Switch）**：管理员可在控制台一键禁用特定 Agent 或租户的 API Key，或全局冻结支付转发。

::: tip 在 AgentPay 中
AgentPay 提供了全局与项目级的策略配置，内置熔断保护与一键吊销机制。详见 [多租户数据模型规范](/specs/multitenant-data-model)。
:::
