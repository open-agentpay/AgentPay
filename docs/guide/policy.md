---
title: Policy Configuration Guide
description: Financial firewall rules, per-transaction caps, 24-hour rolling budgets, and payee whitelisting.
---

# Policy Configuration Guide

AgentPay protects your capital by enforcing strict limits before authorizing any payment forwarding.

## Policy Rule Definition

```yaml
policy_id: "pol_weather_default"
tenant_id: "ten_default"
max_amount_per_tx: 500000        # Max 0.5 USDC per tx (6 decimal places)
rolling_budget_24h: 5000000      # 24-hour rolling budget 5.0 USDC
allowed_payees:
  - "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
circuit_breaker:
  consecutive_failures_limit: 3
  cooldown_seconds: 60
```

When an agent invocation exceeds configured limits, the gateway instantly blocks the request and returns `403 Forbidden` with a canonical violation error code.
