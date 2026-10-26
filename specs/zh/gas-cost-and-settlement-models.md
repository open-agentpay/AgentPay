# Gas 经济学与结算代付模型规范

## 1. 背景与问题陈述

在面向自主 AI Agent 的 Web3 微支付架构中，交易 Gas 费（矿工/排序器费用）是一个关键的工程摩擦点：

1. **AI Agent 使用摩擦：** 如果要求 AI Agent 钱包同时持有 ERC-20 支付代币（如 USDC）和原生 Gas 代币（ETH / L2 ETH），会引入多资产余额管理复杂度、Gas 价格波动风险，以及因 Gas 耗尽导致的高失败率。
2. **EIP-3009 免 Gas 特性：** EIP-3009（`transferWithAuthorization`）原生支持元交易（Meta-Transactions）。代币持有者（`from`）仅需在链下生成密码学签名（EIP-712），由负责将交易提交上链的实体（`msg.sender`）支付 EVM Gas 费。
3. **不同业务场景的经济模型差异：** 取决于链层级（L1 vs L2）、API 价格弹性、调用频率（流式逐 Token vs 离散调用）以及网络拥堵程度，单一固定的 Gas 承担方式无法满足所有企业与开发者的需求。

为实现生产级灵活性，AgentPay 规范定义了**三种受支持的 Gas 代付及结算模型**。

---

## 2. Gas 模型分类全景

```
┌────────────────────────────────────────────────────────────────────────┐
│                        AgentPay Gas 结算模型                           │
├──────────────────────────┬─────────────────────────┬───────────────────┤
│ 模式 1：卖方补贴免 Gas   │ 模式 2：动态 Gas 附加费 │ 模式 3：批量聚合  │
│         (2026 基线)      │         (2027 Q1)       │   与通道 (2027 Q2)│
├──────────────────────────┼─────────────────────────┼───────────────────┤
│ • 买方：0 Gas（仅需 USDC）│ • 买方：以 USDC 支付    │ • 买方：高频签署  │
│ • Facilitator/卖方承担   │   基础价 + Gas 附加费   │   链下状态授权    │
│ • Agent 零接入摩擦       │ • Facilitator 收支平衡  │ • 批量聚合上链    │
│ • 适合 L2 微支付         │ • 适合 L1 或低毛利 API  │ • 适合亚美分/流式 │
└──────────────────────────┴─────────────────────────┴───────────────────┘
```

---

## 3. 三种 Gas 模型详细规范

### 3.1 模式 1：卖方补贴型免 Gas 结算（已实现 / 2026 基线）

* **架构角色分工：**
  * **买方（AI Agent）：** 100% 免 Gas，仅需持有 USDC。
  * **卖方网关 / Facilitator：** 维护一个持有原生代币（如 Arbitrum Sepolia ETH / Base ETH）的运营钱包，用于代付链上执行 Gas。
* **运行机制：**
  1. 网关签发标准 `x402` 挑战，指定精确的基础价格（如 `1000` = 0.001 USDC）。
  2. 买方针对 `1000` 最小单位签署 EIP-712 `TransferWithAuthorization` 授权。
  3. Facilitator 以 `msg.sender = Facilitator` 提交 `transferWithAuthorization` 交易并支付 L2 Gas（在 Arbitrum/Base 上约为 $0.001 - $0.005）。
  4. 0.001 USDC 全额从买方划转至收款商户地址。
* **经济学依据：**
  * 在 Layer 2 Rollup 网络上，Gas 成本相比 SaaS API 价值几乎可以忽略，卖方可将 Gas 视为基础设施带宽成本，或直接含入 API 基础定价中。
* **实现状态：** 已在 AgentPay v0.1.0 中完整支持（`agentpay-gateway`、`EvmRpcClient`、Arbitrum Sepolia Facilitator）。

#### 3.1.1 Facilitator 链上广播执行规范：EIP-1559 动态定价与防波动保护

在 L2 Rollup（如 Arbitrum Nitro、Base）及以太坊主网的区块结算过程中，底层网络的 `baseFee` 会随区块交易密度动态波动。为确保 Facilitator 结算网关在异步广播 `transferWithAuthorization` 交易时，**绝不因亚秒级的区块 BaseFee 瞬时上浮而被节点拒单**（例如报错 `RPC error -32000: max fee per gas less than block base fee`），AgentPay Facilitator 链上结算引擎标准化了以下费用管理机制：

##### 1. 强制采用 EIP-1559 (Type 2) 标准交易
* 链上广播默认采用 **EIP-1559（Type 2）交易格式**，显式指定 `maxFeePerGas` 与 `maxPriorityFeePerGas`，摒弃容易因瞬时费率上浮而被拒单的固定费率 Legacy Type 0 格式。

##### 2. 1.5 倍 BaseFee 安全缓冲公式
在组装链上结算交易前，Facilitator 通过 JSON-RPC 查询最新区块头的 `baseFee`、矿工小费 `priorityFee` 及全局 `gasPrice`，按以下公式计算动态费用上限：

$$\text{maxFeePerGas} = \max\left(\lfloor \text{baseFee} \times 1.5 \rfloor + \text{priorityFee},\; \lfloor \text{gasPrice} \times 1.35 \rfloor + \text{priorityFee}\right)$$

* **保底优先小费**：`priorityFee` 默认维持至少 $0.001\text{ Gwei}$（$1{,}000{,}000\text{ wei}$）的打包激励缓冲。
* **自愈回退基线**：若 RPC 节点未暴露 `baseFee` 或返回 0，引擎平滑回退至使用当前 `gasPrice` 作为公式计算基线。

> [!NOTE]
> **经济学原理与零额外 Gas 开销保证**：  
> 在 EIP-1559 机制下，`maxFeePerGas` 仅代表交易发送方愿意承担的**最高价格上限**；链上实际燃烧并扣减的 Gas 费用严格等于打包区块的**实际 `baseFee + priorityFee`**。因此，将费用上限设置 1.5 倍安全缓冲，可在**不多花 1 wei 冤枉钱**的前提下，100% 免疫突发的区块费用尖峰。

##### 3. 自适应 Legacy 回退机制
当对接不支持 Type 2 交易的私有链、定制测试网或旧版 EVM 节点时（捕获包含 `not supported` 或 `invalid transaction type` 的 RPC 错误）：
* 执行引擎**自动降级回退**为带 1.5 倍缓冲的 **Legacy Type 0 交易**。
* 回退后的 Legacy `gasPrice` 同样取 $\max(\text{maxFeePerGas},\; \lfloor \text{gasPrice} \times 1.35 \rfloor + 1{,}000{,}000)$，确保在传统链上依然具备抗费率抖动能力。

---

### 3.2 模式 2：动态 Gas 附加费补偿（规划中 / 2027 Q1 v0.2.0）

* **架构角色分工：**
  * **买方（AI Agent）：** 使用单一代币（USDC）同时支付 API 服务费与所需 Gas 成本，依然无需持有原生 ETH。
  * **卖方网关：** 动态估算 Gas 消耗量，并通过价格预言机将 Gas 成本折算为 USDC 最小单位。
  * **Facilitator：** 代付节点通过附加费获得原生 Gas 支出的全额补偿。
* **运行机制与公式：**
  $$\text{Total Amount} = \text{Base Price} + \text{Gas Surcharge}$$
  $$\text{Gas Surcharge (USDC)} = \left( \text{Estimated Gas} \times \text{Effective Gas Price} \times \text{ETH/USD Oracle} \right) \times (1 + \text{Slippage Margin})$$
  1. 构造 `x402` 挑战时，网关查询 `EvmRpcClient.estimate_gas()` 与当前实时 Gas 价格。
  2. 网关将附加费合并至挑战的 `amount` 中（或写入可选挑战扩展字段 `gas_surcharge: {"amount": "...", "asset": "USDC"}`）。
  3. 买方金融防火墙（策略引擎）校验 `total_amount <= policy.max_amount_per_tx`。
  4. 买方对 `total_amount` 签署授权。
  5. Facilitator 广播交易，通过分账合约或直接收取附加费以补偿原生 Gas 支出。
* **适用场景：**
  * 高 Gas 链（以太坊主网 L1）、网络拥堵高峰期、低毛利或算力透传类 API（例如无法自行吸收 $0.05 Gas 成本的重度 GPU 推理服务）。

---

### 3.3 模式 3：批量聚合与状态通道延迟结算（规划中 / 2027 Q2 v0.3.0）

* **架构角色分工：**
  * **买方（AI Agent）：** 高频签发微支付授权。
  * **卖方网关与 Facilitator：** 将多笔支付在链下聚合或通过状态通道累计，按批次统一上链结算，大幅摊薄单次请求的 Gas 成本。
* **运行机制：**
  * **子方案 A（批量聚合执行）：**
    * 网关在链下接收并校验 EIP-3009 签名，通过本地 Redis/PostgreSQL 缓存核验 Nonce。
    * 验签通过后网关立即交付 API 响应。
    * 批量结算工作线程定期（例如每累计 100 笔请求或每 5 分钟）将多笔授权打包进一次 `batchTransferWithAuthorization` 合约调用中，将 21,000 基础 EVM 交易开销分摊到 $N$ 笔微支付上（单笔均摊 Gas 从约 $80,000$ 降至约 $30,000$）。
  * **子方案 B（状态通道 / 额度流式支付）：**
    * 买方预先将预算锁定至托管/状态通道合约，或向状态通道管理器授权额度。
    * 每次 API 调用或每个生成的 Token 流，买方仅签署递增的状态更新凭证。
    * 仅当通道关闭或预算到期时，才向区块链提交一笔净额结算交易。
* **适用场景：**
  * 超低延迟、亚美分级微支付（例如每次 LLM 输出 Token $0.0001、流式遥测数据、按查询计费的搜索排序服务）。

---

## 4. 架构对比与选型矩阵

| 评估维度 | 模式 1：卖方补贴型 | 模式 2：动态附加费 | 模式 3：批量聚合 |
| :--- | :--- | :--- | :--- |
| **买方 Gas 代币需求** | 零（仅需 USDC） | 零（仅需 USDC） | 零（仅需 USDC） |
| **买方实际支出** | 仅 API 基础价格 | 基础价格 + 预估 Gas | 基础价格 + 极小分摊比例 |
| **卖方 Gas 支出** | 承担每笔 L2 Gas | 全额获得补偿 | 大幅摊薄（<5%） |
| **结算延迟** | 逐请求实时（1-2s） | 逐请求实时（1-2s） | 周期性 / 批量结算 |
| **智能合约依赖** | 标准 EIP-3009（Circle USDC） | 标准 EIP-3009 或分账合约 | 批量结算合约 / 状态通道合约 |
| **目标发布版本** | **v0.1.0 (2026)** | **v0.2.0 (2027 Q1)** | **v0.3.0 (2027 Q2)** |

---

## 5. 网关配置 Schema 演进草案（2027）

```yaml
version: "2.0"

server:
  host: "0.0.0.0"
  port: 8000

gas_policy:
  mode: "dynamic_surcharge" # 可选: "subsidized", "dynamic_surcharge", "batch_aggregated"
  
  # 模式 2 配置（动态附加费）
  surcharge:
    gas_limit: 85000
    slippage_buffer_pct: 10
    oracle_source: "chainlink"
    oracle_address: "0x..."
    max_acceptable_gas_usd: 0.05
    
  # 模式 3 配置（批量聚合）
  batch:
    max_batch_size: 50
    flush_interval_seconds: 60
    min_accumulated_usd: 5.00
```
