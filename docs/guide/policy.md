---
title: 策略配置指南
description: 金融防火墙策略规则配置：限额、24小时预算与收款方白名单。
---

# 策略配置指南

AgentPay 通过 Policy Engine 为每个请求执行强约束校验。

## 策略规则示例

```yaml
policy_id: "pol_weather_default"
tenant_id: "ten_default"
max_amount_per_tx: 500000        # 单笔最大 0.5 USDC (6位小数)
rolling_budget_24h: 5000000      # 24小时累计上限 5.0 USDC
allowed_payees:
  - "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
circuit_breaker:
  consecutive_failures_limit: 3
  cooldown_seconds: 60
```

当请求超出单笔限额或滚动预算时，网关直接拦截并返回 `403 Forbidden` 与违规原因码。
