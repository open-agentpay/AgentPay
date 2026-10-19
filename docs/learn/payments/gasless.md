---
title: Gasless Micro-Payments & Gas Models
description: How meta-transactions, EIP-712/3009 authorizations, and sponsored settlement models enable friction-free agent transactions.
---

# Gasless Micro-Payments & Gas Models

When an API call costs $0.001 but on-chain gas fees are $0.05, the transaction is economically unviable.

To enable true machine-scale micro-payments, several gas mitigation strategies are required:
- **EIP-712 & EIP-3009 Meta-Transactions**: The buyer agent signs a structured transfer authorization (`transferWithAuthorization`) off-chain. The caller (Facilitator node) submits the transaction and sponsors the native gas token.
- **Layer 2 / Layer 3 Rollups**: Moving transaction execution to high-throughput networks such as Arbitrum Sepolia or Base.
- **Batch Aggregation**: The gateway verifies signatures locally and fulfills requests immediately, aggregating hundreds of payments into periodic batch multicalls.

::: tip In AgentPay
AgentPay implements an autonomous Facilitator service and formal gas sponsorship models. See the [Gas Economics & Settlement Models Specification](/specs/gas-models).
:::
