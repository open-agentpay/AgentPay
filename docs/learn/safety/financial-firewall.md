---
title: The Financial Firewall Mechanism
description: Designing an out-of-band financial firewall for autonomous agents beyond traditional application firewalls.
---

# The Financial Firewall Mechanism

Traditional Web Application Firewalls (WAFs) focus on SQL injection, XSS, and DDoS traffic. In contrast, autonomous AI agents require a **Financial Firewall** to govern economic behavior.

Key layers of a financial firewall include:
- **Static Per-Transaction Caps**: Maximum spend per tool call (`max_amount_per_tx`).
- **Rolling Time-Window Budgets**: Cumulative spend threshold over a 24-hour window (`rolling_budget_24h`).
- **Payee Whitelisting**: Restricting payments strictly to verified, registered payee addresses.
- **Dynamic Risk Evaluation**: Inspecting request rates and transaction velocity to identify anomalous spikes.

::: tip In AgentPay
The AgentPay embedded Policy Engine evaluates rules before any balance reservation or request forwarding occurs. See the [Security Model Specification](/specs/security-model).
:::
