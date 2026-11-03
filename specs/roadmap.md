# AgentPay Master Roadmap & Technical Evolution Plan

> **Version**: v2.0.0 (Official Baseline)  
> **Status**: **Approved Master Plan**  
> **Overview**: This document defines the unified, single source of truth for AgentPay roadmap initiatives across Enterprise Financial Firewall / AI FinOps, HTTP 402/x402 Micropayments, Dynamic Gas Surcharging, Batching & State Channels, Hardware Security Modules (KMS/HSM), Out-of-Process Sidecar Isolation, and Ecosystem Tooling. All milestone timelines are organized strictly by architecture readiness and production delivery gates rather than arbitrary calendar years.

---

## 1. Dual-Track Vision & Core Moat

Underlying payment settlement rails (USDC / EIP-3009 or enterprise fiat rails) serve as pluggable infrastructure. AgentPay's core defensible moat lies in the **Financial Firewall**, **Anti-Hanging Tri-State Machine (Hold/Capture/Release)**, and **Immutable Behavior Audit Blackbox** positioned between AI intelligence and monetary resources.

```text
                               ┌────────────────────────────────────────────────────────┐
                               │       AgentPay: Enterprise AI Gateway & FinOps Firewall │
                               └───────────────────────────┬────────────────────────────┘
              ┌────────────────────────────┬───────────────┴───────────────┬────────────────────────────┐
              ▼                            ▼                               ▼                            ▼
     [Theme 1: Enterprise HITL]   [Theme 2: FinOps & Fiat]       [Theme 3: Micropayment Rails]    [Theme 4: Ecosystem & SDKs]
     • Feishu/WeCom/DingTalk HITL • Multi-tier Tree Quotas        • Dynamic Gas Surcharge          • Native MCP Middleware
     • Out-of-Process Sidecar     • WeChat/Alipay/Bank Invoicing  • Batching & State Channels      • Langfuse Co-Observability
     • Cloud KMS / HSM Signing    • Non-Custodial Multi-Seller    • Multi-Chain (Solana/L2/4337)   • Java / Spring Boot Starter
     • Anomaly Detection & Alerts • E-Invoicing & Export          • Distributed Gateway Clusters   • x402-v2 Open Standard Spec
```

| Dimension | **Enterprise AI Security & FinOps** | **Global Open Agent Micropayment Protocol** |
|---|---|---|
| **Core Value** | **AI Agent Gateway & FinOps** | **HTTP 402 / x402 Micropayment Infrastructure** |
| **Target Audience** | Enterprise production deployments, Agent builders, SaaS providers | Autonomous AI agents, Web3 developers, API creators |
| **Pain Points** | Runaway spend loops, untracked department budgets, compliance audit | Cross-agent real-time settlement without traditional credit accounts |
| **Payment Rails** | Fiat virtual ledger (micro-units), corporate bank transfer, WeChat/Alipay | USDC, EIP-3009 Gasless authorization, EVM / L2 / Smart Accounts |

---

## 2. Core Pillars & Workstreams

### Pillar 1: Enterprise Security & Human-in-the-Loop (HITL)

#### 1.1 Human-in-the-Loop (HITL) IM Approvals
- **Tiered Risk Thresholds**:
  - **Micro transactions** ($\le$ threshold): Millisecond automated approval.
  - **Medium/High transactions**: Triggers interactive IM approval card.
  - **Excessive transactions**: Hard circuit breaker block (`SINGLE_TX_CAP_EXCEEDED`).
- **402 Asynchronous Suspend & Resume**: Gateway returns `402 Payment Required` with `approval_id` to release HTTP connections immediately while dispatching interactive approval cards to **Feishu / WeChat Work / DingTalk**.
- **One-Click Mobile Approval**: Managers approve or deny on mobile; client SDK polls or receives callback to replay request atomically with hold reservation.

#### 1.2 Out-of-Process Sidecar Isolation Daemon (Mitigating Threat N6)
- **Problem**: In in-process SDK modes, the LLM runtime shares memory with the interceptor. Malicious tools with code execution capabilities (`eval`) could tamper with in-memory limits.
- **Architecture**: Move the policy engine and signer into a dedicated **Sidecar Daemon** running under an isolated, unprivileged operating system user communicating via local IPC / Unix Domain Sockets.

#### 1.3 Cloud KMS & Hardware HSM Signers (Mitigating Threat N1)
- **Core Feature**: Plaintext private keys are never stored on disk or in environment variables.
- **Integrations**: AWS KMS, Google Cloud KMS, Alibaba Cloud KMS, Tencent Cloud KMS, and PKCS#11 HSM modules for institutional-grade compliance.

#### 1.4 Anomaly Detection & Dynamic Rate Limiting
- **Behavioral Profiling**: Track sliding-window spending rates, error spikes, and endpoint call frequencies to dynamically detect abnormal looping or compromised keys before hard budget exhaustion.

---

### Pillar 2: AI FinOps, Fiat Rails & Corporate Accounting

#### 2.1 Multi-Tier Organizational Tree Quotas
- **4-Tier Cost Allocation**: **Tenant $\to$ Department / Business Unit $\to$ Project $\to$ Individual Agent**.
- **Quota Enforcers**: Parent budgets cap total spend; child budgets receive hard/soft monthly quotas.

#### 2.2 Compliant Enterprise Fiat Rails
- **Two-Phase Architecture**:
  - **Phase 1 (Closed-loop Internal Compute/API Credits)**: Prepaid credits for platform-managed resources (zero custodial regulatory risk).
  - **Phase 2 (Multi-Seller Split Payments)**: Integrations with licensed payment institutions (WeChat Pay Partner, Alipay Direct, commercial bank escrow accounts).
- **Precision Conversion**: External transactions processed in minor units (e.g. cents) and converted to 6-decimal integer base units on ingestion ($\text{amount\_base\_units} = \text{payment\_minor\_amount} \times 10,000$).
- **Double Idempotency**: Dual-key deduplication (`external_ref` + `topup:{id}`) prevents duplicate credit ingestion from webhook retries.

#### 2.3 Reconciliation & Automated E-Invoicing
- Daily automatic reconciliation matching gateway ledger events with bank and payment gateway statements.
- Direct integration with electronic invoicing systems for corporate tax compliance.

---

### Pillar 3: Micropayment Protocols & Settlement Economics

#### 3.1 Dynamic Gas Surcharge & Reimbursement (Mode 2)
- **Goal**: Facilitator sponsoring nodes recover native gas expenses without charging buyers native gas tokens.
- **Mechanism**: Gateway monitors real-time L2 Base Fee and L1 Data Fee, estimates transaction gas, and folds the gas cost into the USDC challenge via price oracles. The buyer signs a single USDC payload covering both API cost and gas reimbursement.

#### 3.2 High-Frequency Batch Settlement & State Channels (Mode 3)
- **Goal**: Support streaming, per-token, or per-second billing with $>95\%$ gas reduction.
- **Mechanism**:
  - **Off-Chain Signature Aggregation**: Gateway verifies micro-authorizations locally and batches them via EIP-712 Multicall periodically.
  - **Bidirectional State Channels**: Buyer and seller open a state channel, exchange signed state updates off-chain, and settle the net balance on-chain upon closure.

#### 3.3 Multi-Chain & Account Abstraction Support
- Extend settlement beyond Arbitrum / Base Sepolia to Ethereum Mainnet, Polygon, Optimism, BNB Chain, and Solana.
- Integrate ERC-4337 Smart Contract Accounts with Session Key spending limits.

#### 3.4 Distributed Gateway Clusters & Five-Way Reconciliation
- Replace single-instance in-memory RCU with a distributed Raft/etcd sync layer for multi-node clusters.
- Persistent Five-Way Reconciliation background engine to sweep and recover ambiguous non-terminal settlements.

---

### Pillar 4: Ecosystem, SDKs & Developer Experience

#### 4.1 Native Model Context Protocol (MCP) Middleware
- MCP Server middleware providing one-line price declarations and challenge generation.
- MCP Client interceptor injecting financial firewall checks prior to executing tool calls.

#### 4.2 Co-Observability with Langfuse ("Brain & Hands")
- Clear separation of concerns:
  - **Langfuse**: Prompt engineering, token usage, LLM evaluations, tracing.
  - **AgentPay**: Financial firewall, circuit breaker, monetary spend, immutable audit trail.
- Correlate Langfuse `trace_id` with AgentPay payment intents for end-to-end auditability.

#### 4.3 Multi-Language Ecosystem Expansion
- **Java / Spring Boot Starter**: Annotation-driven (`@AgentPayGuard`) enterprise SDK for financial and corporate tech stacks.
- **Node.js Seller Middleware**: Express, Fastify, and NestJS decorators for seller API monetization.
- Continual enhancements to Python and TypeScript SDKs.

#### 4.4 Standardizing `x402-v2` Protocol Specification
- Publish formal RFC and standards proposals for enterprise two-phase wallet challenges and EIP-3009 gasless micropayments.

---

## 3. Priority Execution Matrix

| Priority | Workstream | Primary Deliverables |
|---|---|---|
| **P0 (Current Mainline)** | **Enterprise HITL & Fiat Top-Up** | Feishu/WeCom approval cards; WeChat/Alipay top-up; Multi-tier quota hierarchy |
| **P0 (Current Mainline)** | **Native MCP & Langfuse Integration** | Native MCP Server/Client wrappers; Langfuse Trace-to-Payment correlation |
| **P1 (Key Enhancements)** | **Dynamic Gas Surcharge (Mode 2)** | Oracle gas estimation in USDC challenge; zero-loss Facilitator gas recovery |
| **P1 (Key Enhancements)** | **Java/Spring Boot SDK & Middleware** | Spring Boot Starter; Node.js Express/Fastify seller packages |
| **P2 (Architecture Depth)** | **KMS/HSM & Out-of-Process Sidecar** | Keyless local memory; unprivileged Sidecar process defense against in-process tools |
| **P2 (Architecture Depth)** | **Batching & State Channels (Mode 3)** | Multicall signature aggregation; High-throughput streaming state channels |
| **P3 (Ecosystem Long-Term)**| **Multi-Chain & Open Protocol Spec** | Solana / ERC-4337 expansion; formal `x402-v2` RFC / EIP submission |
