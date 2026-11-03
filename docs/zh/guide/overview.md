# 项目概览

::: tip 企业级 AI 安全底座
AgentPay 不仅支持单机部署体验，更提供面向生产环境的高性能 Go 反向代理网关、Vue 3 多租户管控台与开箱即用的 Docker 容器编排套件。
:::

## AgentPay 是什么？

**AgentPay** 是一套专为生产级 AI Agent、大模型应用与 MCP 工具生态打造的**企业级 AI 行为审计与资金防火墙 (AI Agent Gateway & FinOps)**。

当 LLM Agent 通过 Tool Calling、MCP、LangChain、Dify 等方式调用外部付费工具、商业大模型与企业内网服务时，企业技术团队与管理层普遍面临三大恐惧：
1. **账单失控**：Agent 陷入死循环递归调用或遭遇 Prompt 注入，一夜之间刷爆数万元 API 额度导致欠费停机；
2. **财务无法对账**：几十万模型与外部 API 开销，分不清是哪个部门、哪个智能体产生的，无法按部门分摊预算，没有合规增值税发票；
3. **合规审计空白**：等保 2.0 审查时，纯应用层日志（Prompt 日志）容易被模型幻觉篡改，缺乏具备法律效力的外部调用网络黑匣子与人工审批留痕。

AgentPay 部署在 AI Agent 与外部商业资源之间，充当企业的**物理金融防火墙与智能体行为网关**。核心原则很简单：

> **LLM 可以“提议”调用与付款，但永远不能“自我批准”预算。每一笔操作在资金流动与外部发包之前，都必须经过独立于模型的确定性防火墙，执行预算校验、熔断检测或人工审批。**

### 适合谁用？

| 你是… | AgentPay 提供… |
| --- | --- |
| **Agent / AI 应用开发者**（买方） | 即插即用的 Python / TypeScript 客户端：透明处理 `HTTP 402`、本地贴身防火墙拦截、防死循环熔断、开箱即用集成 Dify/LangChain。 |
| **企业 IT 架构师 / CTO** | 生产级高性能 Go 反向代理网关：单笔上限硬顶、死循环 3 秒拉闸、超额自动唤醒**飞书 / 企业微信卡片审批**（人在回路）。 |
| **企业财务总监 (CFO) / FinOps** | 部门/项目/Agent 树状配额水表、人民币（CNY）6 位微单位虚拟账本、对公打款水单核销与增值税专票导出。 |
| **API / MCP 工具提供方**（卖方） | 一键变现网关：通过 YAML 声明即可把现有普通 API 包装成按次计费（如 ¥0.05/次）的商业工具，无需自研计费鉴权系统。 |

---

## 解决的问题

| 风险 / 痛点 | 会发生什么 | AgentPay 的应对 |
| --- | --- | --- |
| **失控死循环 (Runaway Loop)** | Agent 逻辑缺陷或重试机制错误，在几秒内对同一付费 API 反复请求数百次。 | 24 小时滑动窗口预算、单笔上限、以及第 3 秒自动拉闸的**突发频率熔断器（Circuit Breaker）**。 |
| **提示词注入与越权调用** | 恶意用户通过 Prompt 诱导 Agent 购买高价值付费资源或转账。 | 物理隔离的策略引擎、域名/工具/路由**白名单**与单笔限额，脱离模型上下文硬拦截。 |
| **大额操作失控 (需要人管)** | Agent 自主发起一次高达数百元的高算力任务或商业数据采购。 | **人在回路 (HITL)**：触发 `402` 异步挂起，向**飞书 / 企微 / 钉钉**推送审批卡片，手机一键放行。 |
| **Prompt 监控无法拦截物理资金** | 传统工具（如 Langfuse）只能在事后记录 Token 消耗，无法在网络层掐断连接。 | **AI 运维双子星**：原生打通 Langfuse Trace ID，AgentPay 负责网络层物理阻断并回写真实法币账单。 |
| **重复扣款与网络超时** | 把超时误判为失败并盲目重试，导致买方被重复扣费。 | **三态解耦状态机**（意图 / 结算 / 履约）、幂等键，以及“网络超时 ≠ 扣款失败”的对账自愈保障。 |
| **预算并发竞态** | 并发智能体同时看到“余额充足”，并发执行导致账户被透支。 | **两阶段原子预占**（`Hold → Capture / Release`）与数据库行级排他锁。 |
| **多智能体责任混乱** | 复杂多 Agent 协同体系中，不知道哪只 Agent 刷爆了额度。 | 严格的身份单一来源（`tenant → project → agent`）、**不可篡改行为审计黑匣子**与一键停用电闸（Kill Switch）。 |

---

## 一笔支付是如何完成的

AgentPay 支持两条支付路径，网关根据请求携带的凭证**确定性地**选择其一。

### 路径 A — 虚拟钱包（API Key）

```text
Agent ── HTTP + Authorization: Bearer apk_live_… ──▶ Gateway
   1. 校验 Key        → 解析租户、项目与 Agent
   2. 策略检查        → 单笔上限、预算、频率限制、路由白名单
   3. HOLD（冻结）    → 在租户钱包中冻结价格（行级锁）
   4. 转发上游（上游凭证由网关注入）
   5a. 2xx  → CAPTURE（确认扣款）→ 追加账本流水，返回 200
   5b. 4xx  → RELEASE（释放）    → 解冻，不扣费
   5c. 5xx / 超时 → 保持 HELD，标记 needs_review（既不自动扣费，也不自动释放）
```

### 路径 B — 链上 x402（EIP-3009）

```text
Agent ── GET /resource ──▶ Gateway
Agent ◀── 402 + PAYMENT-REQUIRED（价格、收款人、资产、网络、有效期）──
   1. 客户端解析挑战
   2. 本地策略引擎：评估并原子地预留（RESERVE）预算
   3. 本地签名器：生成 EIP-712 / EIP-3009 授权签名
   4. 携带 PAYMENT-SIGNATURE 重放请求
   5. 网关校验签名、Nonce 与价格 → 转发上游
   6. Facilitator 上链结算（Gas 由其代付，买方只需持有 USDC）
   7. 买方提交（或在授权过期且确认未花费后释放）预留额度
```

::: tip 禁止静默降级
钱包路径余额不足时，网关返回 `402 ERR_WALLET_INSUFFICIENT_BALANCE`，**绝不**悄悄改走链上扣费。改用链上支付必须是买方显式发起、并经其签名的新请求。
:::

---

## 架构

```text
┌──────────────────────┐        ┌───────────────────────────────┐        ┌────────────────────┐
│       AI Agent       │        │        AgentPay Gateway       │        │      上游服务      │
│ LangChain / CrewAI / │ ─────▶ │  鉴权 · 策略 · x402 · 代理    │ ─────▶ │  （你现有的        │
│  自研 + SDK 客户端   │        │  钱包 Hold/Capture/Release    │        │   HTTP API）       │
└──────────┬───────────┘        └───────┬───────────────┬───────┘        └────────────────────┘
           │                            │               │
   本地策略引擎 /                PostgreSQL 15+     Facilitator ──▶ EVM 链
   本地签名器（链上路径）      （账本、配置、      （验证与结算，Arbitrum Sepolia）
                                 审计事件）
                                        ▲
                              管理控制台（Vue 3）
                              路由 · 策略 · 钱包 · Agent · 审计
```

| 组件 | 职责 |
| --- | --- |
| **Python SDK**（`agentpay`） | 透明的 `httpx` 拦截器（同步 + 异步）：识别 `402`、执行本地防火墙、签名、重放、提交或释放；内置 LangChain 付费工具封装。 |
| **金融防火墙 / 策略引擎** | 单笔上限、24h 滚动预算、终身限额、收款人 / 域名 / 网络白名单、频率限制、熔断器；买方侧基于 SQLite 的原子预留。 |
| **网关**（`gateway-go`） | Go 反向代理：路由匹配、402 挑战生成、签名校验、请求头清洗、上游凭证隔离、幂等、故障恢复、Agent 级策略与 Kill Switch；v0.1.0 基线同时提供 Python 参考实现。 |
| **虚拟钱包与账本** | 每租户一个钱包；只追加账本；Hold / Capture / Release；对账。 |
| **Facilitator** | 校验授权并提交由其代付 Gas 的 `transferWithAuthorization` 交易；本地运行时提供 Mock Facilitator。 |
| **管理控制台** | 路由与上游鉴权、配置草稿 → 校验 → 发布 → 回滚、钱包、Agent 与审计时间线。 |
| **PostgreSQL** | 租户、项目、路由、策略、账本、支付与审计事件的权威数据源。 |

---

## 核心概念

### 租户与身份

`Tenant → Project → Agent → API Key`。每个项目归属唯一租户，每个 API Key 归属唯一 Agent，且 **`agent_id` 始终由凭证解析得出**——客户端传入的 `X-Agent-ID` 等请求头会被剥离，绝不信任。可选的自报元数据（`X-Session-ID`、`X-Task-ID`、`X-Tool-Name`）仅用于审计时间线聚合，不参与鉴权、计费与策略判定。

### 三态模型

单一的"支付成功 / 失败"标记无法描述真实情况。AgentPay 维护三条相互正交的状态线：

| 对象 | 回答的问题 | 示例状态 |
| --- | --- | --- |
| **PaymentIntent（支付意图）** | 这次请求处于整体生命周期的哪一步？ | `created`、`challenged`、`authorized`、`completed`、`rejected`、`expired`、`failed`、`needs_review` |
| **Settlement（结算）** | 钱是否真的动了？ | `not_started`、`held`、`submitted`、`pending`、`confirmed`、`released`、`failed`、`unknown` |
| **Fulfillment（履约）** | 上游是否真的交付了？ | `not_started`、`in_progress`、`succeeded`、`failed`、`unknown` |

`unknown` 是一等状态：结果不明确时，既不视为成功，也不视为失败，只能通过查询确认，严禁盲目重试。详见[状态机规范](/zh/specs/state-machine)。

### 幂等

客户端携带 `Idempotency-Key`。相同 Key 且请求指纹一致时，直接回放已存结果（`X-Cache: HIT`），**零额外扣费**；相同 Key 但载荷不同则返回 `409 ERR_IDEMPOTENCY_CONFLICT`。上游 5xx 后使用相同 Key 重试，会复用原冻结，不会再次扣款。

### 版本化配置

路由与策略以草稿形式编辑、校验，再发布为**不可变、带哈希指纹的快照**。网关零停机原子切换到新快照；每个请求都会记录其命中的配置版本；回滚只是一次指针切换。控制面宕机时，网关继续使用最近一次可用快照提供服务，对无法校验的请求一律拒绝（fail-closed）。

---

## 安全不变量

以下规则不是可调参数，而是在代码、数据库与测试中强制执行的设计约束。

1. **只用整数表示金额。** 所有金额均为最小单位整数（USDC 为 6 位小数），任何地方禁止浮点数，数据库列带整数 / 非负 `CHECK` 约束。
2. **默认拒绝（Fail-Closed）。** 锁超时、解析错误、验签失败或任何未捕获异常 ⇒ 拒绝。
3. **批准后才签名。** 只有策略引擎已批准的预留才会调用签名器。
4. **已发出的签名不会因超时而"视为未花费"。** 只有授权过期**且**链上确认未花费后，才会释放预算。
5. **未知状态绝不自动处置。** 履约未知时钱包资金保持冻结，并转入 `needs_review`。
6. **账本只追加。** 账本流水与审计事件永不更新、删除；更正通过新增冲正 / 调账流水完成。
7. **模型没有资金权限。** 策略引擎不是 LLM 工具，LLM 无法读取或修改它。

---

## 部署拓扑

| | 本地嵌入式沙箱 | 云原生分布式 |
| --- | --- | --- |
| **用途** | 开发、CI、演示（`make demo`） | 真实测试网集成 |
| **传输** | 内存 ASGI 传输（不占用端口） | 买卖双方独立域之间的 HTTPS |
| **链** | 本地 Mock EVM RPC | Arbitrum Sepolia（`eip155:421614`） |
| **密钥** | 测试密钥 | 买方本地签名器 / KMS，绝不上传 |
| **Gas** | 模拟 | 由 Facilitator 代付，买方仅持有 USDC |

详见[部署指南](/zh/guide/deployment)。

---

## 范围与已知限制

AgentPay 明确声明**不做**什么：

- **v1 不支持提现。** 虚拟钱包是闭环消费余额，不支持提现与用户间转账。
- **不做语义质量判断。** AgentPay 验证的是"付费调用已交付"，而非"答案是否有价值"。在预算内向白名单 API 花了钱却得到无用结果，不在防护范围内。
- **宿主机失陷不在范围内。** 攻击者若取得运行本地签名器的机器 root 权限，可读取私钥；HSM / KMS 签名器已在规划中。
- **进程内工具不做沙箱隔离。** 嵌入模式下 SDK 与 Agent 共享进程，具备任意代码执行能力的工具可能绕过拦截器；独立进程 Sidecar 已在规划中。
- **一个密钥对应一个本地账本。** 同一私钥在多个进程中使用各自独立的账本，会使预算成倍放大。
- **单一资产，仅限测试网。** v1 仅支持一种资产（兼容 EIP-3009 的 USDC），且运行于测试网。

完整的防护范围请阅读[威胁模型](/zh/specs/)。

---

## 当前状态与路线图

**当前已具备（v0.1.x / v0.2.x）**

- Python SDK：`httpx` 拦截器、本地防火墙（SQLite 原子预留、熔断器）、EIP-712 / EIP-3009 签名器、LangChain 付费工具封装
- TypeScript / Node.js SDK（`open-agentpay`）：原生 `fetch` 拦截器、`better-sqlite3` 本地防火墙、基于 `viem` 的 EIP-3009 签名器，以及 LangChain.js、Vercel AI SDK 与 MCP 集成
- Go 网关：反向代理、x402 处理、钱包路径、幂等与故障恢复、Agent 级策略、Kill Switch、审计时间线
- 多租户控制面：租户、项目、RBAC、Agent、API Key、钱包与管理员发放、路由与策略
- 管理控制台：路由与上游鉴权、配置发布与回滚、钱包、Agent、支付审计
- PostgreSQL 15+ 数据模型，Python / Go / TypeScript 跨语言一致性测试向量
- Arbitrum Sepolia 测试网结算，以及一条命令复现的**失控循环 Demo**

**进行中 / 主线路线图**

完整技术架构演进与任务矩阵详见 [**核心任务全景规划与路线图**](/specs/zh/roadmap.md)：

- **企业协同 HITL 与法币充值闭环（P0 主线）**：飞书 / 企微 / 钉钉 402 审批卡片直连、多层树状配额管控、微信/支付宝/对公汇款合规充值
- **原生 MCP 支持与 Langfuse 双子星联动（P0 主线）**：Model Context Protocol 官方支付中间件、Langfuse Trace 联合追踪面板
- **动态 Gas 加价补偿（Mode 2 / P1 演进）**：预言机动态估价并入 USDC Challenge，Facilitator 零亏损垫付
- **Java / Spring Boot Starter 与卖方中间件（P1 演进）**：`@AgentPayGuard` 注解式 SDK 与 Express/Fastify/NestJS 卖方货币化包
- **KMS / HSM 硬件签名与进程外 Sidecar 守护（P2 架构）**：签名私钥脱离明文内存、独立低权限进程防御代码注入逃逸
- **高频批量聚合与状态通道结算（Mode 3 / P2 架构）**：逐 Token 微额流式计费，Gas 摊薄 95%+，多节点分布式网关集群
- **多链扩展与开放协议标准化（P3 远景）**：扩展至 Solana 与 ERC-4337 抽象账户，正式推进 `x402-v2` 官方标准规范草案

---

## 接下来读什么

| 我想… | 阅读 |
| --- | --- |
| 五分钟跑起来 | [5 分钟快速开始](/zh/guide/quickstart) |
| 接入 Python Agent | [Python SDK](/zh/guide/python-sdk) |
| 接入 TypeScript / Node.js Agent | [TypeScript SDK](/zh/guide/typescript-sdk) |
| 把 API 变成付费 API | [网关配置](/zh/guide/gateway) |
| 设置花费限额 | [策略配置](/zh/guide/policy) |
| 管理租户、密钥与路由 | [管理控制台](/zh/guide/console) |
| 了解协议细节 | [API 规范](/zh/guide/api-specs) |
| 理解各项保证 | [状态机规范](/zh/specs/state-machine) |
| 部署 | [部署指南](/zh/guide/deployment) |
| 参与贡献 | [Good First Issues](/zh/contributing/good-first-issues) |