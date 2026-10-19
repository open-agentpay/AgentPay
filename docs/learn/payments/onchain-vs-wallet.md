---
title: On-Chain Payments vs Virtual Wallets
description: Comparing direct on-chain settlement against prepaid custodial virtual wallets across latency, gas economics, and trust models.
---

# On-Chain Payments vs Virtual Wallets

Architects designing payment systems for autonomous agents face a fundamental architectural choice:

| Dimension | Direct On-Chain Payments | Prepaid Virtual Wallet |
|---|---|---|
| **Latency** | Multiple seconds (block confirmation times) | Sub-millisecond (in-memory & row-level locking) |
| **Transaction Gas** | Every micro-payment incurs blockchain gas fees | Zero gas per request (gas only incurred during top-up/settlement) |
| **Decentralization** | Fully trustless, verifiable by any third party | Relies on an immutable double-entry ledger |
| **High-Frequency Bursts** | Constrained by L1/L2 block gas limits & RPC rate-limiting | Scalable to thousands of requests per second |

In practice, **hybrid architectures** offer the best of both worlds: high-frequency internal micro-transactions run through an append-only virtual wallet ledger, while inter-organizational clearing settles on-chain.

::: tip In AgentPay
AgentPay unifies on-chain and virtual wallet payment methods under a single client interface. The gateway routes requests according to configuration without code changes. See the [Wallet & Ledger Specification](/specs/wallet-ledger).
:::
