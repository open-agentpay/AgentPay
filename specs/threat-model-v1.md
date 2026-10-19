# AgentPay 全面威胁模型规范 (Threat Model v1.0)

> **规范版本**：1.0.0  
> **发布日期**：2026-10-18  
> **适用范围**：Go Gateway 数据面、PostgreSQL 存储层、虚拟钱包与账本、控制面与买方 SDK  
> **任务追踪**：研发任务清单 T1.6 (P0)

---

## 1. 架构上下文与信任边界 (System Context & Trust Boundaries)

AgentPay 位于非确定性的大语言模型 (LLM) 智能体、确定性企业多租户控制网关、底层高并发虚拟钱包账本以及外部不可信去中心化网络/第三方 API 之间。

```mermaid
graph TD
    subgraph Boundary0 [边界 0：非受信外部环境 UNTRUSTED]
        Agent[LLM Agent / Prompt 执行环境]
        ClientSigner[外部 Signer / 客户端私钥]
        Attacker[恶意第三方攻击者 / 流量 Flood]
        PublicWeb[第三方非受信上游服务]
    end

    subgraph Boundary1 [边界 1：数据面反向代理接入 GATEWAY INGESTION]
        Gateway[Go Gateway 数据面反向代理]
        RateLimiter[IP / Route 速率限制器]
        RCUCache[RCU 零停机配置内存镜像]
        X402Engine[x402 协议编解码与验签]
    end

    subgraph Boundary2 [边界 2：核心平台与强事务存储 CORE & STORAGE]
        WalletEngine[虚拟钱包与账本引擎]
        PG[(PostgreSQL 15+ 强事务存储)]
        AuditLogger[只追加不可变审计流]
    end

    subgraph Boundary3 [边界 3：控制面与运维特权 CONTROL PLANE]
        Console[Vue3 控制台管理界面]
        AdminAPI[控制面管理 API]
        ConfigPublisher[配置发布与快照管理器]
    end

    subgraph Boundary4 [边界 4：外部清算与结算清算层 SETTLEMENT INFRA]
        RPC[链上 RPC 节点 (Base / Arbitrum)]
        Facilitator[Gasless 批量中继 Facilitator]
        FiatGateway[法币聚合支付网关 (Stripe)]
    end

    Agent -->|HTTP + API Key / Signature| Gateway
    ClientSigner -->|402 签名响应| Gateway
    Attacker -.->|DoS / 伪造充值 / 重放| Gateway
    Gateway -->|反向代理履约| PublicWeb
    Gateway <-->|强事务 Hold/Capture/Release| WalletEngine
    WalletEngine <-->|行级排他锁 / 流水追加| PG
    Gateway <-->|幂等持久化| PG
    ConfigPublisher -->|快照发布/RCU 广播| Gateway
    AdminAPI <-->|RBAC / 审计追踪| PG
    Console -->|Admin Session| AdminAPI
    Gateway -->|链上验签 / 广播| RPC
    Gateway -->|批量代付提交| Facilitator
    FiatGateway -.->|Webhook 充值到账回调| AdminAPI

    style Boundary0 fill:#ffebee,stroke:#c62828,stroke-width:2px
    style Boundary1 fill:#e3f2fd,stroke:#1565c0,stroke-width:2px
    style Boundary2 fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px
    style Boundary3 fill:#fff3e0,stroke:#e65100,stroke-width:2px
    style Boundary4 fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
```

### 1.1 信任等级矩阵 (Trust Level Matrix)

| 组件 / 参与方 | 信任等级 | 安全定位与核心防御假设 |
| :--- | :---: | :--- |
| **LLM Agent & Prompt 上下文** | ❌ **完全不可信** | 易受提示词注入 (Prompt Injection) 和幻觉劫持。绝对禁止持有平台管理密钥或跨租户访问。 |
| **客户端 Signer 私钥** | ⚖️ **半受信** | 客户端独立自持。网关仅校验签名数学有效性与 Nonce，不得接触或代管私钥明文。 |
| **Go Gateway 数据面** | 🛡️ **高受信** | 无状态微服务，只读读取 RCU 内存配置，通过安全连接访问数据库，强制执行 fail-closed。 |
| **虚拟钱包与账本引擎** | 🛡️ **极高受信 (核心资金边界)** | 严格单向流水追加，执行 `SELECT ... FOR UPDATE` 行级悲观锁，禁止脏读与负余额。 |
| **PostgreSQL 数据库** | 🛡️ **极高受信** | 数据落盘与 ACID 核心，采用行级安全隔离与唯一约束防重放。 |
| **第三方上游服务** | ❌ **完全不可信** | 可能返回恶意 5xx、超时悬挂、内网 SSRF 诱导、篡改报价或丢弃连接。 |
| **链上 RPC 与 Facilitator** | ⚖️ **半受信** | 节点可能出现分叉回滚（Reorg）、交易拥堵挂起或恶意抢跑（MEV）。 |
| **控制台管理员 (Operator)** | ⚖️ **受限信任** | 必须受最小特权 RBAC 约束，所有人工调账与配置变更必须全字段记录不可抵赖审计事件。 |

---

## 2. 虚拟钱包与账本专属威胁分析 (Virtual Wallet Threat Vectors)

由于平台引入了内部虚拟钱包作为核心支付轨，资金风险从链上转移至平台内部账户系统。以下是核心威胁、攻击场景与防御规范：

### 2.1 [W-T1] 并发双花与高并发争抢透支 (Concurrent Double-Spending)
- **攻击场景**：恶意租户同时发起 50 个高并发并发请求，每个请求扣减 10 USDC，但钱包可用余额仅有 10 USDC。若采用非原子或宽松无锁的 `SELECT available_cache`，所有请求可能均通过余额校验，导致 500 USDC 实际消费。
- **影响严重度**：Critical (P0)
- **核心防御措施**：
  1. **行级排他悲观锁**：在冻结可用额度（`Hold`）事务中，必须对 `ap_wallet` 目标行执行 `SELECT ... FOR UPDATE`。
  2. **行锁释放保证**：Hold 事务必须是微秒级轻量事务（查询余额 -> 扣减可用额度并增加冻结额度 -> 插入 `ap_hold` -> 提交释放锁），严禁在持有锁期间执行耗时的上游 HTTP 履约调用。
  3. **数据库级守底约束**：`available_cache BIGINT NOT NULL CHECK (available_cache >= 0 OR status = 'frozen_arrears')`。任何导致非欠费钱包余额小于 0 的 SQL 将直接触发 DB Check Constraint 失败回滚。

### 2.2 [W-T2] 负余额透支与超额释放 (Over-Release / Negative Balance Exploit)
- **攻击场景**：攻击者利用网络重试，向网关并发发送针对同一笔 Hold 的 Release 和 Capture 请求，试图“既捕获扣款又释放额度”产生双倍返还。
- **影响严重度**：Critical (P0)
- **核心防御措施**：
  1. **Hold 状态机强约束**：`ap_hold.status` 仅允许合法状态迁移（`held -> captured` 或 `held -> released`）。
  2. **状态更新 CAS 保护**：
     ```sql
     UPDATE ap_hold SET status = 'released' WHERE hold_id = $1 AND status = 'held';
     ```
     若影响行数为 0，说明已被捕获或已释放，立即拒绝并回滚，严禁二次调整可用余额。
  3. **冻结额度下限约束**：`held_cache BIGINT NOT NULL CHECK (held_cache >= 0)`。

### 2.3 [W-T3] 越权人工调账与内部人员提权 (Unauthorized Adjustment / Insider Threat)
- **攻击场景**：恶意管理员或被利用的控制台凭据调用调账接口，随意为自己或关联租户增加 1,000,000 USDC 余额。
- **影响严重度**：High (P0)
- **核心防御措施**：
  1. **多重权限 RBAC 鉴权**：仅具备 `owner` 或 `finance_admin` 角色成员允许调用调账接口。
  2. **调账不可变记账约束**：任何金额调整必须强制写入 `ap_ledger_entry`（`type = 'adjustment'`），且字段 `operator_id` 与 `reason`（不少于 10 字符）必须非空。
  3. **双重审计事件广播**：调账操作同步触发 `admin.adjustment.executed` 审计事件，推送到企业不可篡改审计系统及告警频道。

### 2.4 [W-T4] 充值重放、Webhook 伪造与假充值 (Deposit Replay & Fake Top-Up)
- **攻击场景**：攻击者伪造第三方法币支付或链上转账交易哈希，重复调用充值回调接口，试图刷取平台虚拟额度。
- **影响严重度**：Critical (P0)
- **核心防御措施**：
  1. **外部流水号唯一性约束**：`ap_topup_order` 的 `external_ref` 具备全局唯一性约束（`UNIQUE (external_ref)`）。
  2. **Webhook 强签名校验**：法币充值回调强制验证基于 HMAC-SHA256 的时间戳签名，时间漂移不得超过 300 秒。
  3. **单向状态推进**：充值单终态（`confirmed`, `failed`, `expired`）不可变，禁止已成功订单重复入账。

### 2.5 [W-T5] 链上充值区块重组回滚 (Blockchain Reorg Rollback on Deposits)
- **攻击场景**：用户向平台充值地址发起 100 USDC 链上转账，网关仅等待 1 个区块确认便为其入账并允许消费上游 API；随后由于 L1/L2 发生区块重组（Reorg），该笔交易在链上被丢弃或双花，导致平台承受净资金损失。
- **影响严重度**：High (P1)
- **核心防御措施**：
  1. **多重安全确认数门槛**：
     - EVM L2 (Arbitrum / Base)：至少 12 个 Finalized / Safe 区块确认数。
     - Ethereum L1：至少 32 个区块（2 个 Epoch Finality）。
  2. **链上深度监听与重组巡检 Worker**：定期校验已入账 `tx_hash` 是否仍处于主链有效区块中；若发生重组导致交易消失，立即对对应钱包执行 `status = 'frozen_arrears'` 并创建会计冲正记录。

### 2.6 [W-T6] 履约悬挂引发的冻结泄漏与额度 DoS (Hold Leakage / Starvation DoS)
- **攻击场景**：客户端持续发起请求，网关完成 Hold 冻结后上游服务发生死锁或连接悬挂，请求既未成功也未失败。海量资金卡在 `held` 状态，耗尽租户可用额度。
- **影响严重度**：Medium (P1)
- **核心防御措施**：
  1. **显式 Hold 超时 TTL (W3)**：每个 `ap_hold` 创建时必须指定 `expires_at = NOW() + INTERVAL '30 seconds'`。
  2. **后台超时自动释放 Sweeper**：定时调度器轮询 `WHERE status = 'held' AND expires_at < NOW()`，安全释放超时未捕获的冻结额度，将资金归还 `available_cache` 并记录审计事件。
  3. **未知状态保护原则 (W4)**：若上游返回 5xx 或网络超时导致结果未知，标记 `needs_review`，在人工确认前保持冻结或由保全流程裁决，严禁静默泄露。

### 2.7 [W-T7] 余额不足自动降级绕过策略攻击 (Auto-Downgrade Bypass Attack)
- **攻击场景**：攻击者在虚拟钱包余额不足时，期望系统自动降级到链上支付路径，试图通过未配置链上配额检查的漏洞绕过策略拦截。
- **影响严重度**：High (P0)
- **核心防御措施 (W5 原则)**：
  - **严禁自动隐式降级**：若请求路由要求钱包支付但余额不足，网关必须立刻以 `402 INSUFFICIENT_WALLET_BALANCE` 拒绝请求，**绝对禁止静默切换至链上扣款**。
  - 买方若需改走链上，必须由买方应用在感知到 402 错误后显式发起新请求，并签署合规的 EIP-712/EIP-3009 链上授权载荷。

---

## 3. 网关与数据面通用威胁分析 (Gateway STRIDE Threat Model)

针对 Go Gateway 数据面与请求链路，应用标准 STRIDE 模型展开威胁与缓解设计：

| STRIDE 威胁类别 | 具体威胁编号与场景 | 影响组件 | 核心缓解方案 (Mitigation) |
| :--- | :--- | :--- | :--- |
| **Spoofing (身份伪造)** | **G-S1: 伪造买方签名 / Nonce 重放**<br>攻击者重放已使用过的 EIP-3009 授权签名，试图免费调用服务。 | Go Gateway / x402 编解码器 | 数据库唯一约束 `uq_nonce_replay_guard(network_id, asset_address, payer_address, nonce)`，在内存与 DB 双重校验 Nonce 唯一性。 |
| **Spoofing (身份伪造)** | **G-S2: API Key 伪造与跨租户越权**<br>攻击者伪造 `Authorization: Bearer` 试图冒充其他租户的项目。 | API Key 鉴权中间件 | 存储仅存 `key_hash = SHA256(raw_key)`，使用常数时间比较算法（Constant-Time Compare）防止时序侧信道攻击；API Key 绑定全局租户 ID。 |
| **Tampering (数据篡改)** | **G-T1: 配置发布中间人篡改**<br>攻击者在网络或控制面篡改路由上游地址或价格，将资金重定向到攻击者收款地址。 | 配置发布订阅器 | 强制配置快照采用不可变指纹 `snapshotHash = sha256(...)`，发布时由系统管理员私钥或签名校验后才生效；Gateway 本地严格比对 Hash。 |
| **Tampering (数据篡改)** | **G-T2: 402 挑战价格篡改 (Price Tampering)**<br>攻击者拦截 402 挑战，将其中的 `amount` 修改为 0.0001 USDC 后签名提交。 | 支付意图引擎 | Gateway 收到买方 Authorization 时，**重新从本地路由配置严格计算目标金额**，比对 `auth.value == route.price`；不符立即报错 `PRICE_MISMATCH` 并拒绝。 |
| **Repudiation (抵赖与审计擦除)** | **G-R1: 恶意管理员擦除扣款记录**<br>平台管理员或攻击者侵入数据库，试图修改或删除消费记录。 | 审计与账本存储 | `ap_ledger_entry` 与 `ap_audit_event` 表配置数据库级仅追加约束（禁止 UPDATE/DELETE，通过 DB Trigger 或只读权限限制）；定期归档至 WORM (Write Once Read Many) 存储。 |
| **Information Disclosure (信息泄露)** | **G-I1: 错误日志与堆栈泄漏敏感凭据**<br>上游服务崩溃返回包含 DB 连接串或 API Key 的错误堆栈，网关透传给买方。 | 网关错误转换中间件 | 严格脱敏上游响应；截断错误日志至 1024 字符；API Key 仅记录前缀 `agp_live_****` 与 Hash，绝不明文输出到日志或响应头。 |
| **Information Disclosure (信息泄露)** | **G-I2: SSRF 内网探针攻击**<br>攻击者将上游配置为 `http://169.254.169.254/` 或 `http://10.0.0.1/admin` 试图探测内网云元数据。 | 反向代理传输层 (Transport) | Gateway 禁止反向代理重定向（`CheckRedirect` 禁用）；解析上游 DNS 时屏蔽私有保留网段（RFC 1918、Link-Local、Loopback）。 |
| **Denial of Service (拒绝服务)** | **G-D1: 402 挑战泛洪与内存耗尽**<br>攻击者大量发送无凭据请求，导致网关频繁生成 402 挑战并占用内存。 | 网关速率限制器 | 在 402 签发前置入基于 IP 和路由的 Sliding Window Token Bucket 限流；对未授权恶意探测 IP 触发动态封禁黑名单。 |
| **Elevation of Privilege (提权越权)** | **G-E1: 多租户 IDOR / BOLA 跨租户数据访问**<br>租户 A 传入租户 B 的 `intent_id` 或 `wallet_id` 试图窃取状态或执行操作。 | 业务逻辑与查询层 | 所有业务查询强制注入 `WHERE tenant_id = $session_tenant_id`，禁止仅凭全局 ID 跨租户查询；数据模型外键均级联至租户根。 |

---

## 4. 核心安全不变量与代码守则 (Security Invariants)

平台代码必须严格执行以下**不可妥协的 Phase 0 / Phase 1 安全红线**：

1. **红线 1：整数最小单位与禁用浮点数**
   - 所有金额（代币、资金池、配额、冻结额度）在代码与数据库中必须且仅能以 64 位整数（`int64` / `BIGINT`）表示。
   - 严禁在任何逻辑中转换为 `float32` / `float64` 进行金额运算。
2. **红线 2：Fail-Closed 默认拒绝原则**
   - 任何数据库超时、网络中断、验签抛错、未知错误发生时，交易与请求必须判定为 `REJECTED` 或 `UNKNOWN`，严禁假定“未报错即成功”。
3. **红线 3：未知状态防盲目重试 (Anti-Blind Retry)**
   - 针对 `unknown` 状态的链上结算或履约，系统严禁发起重复扣款或重新提交链上交易。仅允许被动查询和探针恢复。
4. **红线 4：私钥隔离与不落地原则**
   - Go Gateway 与云端控制面**物理不接触买方私钥**。私钥仅存在于客户端独立的 `LocalSigner` 或企业 KMS 模块中。
5. **红线 5：配置变更一键原子生效与回滚**
   - 任何破坏性配置导致错误率飙升时，控制面可在 1 秒内执行原子指针回滚（RCU Pointer Swap），Gateway 内存无停机即时生效。

---

## 5. 威胁缓解措施验证与测试矩阵 (Verification Matrix)

每个识别出的威胁必须拥有对应的自动化测试用例，覆盖于 `python/tests` 与后续 `go/tests`：

| 威胁代号 | 验证类型 | 测试用例位置 | 验证断言与方法 |
| :--- | :---: | :--- | :--- |
| **W-T1** (并发双花) | 并发单元测试 | `tests/unit/test_wallet_concurrency.py` | 模拟 50 线程抢占 1 份余额，断言仅有 1 笔成功，其余全部报错返回 `INSUFFICIENT_WALLET_BALANCE`，最终余额非负。 |
| **W-T2** (负余额) | 状态机测试 | `tests/unit/test_state_machine_matrix.py` | 验证 `Hold` 在 `captured` 后调用 `release` 抛出非法状态迁移异常。 |
| **W-T4** (充值重放) | 向量测试 | `tests/unit/test_protocol_conformance_vectors.py` | 提交包含相同 `external_ref` 的二次充值，断言返回 `DUPLICATE_IDEMPOTENCY_KEY`。 |
| **W-T7** (不自动降级) | 协议契约测试 | `tests/unit/test_protocol_conformance_vectors.py` | 验证钱包余额不足时直接返回 402 错误，不发起链上挑战。 |
| **G-S1** (Nonce 重放) | 签名防线测试 | `tests/unit/test_signer.py` | 相同 Nonce 提交两次，断言二次调用触发签名或幂等冲突拒绝。 |
| **G-T1** (配置篡改) | 发布测试 | `tests/unit/test_config_publishing.py` | 验证篡改任一路由字段后，快照 SHA-256 指纹校验失败并拒绝加载。 |
| **G-I1** (凭据脱敏) | 审计安全测试 | `tests/unit/test_audit_events.py` | 遍历全量测试向量，断言载荷绝对不存在 `private_key`、`raw_api_key` 等敏感字段。 |

---

## 6. v1.0 明确非目标与残留风险说明 (Explicit Non-Goals & Residual Risks)

| 残留风险 / 非目标 | 当前处理边界 | 未来演进路线 (v2.0 / 2027) |
| :--- | :--- | :--- |
| **N1: 宿主机 Root 权限沦陷** | 若攻击者完全控制运行 SDK 的物理机，可读取内存中的明文私钥。平台不为宿主机完全失陷负责。 | 支持 TPM 2.0 / Apple Secure Enclave 硬件加密芯片；支持 AWS KMS / HashiCorp Vault 远程签名器。 |
| **N2: 恶意上游语义垃圾交付** | 上游 API 收到款后返回 HTTP 200，但内容为毫无价值的乱码或虚假回答。网关仅证明服务交付，不裁决语义质量。 | 引入上游评分质押体系（Staking & Slashing）与买方事后争议仲裁 DAO。 |
| **N3: 闪电网络与非 EVM 资产支持** | v1 仅聚焦 EVM 体系（USDC EIP-3009）与平台虚拟钱包，暂不支持比特币闪电网络或 Solana。 | 在 v2 中扩展 Solana SPL Token Transfer 及闪电网络通道网关。 |
