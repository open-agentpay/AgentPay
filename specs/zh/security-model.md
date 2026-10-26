# AgentPay 安全与威胁模型规范 (v0)

> **规范版本：** 1.1.0（公开威胁模型 v0）  
> **状态：** 正式生产规范  
> **适用运行时：** Python SDK (`agentpay`)、Go 网关 (`services/gateway`)  

---

## 1. 系统上下文与信任边界

AgentPay 运行在非确定性大语言模型（LLM）与确定性密码学金融网络之间的安全边界上。

```mermaid
graph TD
    subgraph UntrustedZone [非授信区域 (UNTRUSTED ZONE)]
        LLM[LLM Agent 核心 / 提示词]
        ContextMem[上下文记忆与工具输出]
        PublicWeb[外部网页 / 第三方 API]
    end

    subgraph BuyerHost [买方宿主机信任边界 (BUYER HOST TRUST BOUNDARY)]
        Interceptor[AgentPay HTTP 拦截器]
        Firewall[金融防火墙 / 策略引擎]
        DB[(本地 SQLite 策略账本)]
        Signer[LocalSigner 密钥库]
    end

    subgraph SellerDomain [卖方 / 网关域 (SELLER / GATEWAY DOMAIN)]
        Gateway[AgentPay 自托管网关]
        Upstream[内部业务上游 API]
    end

    subgraph SettlementLayer [结算层 (SETTLEMENT LAYER)]
        Facilitator[Facilitator 结算服务]
        Chain[(EVM 区块链 / Base / Arbitrum)]
    end

    LLM -->|提示词输出 / 未校验动作| Interceptor
    ContextMem -.->|潜在提示词注入| Interceptor
    Interceptor -->|1. 检查并预留配额| Firewall
    Firewall -->|原子事务与配额锁定| DB
    Firewall -->|2. 批准的挑战载荷| Signer
    Signer -->|3. EIP-712 / EIP-3009 签名| Interceptor
    Interceptor -->|4. HTTP + PAYMENT-SIGNATURE| Gateway
    Gateway -->|5. 校验签名| Facilitator
    Gateway -->|6. 反向代理分发| Upstream
    Facilitator -->|7. 批量免 Gas 结算| Chain

    style UntrustedZone fill:#ffebee,stroke:#c62828,stroke-width:2px
    style BuyerHost fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px
    style SellerDomain fill:#e3f2fd,stroke:#1565c0,stroke-width:2px
```

### 信任矩阵

| 参与方 / 组件 | 信任级别 | 依据与说明 |
| :--- | :---: | :--- |
| **LLM Agent 与提示词上下文** | ❌ **非授信** | 易受提示词注入（Prompt Injection）、越狱与幻觉影响，绝不能持有或接触明文私钥。 |
| **策略引擎（金融防火墙）** | 🛡️ **授信** | 独立于模型 Token 执行的进程内确定性代码。 |
| **本地 SQLite 策略账本** | 🛡️ **授信** | 运行在排他文件锁（`WAL` + `BEGIN IMMEDIATE`）下的本地文件系统存储。 |
| **LocalSigner / 钱包签名器** | 🛡️ **授信** | 内存隔离的签名器；仅对通过策略预检的合法 EIP-712 载荷进行签名。 |
| **自托管网关（Gateway）** | ⚖️ **半授信** | 负责执行报价与代理请求；其行为受 Facilitator 验签与哈希校验约束。 |
| **外部 API / 第三方上游** | ❌ **非授信** | 可能返回恶意天价 402 报价、伪造响应头或在收款后断开连接。 |
| **Facilitator 结算节点** | ⚖️ **半授信** | 负责验证签名并打包上链；由于授权签名已密码学绑定收款方（`payee`），Facilitator 无法盗取或篡改资金流向。 |

---

## 2. 防御范围内的威胁与缓解机制（买方域）

v0 安全架构明确针对买方侧的 7 类核心威胁向量提供防护：

### T1：金融提示词注入与自我提权
* **防御机制：** LLM 上下文没有任何可修改策略文件、SQLite 表或预算配置的编程接口。所有安全检查均在调用私钥前由进程内确定性 Python 代码完成。

### T2：失控 Agent 死循环耗尽资金
* **防御机制：** 滚动 RPM 频率熔断器（如最高 15 次/分钟）+ 硬性 24 小时滚动预算池。检测到突发流量时立即跳闸进入 `OPEN` 熔断状态。

### T3：恶意上游天价敲诈与价格暗涨 (P1-7)
* **防御机制：** 硬性 `max_per_tx_base_units` 整数比较 + 严格的收款方/资产/网络白名单校验；支持配置单资源价格上限以抵御突发涨价。

### T4：跨链与跨合约重放攻击
* **防御机制：** EIP-712 Domain Separator 在密码学上严格绑定 `chainId`（如 `421614` / `84532`）与 `verifyingContract`（代币合约地址）。

### T5：签名重放与双花攻击
* **防御机制：** 使用 `secrets.token_bytes(32)` 生成 EIP-3009 32 字节密码学安全随机 Nonce；网关强制校验 `(network, asset, payer, nonce)` 联合唯一性。

### T6：延迟提交与套利扣留
* **防御机制：** 授权签名强制绑定严格的有效期窗口：$\text{validBefore} = \text{validAfter} + 300\text{s}$。过期授权在链上合约层面数学失效。

### T7：高并发预算竞态条件
* **防御机制：** 在排他性 SQLite 事务（`BEGIN IMMEDIATE`）内执行评估与锁定，签名生成前先将配额状态置为 `RESERVED`。

---

## 3. 网关安全与入站威胁防御（卖方网关域, P0-5）

自托管网关明确防御以下 5 类卖方侧威胁：

| 威胁类别 | 威胁描述 | 核心防御机制 |
| :--- | :--- | :--- |
| **G1：抢跑抽干资金 / 撤销授权攻击** | 在“乐观交付”模式下，买方在链上结算前收到数据，随后立即转空钱包余额或调用 `cancelAuthorization`。 | 1. 网关在验签阶段查询节点 RPC 核实买方代币余额。<br>2. 高价值路由支持开启 `settle_before_deliver: true`（先结算确认后交付）。<br>3. 低价值微支付可接受极小违约风险以换取低延迟。 |
| **G2：幂等缓存劫持** | 攻击者猜测或嗅探他人的 `Idempotency-Key`，企图不付款直接获取已缓存的付费响应。 | 幂等缓存严格绑定 `(payer_address, idempotency_key)` 或 `(tenant_id, idempotency_key)`；重放请求必须携带同一付款主体的有效凭证。 |
| **G3：SSRF 与内网暴露** | 恶意 HTTP 请求企图通过反向代理进行路径遍历或探测内网私有地址。 | 1. 网关路由严格通过静态配置或受控快照定义上游 URL。<br>2. 剥离逐跳（hop-by-hop）请求头与内部鉴权头。<br>3. 禁用向私有子网的 HTTP 自动重定向（`follow_redirects=False`）。 |
| **G4：402 挑战与 Facilitator DoS 洪泛** | 攻击者发送海量未认证请求以耗尽 Facilitator 配额或网关 CPU。 | 1. 对 402 挑战生成实施入站 IP 速率限制。<br>2. 网关对发往 Facilitator `/v1/verify` 的调用启用熔断保护。 |
| **G5：本地配置文件篡改** | 本地篡改路由价格或收款地址配置。 | 网关 YAML 配置文件以只读权限（`chmod 400`）加载，或在数据库快照模式下通过 SHA-256 指纹强校验。 |

---

## 4. v0 明确非目标（暂不防御范围, P0-4, P1-10）

以下威胁**明确不在 v0 里程碑的防护范围内**：

| 威胁类别 | v0 状态 | 说明与 2027 演进计划 |
| :--- | :---: | :--- |
| **N1：宿主机 Root 权限失陷** | ❌ **不防护** | 若攻击者已获取宿主机 root 权限或可直接读取进程内存，即可从 `LocalSigner` 提取明文私钥。<br>*2027 计划：* 接入硬件安全模块（HSM）与云端 KMS 签名器。 |
| **N2：预算额度内的经济浪费** | ❌ **不防护** | 若 Agent 在允许的预算范围内调用了白名单 API，但上游返回了毫无价值的回答，AgentPay 无法替业务判断语义质量。 |
| **N3：上游数据语义质量 / 幻觉内容** | ❌ **不防护** | 若上游 API 返回 HTTP 200 但内容失真，AgentPay 仅验证付费调用已完成交付，不验证内容真实性。 |
| **N4：Facilitator 完全拒绝服务 (DoS)** | ❌ **不防护** | 若所有 Facilitator 节点离线，交易将无法上链结算；此时买方资金安全（不会被盗），但支付流程暂停。 |
| **N5：非标准代币合约** | ❌ **不防护** | 不支持原生 EIP-3009（`transferWithAuthorization`）的代币无法免 Gas 结算；v0 严格限定标准 USDC 兼容合约。 |
| **N6：恶意进程内工具 (P0-4)** | ❌ **不防护** | 在 v0 嵌入式库模式下，LLM 运行时与 SDK 共享同一 Python 进程；具备任意代码执行（`eval`）或任意文件读写能力的恶意工具可绕过拦截器。<br>*2027 计划：* 演进为以独立操作系统用户运行的进程外本地 Sidecar 守护进程。 |

---

## 5. 核心安全不变量与运行铁律

### 不变量 1：防超支铁律 (P0-1, P1-9)
* 一旦支付意图进入 `SIGNATURE_SENT` 状态，任何网络故障**均不得**自动触发配额释放。配额必须保持 `RESERVED` 锁定状态，直至满足 `now > validBefore + 60s` **且**链上查询证实该授权尚未被花费。

### 不变量 2：单账本不变量 (P0-4)
* **一个私钥严格对应唯一一个本地 SQLite 账本文件。** 严禁使用同一钱包私钥在多个挂载不同 SQLite 文件的 Agent 进程中并发运行，否则会导致预算上限被成倍放大。

### 不变量 3：默认拒绝（Fail-Closed）
* 在数据库锁超时、序列化错误或策略引擎内部发生任何未捕获异常时，**交易判定必须默认返回 REJECTED（拒绝）**。
