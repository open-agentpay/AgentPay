# AgentPay 技术方案与产品规划文档

> **项目定位：AI Agent 的 Financial Firewall（金融防火墙）与自主支付基础设施**
>
> Repository: `open-agentpay/AgentPay`  
> License: Apache-2.0  
> 当前阶段：Experimental / Early-stage / Devnet evaluation  
> 文档基于当前仓库结构与 README 对项目的分析整理；当前仓库明确提示尚未经过安全审计，不应使用于生产主网资金。  

---

## 1. 项目摘要

AgentPay 的核心价值不是简单地“让 AI 能够支付”，而是建立一层位于 **AI Agent 与真实资金之间的安全控制层**：

```text
                    AI Agent
                       │
                 Tool / API Call
                       │
                       ▼
              ┌─────────────────┐
              │    AgentPay     │
              │ Financial       │
              │ Firewall        │
              ├─────────────────┤
              │ Identity        │
              │ Policy          │
              │ Budget          │
              │ Rate Limit      │
              │ Risk Control    │
              │ Audit           │
              └────────┬────────┘
                       │
                  Allow / Deny
                       │
                       ▼
                    x402
                       │
                       ▼
               Wallet / Facilitator
                       │
                       ▼
                  Blockchain
```

传统支付解决的是：

> **人如何付款。**

AgentPay 要解决的是：

> **AI 在没有人工逐笔确认的情况下，什么情况下可以花钱、最多花多少钱、可以向谁付款、发生异常时如何停止，以及支付和服务交付不一致时如何恢复。**

因此，AgentPay 的长期产品定位应从：

> Open-source payment infrastructure for autonomous AI agents

进一步强化为：

> **The Financial Firewall for AI Agents**

---

# 2. 背景与问题定义

## 2.1 Agent 正从“回答问题”变成“执行任务”

传统 LLM：

```text
用户
 ↓
Prompt
 ↓
LLM
 ↓
文本结果
```

Agent：

```text
用户
 ↓
任务
 ↓
Agent
 ├── Search
 ├── API
 ├── MCP Tool
 ├── Database
 ├── Other Agent
 ├── Compute
 └── Paid Service
 ↓
最终结果
```

Agent 的能力边界从“产生文本”逐渐扩展到：

- 调用外部 API
- 获取付费数据
- 使用计算资源
- 调用其他 AI Agent
- 购买 SaaS 能力
- 自动执行任务
- 自动完成多步骤工作流

因此，Agent 将逐渐拥有一种新的能力：

> **Economic Agency / Spending Capability**

即 AI 不只是“做决定”，而开始拥有“执行经济行为”的能力。

---

# 3. 核心痛点

## 3.1 AI 不应该直接拥有无限支付权限

如果 Agent 直接持有私钥：

```text
Agent
 ↓
Private Key
 ↓
Wallet
 ↓
Unlimited Funds
```

风险非常高。

LLM 可能出现：

- hallucination
- prompt injection
- tool misuse
- recursive loop
- unexpected retry
- malicious tool
- 错误金额
- 错误商户
- 无限调用

因此需要：

```text
Agent
 ↓
Payment Request
 ↓
Independent Policy Engine
 ↓
Allow / Reject
 ↓
Payment
```

核心安全原则：

> **LLM 可以提出支付请求，但不能自行授予自己支付权限。**

---

# 4. AgentPay 的核心价值

## 4.1 Financial Firewall

AgentPay 最核心的创新不是 Payment SDK，而是：

> **Financial Firewall / Spending Policy Engine**

它应该成为 Agent 与资金之间的 Zero-Trust Boundary。

### 典型规则

```yaml
agent:
  name: research-agent

budget:
  per_transaction: 1
  daily: 20
  monthly: 200

allowed_merchants:
  - search-api
  - data-api
  - llm-api

rules:
  max_calls_per_minute: 20
  approval_required_above: 5
```

Agent 请求：

```text
Pay $0.20 to data-api
```

Policy：

```text
✓ Merchant allowed
✓ Per transaction <= $1
✓ Daily budget remaining
✓ Frequency normal

=> ALLOW
```

如果 Agent 请求：

```text
Pay $20
```

则：

```text
✕ Per transaction limit exceeded

=> DENY
```

Agent 无法通过自然语言、Tool Calling 或重新调用 Policy API 来绕过限制。

---

# 5. 产品定位

## 5.1 推荐定位

### The Financial Firewall for AI Agents

核心价值主张：

> **Let AI spend money without giving AI unlimited control over money.**

中文：

> **让 AI 可以自主花钱，但不能自主突破资金权限。**

---

# 6. 系统总体架构

```text
┌───────────────────────────────────────────────┐
│                 User / Developer              │
└──────────────────────┬────────────────────────┘
                       │
                       ▼
┌───────────────────────────────────────────────┐
│                  AI Agent                     │
│                                               │
│ LLM / LangChain / CrewAI / AutoGen / MCP      │
└──────────────────────┬────────────────────────┘
                       │ Payment Request
                       ▼
┌───────────────────────────────────────────────┐
│             AgentPay Financial Firewall       │
│                                               │
│  Identity                                      │
│  Policy Engine                                 │
│  Budget Engine                                 │
│  Merchant Whitelist                            │
│  Rate Limiter                                  │
│  Circuit Breaker                               │
│  Risk Engine                                   │
└──────────────────────┬────────────────────────┘
                       │ Approved
                       ▼
┌───────────────────────────────────────────────┐
│                    x402                       │
│          HTTP 402 Payment Protocol             │
└──────────────────────┬────────────────────────┘
                       │
                       ▼
┌──────────────────────┴────────────────────────┐
│                 Wallet Layer                  │
│                                               │
│ Local Signer / KMS / Custodial Boundary       │
└──────────────────────┬────────────────────────┘
                       │
                       ▼
┌───────────────────────────────────────────────┐
│                Facilitator                   │
│                                               │
│ Verify / Settle / Relay                       │
└──────────────────────┬────────────────────────┘
                       │
                       ▼
┌───────────────────────────────────────────────┐
│             EVM / Base / USDC                 │
└───────────────────────────────────────────────┘
```

---

# 7. 核心交易流程

AgentPay 当前设计围绕 x402 V2，并将 Policy、签名、验证、结算和资源交付解耦。仓库 README 描述了以下基本流程。

```text
1. Agent 请求资源
        ↓
2. Seller 返回 HTTP 402
        ↓
3. AgentPay 解析 Payment Challenge
        ↓
4. Policy Engine 检查
        ↓
5. Budget Reserve
        ↓
6. Agent 签署 EIP-712 / EIP-3009 Authorization
        ↓
7. 带 PAYMENT-SIGNATURE 重试
        ↓
8. Seller → Facilitator /verify
        ↓
9. 验证成功
        ↓
10. Seller 返回资源
        ↓
11. Facilitator /settle
        ↓
12. Blockchain Settlement
        ↓
13. Reconciliation
```

关键设计：

> **资源交付与链上 Settlement 不应该被视为一个同步操作。**

---

# 8. Tri-State State Machine

AgentPay 一个重要的工程设计是把：

1. Payment Intent
2. Settlement
3. Fulfillment

拆成三个独立状态机。

## 8.1 Payment Intent

```text
CREATED
   ↓
AUTHORIZED
   ↓
VERIFIED
   ├── COMPLETED
   ├── REJECTED
   ├── EXPIRED
   └── CANCELLED
```

## 8.2 Settlement

```text
NOT_STARTED
   ↓
SUBMITTED
   ↓
PENDING
   ├── CONFIRMED
   ├── FAILED
   ├── REORGED
   └── UNKNOWN
```

## 8.3 Fulfillment

```text
NOT_STARTED
   ↓
PROCESSING
   ├── DELIVERED
   ├── FAILED
   └── RECOVERY_REQUIRED
```

---

# 9. 为什么必须拆状态机

例如：

```text
Agent
 ↓
支付 $0.1
 ↓
链上交易已经成功
 ↓
API 响应超时
```

错误实现可能认为：

```text
Timeout = Payment Failed
```

然后：

```text
Retry
 ↓
再次支付 $0.1
```

结果：

```text
第一次支付成功
第二次支付成功
```

造成 Double Charge。

正确逻辑：

```text
Network Timeout
       ↓
UNKNOWN
       ↓
Query Settlement
       ↓
Confirmed?
 ┌─────┴─────┐
Yes          No
 ↓            ↓
Continue     Recovery
```

核心原则：

> **Network timeout != failure**

---

# 10. Idempotency 与 Reconciliation

这是 AgentPay 从 Demo 走向真实支付基础设施必须解决的问题。

## 10.1 Idempotency Key

每一个支付请求都应该拥有：

```text
idempotency_key
```

建议组成：

```text
tenant_id
+
agent_id
+
payment_intent_id
```

数据库层建立唯一约束。

例如：

```sql
UNIQUE (
  tenant_id,
  idempotency_key
)
```

---

## 10.2 Budget Reservation

不能简单：

```text
check budget
↓
payment
```

否则并发 Agent 会产生 Race Condition。

应该：

```text
Check
 ↓
Atomic Reserve
 ↓
Payment
 ↓
Commit / Release
```

例如：

```text
Daily Budget = $10

Request A = $6
Request B = $6
```

如果非原子：

```text
A sees $10
B sees $10

A approve
B approve

Total = $12
```

如果 Atomic Reservation：

```text
A reserve $6
B sees remaining $4

B reject
```

---

# 11. Gasless Micro-Settlement

AgentPay 使用 EIP-712 / EIP-3009 类授权机制，使 Agent 不必为每一笔微支付直接管理原生 Gas Token。

逻辑：

```text
Agent
 ↓
Generate Authorization
 ↓
Sign
 ↓
Facilitator
 ↓
Sponsor Gas
 ↓
Submit On-chain
```

价值：

> Agent 不需要为了支付 0.001 USDC 的 API 费用而持有 ETH/MATIC 等 Gas Token。

---

# 12. MCP + AgentPay

MCP 是非常值得重点建设的生态入口。

可以形成：

```text
MCP
 ↓
Tool Discovery
 ↓
Paid Tool
 ↓
AgentPay
 ↓
Policy
 ↓
x402
 ↓
USDC
```

例如：

```text
MCP Server

Tools:

company.lookup
$0.02

market.research
$0.05

translation
$0.001
```

Agent：

```text
Need company.lookup
```

AgentPay：

```text
Price: $0.02

Policy:
✓ Merchant allowed
✓ Under transaction limit
✓ Budget available
✓ Rate normal

=> PAY
```

然后返回结果。

这使：

> **MCP = Tool Layer**

> **x402 = Payment Layer**

> **AgentPay = Financial Control Layer**

形成清晰的三层架构。

---

# 13. Agent-to-Agent Commerce

长期最大的想象空间不是：

```text
Human → API
```

而是：

```text
Agent A
   ↓
Agent Marketplace
   ↓
Agent B
   ↓
Agent C
```

例如：

```text
User:
帮我找一份新加坡 AI 公司数据库。
```

Research Agent 自动搜索：

```text
Provider A   $0.20
Provider B   $0.08
Provider C   $0.12
```

自动选择：

```text
Provider B
 ↓
HTTP 402
 ↓
AgentPay Policy
 ↓
ALLOW $0.08
 ↓
x402
 ↓
Dataset
```

整个过程无需人工点击支付。

这就是：

> **Agent Commerce**

而 AgentPay 的定位可以是其中的 Financial Middleware。

---

# 14. Agent Wallet

下一阶段可以引入 Agent Wallet / Spending Account 概念。

不要：

```text
Human Wallet
 ↓
Private Key
 ↓
Agent
```

而是：

```text
Human Wallet
       ↓
AgentPay Vault
       ↓
 ┌─────┼─────┐
 ▼     ▼     ▼
Agent A  Agent B  Agent C
 $10/day  $20/day  $5/day
```

用户授予：

> **Spending Capability**

而不是：

> **Unlimited Private Key**

---

# 15. 钱包安全架构

建议采用分层设计：

```text
Policy Layer
     │
     ▼
Spending Authorization
     │
     ▼
Signer Abstraction
     │
 ┌───┴──────────────┐
 ▼                  ▼
Local Signer       KMS
Dev/Test            Production
```

Production 环境不应该要求 Agent 直接读取私钥。

推荐：

```text
Agent
 ↓
Policy
 ↓
Sign Request
 ↓
KMS / HSM
 ↓
Signature
```

---

# 16. 核心模块设计

当前仓库已经按照较清晰的模块化方向组织，包括 `specs`、TypeScript、Python、Java/Spring Boot、独立服务和示例工程。

建议最终形成以下逻辑模块：

```text
agentpay/
├── core
├── protocol
│   └── x402
├── policy
├── budget
├── wallet
├── signer
├── facilitator
├── reconciliation
├── gateway
├── mcp
├── audit
├── identity
└── risk
```

---

# 17. Core

负责：

- Payment Intent
- Transaction
- Agent
- Merchant
- Policy
- State Machine
- Idempotency

核心原则：

> Core 不应该依赖具体 Web Framework。

---

# 18. Policy Engine

Policy Engine 是整个项目最值得形成壁垒的模块。

## Policy 维度

### Transaction

```text
max_transaction_amount
```

### Time

```text
daily_budget
weekly_budget
monthly_budget
```

### Lifetime

```text
lifetime_limit
```

### Merchant

```text
merchant_whitelist
merchant_blacklist
```

### Resource

```text
allowed_resources
blocked_resources
```

### Frequency

```text
requests_per_minute
requests_per_hour
```

### Approval

```text
amount > threshold
→ Human Approval
```

---

# 19. Circuit Breaker

用于防止 Agent Loop。

例如：

```text
Agent
 ↓
Tool
 ↓
Payment
 ↓
Tool
 ↓
Payment
 ↓
Tool
 ↓
Payment
...
```

检测：

```text
same_agent
+
same_merchant
+
same_resource
+
high_frequency
+
repeated_failure
```

触发：

```text
CIRCUIT OPEN
```

立即阻断支付。

---

# 20. Risk Engine

未来可以进一步引入 Risk Score：

```text
Risk Score = f(
  amount,
  merchant,
  frequency,
  agent_history,
  resource,
  time,
  failure_rate,
  behavior_pattern
)
```

例如：

```text
Risk = 0.08
→ Auto Approve

Risk = 0.51
→ Additional Verification

Risk = 0.92
→ Block
```

---

# 21. Reverse Gateway

Seller 侧可以通过反向代理快速接入 AgentPay：

```text
Agent
 ↓
AgentPay Gateway
 ↓
Policy / x402
 ↓
Existing API
```

YAML：

```yaml
routes:
  - path: /v1/data
    price: 0.05
    payTo: "0x..."
    upstream: "http://data-service:8080"
```

价值：

> 让没有修改现有业务代码的 API 服务，也可以快速变成 Paid API。

---

# 22. Seller / Buyer 双侧架构

AgentPay 不应该只服务 Buyer。

## Buyer

```text
AI Agent
 ↓
AgentPay Client
 ↓
Financial Firewall
 ↓
x402
```

## Seller

```text
API
 ↓
AgentPay Middleware
 ↓
x402 Challenge
 ↓
Payment Verification
 ↓
Resource
```

最终：

```text
                AgentPay Network

Buyer                           Seller
  │                               │
  ▼                               ▼
AgentPay Client          AgentPay Middleware
  │                               │
  └──────────── x402 ─────────────┘
                 │
                 ▼
             Facilitator
```

---

# 23. Polyglot SDK

当前项目规划包括：

- TypeScript
- Python
- Java / Spring Boot

长期建议不要把“语言数量”作为主要卖点，而是把：

> **Protocol Compatibility + Developer Experience**

作为重点。

## 第一优先级

### Python

服务：

- LangChain
- CrewAI
- LlamaIndex
- AutoGen 等 Agent 生态

## 第二优先级

### TypeScript

服务：

- Node.js
- Express
- Fastify
- MCP
- Web Agent

## 第三优先级

### Java

主要面向：

- Enterprise
- Spring Boot
- API Provider
- Payment Backend

---

# 24. 推荐 MVP

当前项目功能面较宽，第一阶段应严格收敛。

## MVP 目标

只证明一个问题：

> **“AI Agent 可以自主购买付费 API，但绝对不能突破人类设置的资金权限。”**

---

## MVP 组件

```text
1. Python Buyer SDK
2. x402 V2
3. Financial Firewall
4. Budget Reservation
5. EIP-3009
6. Facilitator Client
7. Seller Middleware
8. SQLite
9. MCP Paid Tool Demo
10. Runaway Loop Demo
```

暂时不要优先做：

```text
Java Enterprise
复杂 Control Plane
多租户 SaaS
复杂 Risk AI
大规模 PostgreSQL
多链
复杂托管钱包
```

---

# 25. 最重要的 Demo

推荐制作一个极强的端到端 Demo。

## Demo：Research Agent

用户：

```text
Research the latest NVIDIA GPU information.
```

Agent：

```text
Search API          $0.02
Benchmark API       $0.10
Data API             $0.30

Total                $0.42
```

AgentPay：

```text
Financial Firewall

✓ Per transaction: $1
✓ Daily budget: $20
✓ Merchant allowed
✓ Frequency normal

PAYMENT APPROVED
```

然后故意：

```text
Agent:
Call expensive-data API
Price: $20
```

Firewall：

```text
┌─────────────────────────────┐
│ PAYMENT BLOCKED             │
│                             │
│ Requested: $20              │
│ Maximum:   $1               │
│                             │
│ Reason:                     │
│ Per-transaction limit       │
│ exceeded                    │
└─────────────────────────────┘
```

这比展示几十个 SDK API 更能体现项目价值。

---

# 26. Runaway Loop Demo

第二个 Demo：

```text
Agent
 ↓
Tool A
 ↓
$0.01
 ↓
Tool B
 ↓
$0.01
 ↓
Tool A
 ↓
$0.01
 ↓
...
```

系统检测：

```text
Payment #1
Payment #2
Payment #3
...
Payment #20
```

触发：

```text
Circuit Breaker

Reason:
Repeated payment loop detected.
```

这是一个非常适合传播的技术 Demo。

---

# 27. MCP Paid Tool Demo

第三个 Demo：

```text
MCP Tool
company.lookup
price = $0.02
```

Agent 调用：

```text
tool.call()
```

自动：

```text
MCP
 ↓
402
 ↓
AgentPay
 ↓
Policy
 ↓
EIP-3009
 ↓
x402
 ↓
Tool Result
```

用户只看到：

> Agent 自己购买了这个工具。

这非常符合 Agent-native UX。

---

# 28. 产品形态

## 28.1 Open Source SDK

免费：

```text
AgentPay Core
AgentPay SDK
x402 middleware
Policy Engine
Local Signer
```

目标：

> 获得开发者采用。

---

## 28.2 AgentPay Cloud

商业化：

```text
Agent Management
Wallet Management
Policy Management
Budget Management
Audit
Analytics
Risk Control
Reconciliation
```

---

## 28.3 Enterprise

```text
SSO
RBAC
KMS / HSM
Private Deployment
Audit
Compliance
Multi-Tenant
Organization Policies
Approval Workflow
```

---

# 29. 商业模式

## Model A：SaaS

```text
Free
$0

Pro
$29/month

Team
$199/month

Enterprise
Custom
```

---

## Model B：Transaction Fee

例如：

```text
0.1% ~ 0.5%
```

但早期不建议过度依赖交易费。

更重要的是：

> 先让 AgentPay 成为开发者默认的 Financial Firewall。

---

# 30. 护城河

AgentPay 的护城河不应该是 x402。

因为：

```text
x402
```

属于开放协议。

真正应该积累：

```text
Policy
+
Identity
+
Risk
+
Transaction History
+
Merchant Reputation
+
Agent Reputation
+
Developer Ecosystem
```

最终形成：

```text
Agent Identity
      ↓
Spending Policy
      ↓
Payment History
      ↓
Risk Score
      ↓
Merchant Reputation
      ↓
Agent Commerce Network
```

---

# 31. 长期产品架构

最终可以形成：

```text
                    AgentPay
                       │
        ┌──────────────┼──────────────┐
        │              │              │
        ▼              ▼              ▼
     Identity        Policy         Wallet
        │              │              │
        └──────────────┼──────────────┘
                       ▼
                   Payments
                       │
             ┌─────────┴─────────┐
             ▼                   ▼
            x402              Facilitator
             │                   │
             └─────────┬─────────┘
                       ▼
                   Settlement
                       │
                       ▼
                Agent Commerce
                       │
       ┌───────────────┼──────────────┐
       ▼               ▼              ▼
    API Tools       MCP Tools      Agents
```

---

# 32. Roadmap

## Phase 0：Protocol Prototype

目标：

> 能支付

完成：

- x402 V2
- EIP-712
- EIP-3009
- Base Sepolia
- Buyer / Seller
- Facilitator

---

## Phase 1：Financial Firewall

目标：

> 能安全支付

完成：

- Per transaction limit
- Daily budget
- Lifetime limit
- Whitelist
- Rate limit
- Circuit breaker
- Atomic budget reservation
- Idempotency

这是 MVP 的核心阶段。

---

## Phase 2：Agent Ecosystem

目标：

> Agent 可以自然使用 Paid Tools

完成：

- Python SDK
- MCP
- LangChain
- CrewAI
- LlamaIndex
- Reverse Gateway

---

## Phase 3：Control Plane

目标：

> 企业能够管理大量 Agent

完成：

- Multi-tenant
- Organization
- RBAC
- Agent management
- Policy management
- Audit
- Dashboard
- PostgreSQL

---

## Phase 4：Agent Wallet

目标：

> 建立 Agent 原生资金账户

完成：

- Agent Wallet
- Spending Account
- KMS
- Vault
- Approval
- Allowance
- Delegation

---

## Phase 5：Agent Commerce Network

目标：

> Agent ↔ Agent 自动交易

完成：

- Merchant Registry
- Agent Registry
- Reputation
- Pricing
- Discovery
- Marketplace
- Agent-to-Agent payment

---

# 33. 技术优先级

建议：

```text
                    High Value
                       ▲
                       │
             Financial Firewall
                       │
                Idempotency
                       │
               Reconciliation
                       │
                 Agent Wallet
                       │
                 MCP Paid Tool
                       │
                    x402
                       │
             Multi-language SDK
                       │
                       ▼
                    Lower Value
```

重点：

> **不要把大量资源投入“支持更多语言”，而应该投入 Policy / Reliability / Security / Agent Wallet。**

---

# 34. 安全模型

AgentPay 涉及真实资金，因此安全模型必须独立于 Agent。

核心原则：

### Principle 1

```text
LLM != Financial Authority
```

### Principle 2

```text
Policy != LLM Tool
```

### Principle 3

```text
Private Key != Agent Runtime
```

### Principle 4

```text
Timeout != Failure
```

### Principle 5

```text
Payment != Fulfillment
```

### Principle 6

```text
Every Payment Must Be Idempotent
```

---

# 35. Threat Model

主要威胁：

| 威胁 | 解决方案 |
|---|---|
| Prompt Injection | 独立 Policy Engine |
| LLM Hallucination | Amount / Merchant Policy |
| Infinite Loop | Circuit Breaker |
| Double Payment | Idempotency |
| RPC Timeout | Reconciliation |
| Malicious Merchant | Whitelist / Reputation |
| Key Leakage | KMS / HSM |
| Budget Race | Atomic Reservation |
| Replay Attack | Nonce / Expiry |
| Excessive Frequency | Rate Limiting |
| Unauthorized Agent | Identity / Capability |

---

# 36. 可观测性

每次支付应该产生完整 Audit Event：

```json
{
  "agent_id": "research-agent",
  "payment_intent_id": "pi_xxx",
  "merchant": "data-api",
  "resource": "/v1/data",
  "amount": "0.20",
  "currency": "USDC",
  "policy_decision": "ALLOW",
  "policy_version": "v3",
  "settlement": "CONFIRMED",
  "fulfillment": "DELIVERED",
  "timestamp": "..."
}
```

这样才能回答：

> 为什么 Agent 花了这笔钱？

以及：

> 谁允许它花这笔钱？

---

# 37. 关键数据库模型

建议至少包含：

```text
tenants
agents
wallets
merchants
policies
policy_versions
budgets
payment_intents
payment_attempts
settlements
fulfillments
idempotency_keys
audit_events
risk_events
circuit_breakers
```

关系：

```text
Tenant
 ├── Agents
 │    └── Wallet
 │
 ├── Policies
 │
 ├── Budgets
 │
 └── Transactions
       ├── PaymentIntent
       ├── Settlement
       └── Fulfillment
```

---

# 38. API 设计方向

## Payment Intent

```http
POST /v1/payment-intents
```

## Policy Evaluation

```http
POST /v1/policies/evaluate
```

## Budget

```http
POST /v1/budgets/reserve
POST /v1/budgets/commit
POST /v1/budgets/release
```

## Reconciliation

```http
POST /v1/reconciliation
```

## Transaction

```http
GET /v1/transactions/:id
```

## Agent

```http
POST /v1/agents
GET /v1/agents/:id
```

---

# 39. 关键 SDK 体验

最终开发者应该尽量只写：

```python
client = AgentPay(
    budget="$20/day",
    max_transaction="$1"
)

result = client.get(
    "https://api.example.com/data"
)
```

而 AgentPay 自动完成：

```text
HTTP
 ↓
402
 ↓
Parse Challenge
 ↓
Policy
 ↓
Budget Reserve
 ↓
Sign
 ↓
Retry
 ↓
Verify
 ↓
Settlement
 ↓
Reconciliation
```

开发者不需要理解所有支付细节。

---

# 40. Developer Experience 原则

AgentPay 应该遵循：

> **Zero-config for simple use cases, explicit control for advanced use cases.**

简单：

```python
AgentPay(...)
```

高级：

```python
AgentPay(
    policy=Policy(
        max_transaction=1,
        daily_budget=20,
        allowed_merchants=[
            "search-api",
            "data-api"
        ]
    )
)
```

---

# 41. 当前项目的主要风险

## 风险 1：范围过大

当前规划同时包含：

```text
TS
Python
Java
MCP
x402
Wallet
Facilitator
Policy
Gateway
Control Plane
PostgreSQL
```

容易导致：

> Everything, but nothing deeply finished.

### 建议

第一阶段只聚焦：

```text
Python
+
MCP
+
x402
+
Financial Firewall
```

---

## 风险 2：用户教育成本

开发者可能会问：

> 为什么不能直接用 x402？

答案必须非常清楚：

```text
x402:
“How do I pay?”

AgentPay:
“Should my Agent be allowed to pay?”
```

---

## 风险 3：安全责任

这是资金基础设施。

因此：

- 必须明确 experimental 状态
- 必须持续做 security review
- 生产环境需要 KMS / HSM
- 必须做好审计
- 必须明确 custody / non-custody 边界
- 不应过早宣传“安全保证”

当前仓库 README 已明确说明项目尚未审计且仅建议用于研究、测试和 devnet evaluation。

---

# 42. 竞争策略

不要定位成：

> Another x402 SDK

而应该定位成：

> **Financial Firewall + Policy Layer for Agent Commerce**

形成：

```text
x402
= Payment Protocol

AgentPay
= Financial Control Plane
```

如果以后 Agent Commerce 发展起来：

```text
Agent
 ↓
MCP
 ↓
AgentPay
 ↓
x402
 ↓
Merchant
```

AgentPay 就可以成为 Agent Economy 的控制层。

---

# 43. 核心 KPI

早期不要看：

```text
GitHub Stars
```

更应该看：

### Developer

```text
Installed SDKs
Active Agents
Active Sellers
```

### Payment

```text
Payment Intents
Successful Payments
Payment Volume
```

### Safety

```text
Blocked Transactions
Policy Violations
Loop Breaks
Duplicate Payment Prevented
```

### Ecosystem

```text
Paid MCP Tools
Paid APIs
Agent-to-Agent Transactions
```

---

# 44. 最终产品愿景

AgentPay 的终局可以抽象成：

```text
                 HUMAN
                   │
                   ▼
             Agent Creation
                   │
                   ▼
             Agent Identity
                   │
                   ▼
          Spending Permission
                   │
                   ▼
          ┌─────────────────┐
          │    AgentPay     │
          │                 │
          │ Financial OS    │
          └────────┬────────┘
                   │
        ┌──────────┼──────────┐
        ▼          ▼          ▼
      Tools      APIs       Agents
        │          │          │
        └──────────┼──────────┘
                   ▼
                  x402
                   │
                   ▼
                 USDC
                   │
                   ▼
              Settlement
                   │
                   ▼
             Agent Economy
```

---

# 45. 一句话总结

AgentPay 不应该只是：

> **“让 AI Agent 会支付。”**

而应该成为：

> **“控制 AI Agent 如何、何时、向谁、以多少钱进行自主经济活动的金融安全基础设施。”**

进一步可以形成：

```text
Agent Identity
+
Financial Policy
+
Budget
+
Wallet
+
Payment
+
Risk
+
Audit
+
Reconciliation
+
Agent Commerce
```

最终目标：

# **Financial OS for AI Agents**

---

# 46. 下一阶段执行计划

## Sprint 1：核心支付闭环

```text
[x] x402 V2
[x] EIP-712 / EIP-3009
[x] Buyer
[x] Seller
[x] Facilitator
[x] Base Sepolia
```

## Sprint 2：Financial Firewall

```text
[ ] Per transaction limit
[ ] Daily budget
[ ] Merchant whitelist
[ ] Rate limit
[ ] Atomic reservation
[ ] Circuit breaker
```

## Sprint 3：可靠性

```text
[ ] Idempotency
[ ] Tri-state machine
[ ] Settlement reconciliation
[ ] Retry strategy
[ ] Recovery
```

## Sprint 4：Agent Integration

```text
[ ] Python SDK
[ ] httpx interceptor
[ ] MCP paid tool
[ ] LangChain integration
[ ] CrewAI integration
```

## Sprint 5：Demo

```text
[ ] Research Agent
[ ] Paid MCP Tool
[ ] Runaway Loop
[ ] Budget Violation
[ ] Transaction Dashboard
```

## Sprint 6：Open Source Release

```text
[ ] Documentation
[ ] Examples
[ ] Test vectors
[ ] Security model
[ ] Threat model
[ ] Devnet quick start
```

---

# 47. 最终建议

如果把 AgentPay 当成一个长期项目，我建议遵循以下战略：

```text
第一阶段
“让 Agent 能支付”
        ↓
第二阶段
“让 Agent 安全地支付”
        ↓
第三阶段
“让 Agent 能管理自己的预算”
        ↓
第四阶段
“让 Agent 可以购买各种工具”
        ↓
第五阶段
“让 Agent 与 Agent 自动交易”
        ↓
最终
“Agent Financial OS”
```

其中最重要的产品飞轮是：

```text
更多 Agent
    ↓
更多交易
    ↓
更多支付行为数据
    ↓
更好的 Risk / Policy
    ↓
更安全的 Agent
    ↓
更多开发者敢于开放支付能力
    ↓
更多 Paid Tools / APIs
    ↓
更多 Agent
```

**因此，AgentPay 最值得投入的核心不是“支付本身”，而是 `Policy + Security + Reliability + Agent Commerce`。**

---
## 结论

当前 AgentPay 已经具备一个有潜力的基础设施方向：仓库当前明确围绕 x402 V2、Financial Firewall、三态交易状态机、幂等/恢复、Gasless 微支付以及 Buyer/Seller SDK 等方向建设；但项目仍处于早期实验阶段，当前 GitHub 页面显示 12 次提交、0 stars、0 forks，并且 README 明确提示尚未经过审计、不适用于生产主网资金。

**如果继续做，最推荐的产品定位是：**

> # AgentPay — The Financial Firewall for AI Agents

**第一性问题不是“AI 如何付款”，而是“AI 在什么条件下才有资格花钱”。**

