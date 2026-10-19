---
title: Gasless 微支付与 Gas 模型
description: 探讨如何解决微支付场景下 Gas 费用占比过高问题，以及 EIP-712、ERC-4337 与 Gas 抽象机制。
---

# Gasless 微支付与 Gas 模型

当单笔微支付金额仅为 $0.001 时，若网络 Gas 费高达 $0.05，支付在经济学上将完全不可行。

为了实现真正的微支付，行业发展出多种 Gas 优化与免 Gas（Gasless）方案：
- **EIP-712 结构化签名授权**：客户端仅签名支付授权，由 Facilitator 或 Paymaster 代付 Gas 上链。
- **Layer 2 / Layer 3 Rollup**：将交易迁移到 Arbitrum、Base 等低成本网络。
- **账本聚合批量上链（Batch Settlement）**：网关在内部记录成百上千笔交易流水，定期汇总一次性上链清算。

::: tip 在 AgentPay 中
AgentPay 提供了内置的 Facilitator 服务与 Gas 模型规范，支持上游代付与聚合结算。详见 [Gas 与结算模型规范](/zh/specs/gas-models)。
:::
