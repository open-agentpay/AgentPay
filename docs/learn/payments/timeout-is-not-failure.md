---
title: Timeout Is Not Failure
description: "The distributed systems dilemma in payments: Handling timeouts without causing double-spending or fund leakage."
---

# Timeout Is Not Failure

In distributed computing, **a network timeout merely means no response was received within a given timeframe; it does not mean the transaction failed**.

If a client marks a timed-out request as "failed" and rolls back an authorization, severe financial loss can occur:
1. The upstream provider may have successfully charged and fulfilled the service, but the HTTP response was dropped by an intermediate network partition.
2. An on-chain transaction may have been broadcast to the mempool and mined seconds after the client timeout expired.

To prevent double-spending and unrecoverable loss, robust payment gateways implement an **explicit Unknown State** combined with **Hold TTL buffers**:
- When an upstream call times out, fulfillment enters `unknown` while funds remain `held`.
- Funds are never silently refunded until an asynchronous reconciliation worker or manual reviewer verifies delivery status.

::: tip In AgentPay
The AgentPay state machine maps `confirmed + unknown` to the composite status `SETTLED_FULFILLMENT_UNKNOWN_NEEDS_REVIEW`, backed by a 120-second default Hold TTL. See the [Payment State Machine Specification](/specs/state-machine).
:::
