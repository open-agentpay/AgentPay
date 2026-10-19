---
title: Runaway Loops & Circuit Breakers
description: Preventing agents from draining funds in retry storms, reasoning loops, or recursive tool invocations.
---

# Runaway Loops & Circuit Breakers

When an agent encounters unexpected upstream responses, malformed JSON, or reasoning loops, it may rapidly retry operations, draining allocated budgets within minutes. This failure mode is known as a **Runaway Loop**.

## Circuit Breaker Architecture

1. **Rate Limiting**: Throttling the number of payment authorizations an agent can trigger per minute or hour.
2. **Failure Ratio Breaker**: If consecutive calls return upstream failures or timeout errors, the circuit trips automatically, rejecting subsequent payment requests until cooled down.
3. **Emergency Kill-Switch**: Administrators can instantly revoke a compromised agent's API key or halt all gateway routing.

::: tip In AgentPay
AgentPay provides tenant-level and project-level policy configurations with built-in circuit breaking. See the [Multitenant Data Model Specification](/specs/multitenant-data-model).
:::
