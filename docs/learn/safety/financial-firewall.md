---
title: 金融防火墙机制
description: 探讨在传统 WAF 之外，面向自主智能体构建“金融防火墙”的维度与策略要求。
---

# 金融防火墙机制

传统 Web 应用防火墙（WAF）关注的是 SQL 注入、XSS 与流量清洗；而面对能够自主决策并调动资金的 AI Agent，系统需要一层全新的**金融防火墙（Financial Firewall）**。

金融防火墙的核心防线包括：
- **静态硬性限额**：单笔交易最大金额（`max_amount_per_tx`）。
- **时间窗口滚动预算**：24小时内累计支出上限（`rolling_budget_24h`）。
- **收款方白名单（Payee Whitelist）**：仅允许向经过审核或企业登记的合规服务地址付款。
- **动态风控评分**：根据 Agent 的行为模式与近期调用频率动态调整允许的额度。

::: tip 在 AgentPay 中
AgentPay 网关内嵌 Policy Engine，在扣款与转发请求前前置执行多维策略拦截。详见 [安全模型规范](/specs/security-model)。
:::
