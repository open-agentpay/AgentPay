# Gas Economics & Settlement Sponsorship Models Specification

## 1. Background & Problem Statement

In Web3 micro-payment architectures for autonomous AI agents, transaction gas fees (miner/sequencer fees) represent a critical friction point:

1. **AI Agent Friction:** Requiring an AI Agent wallet to hold both an ERC-20 payment token (e.g. USDC) and native gas tokens (ETH/L2 ETH) introduces multi-asset balance management complexity, gas price fluctuation risks, and high failure rates when gas runs out.
2. **EIP-3009 Gasless Property:** EIP-3009 (`transferWithAuthorization`) natively supports meta-transactions. The token owner (`from`) only generates an off-chain cryptographic signature (EIP-712). The entity submitting the transaction (`msg.sender`) to the blockchain is responsible for paying EVM gas fees.
3. **Economics Across Different Scenarios:** Depending on chain layer (L1 vs L2), API price elasticity, frequency of calls (streaming vs discrete), and network congestion, a single fixed gas payment approach cannot fit all enterprise and developer needs.

To achieve production-grade flexibility, AgentPay specifies **three supported Gas sponsorship and settlement models**.

---

## 2. Taxonomy of Gas Models

```
┌────────────────────────────────────────────────────────────────────────┐
│                        AgentPay Gas Models                             │
├──────────────────────────┬─────────────────────────┬───────────────────┤
│ Mode 1: Seller-Subsidized│ Mode 2: Dynamic         │ Mode 3: Batch &   │
│         Gasless (2026)   │         Surcharge (2027)│ Channels (2027)   │
├──────────────────────────┼─────────────────────────┼───────────────────┤
│ • Buyer: 0 Gas (USDC only│ • Buyer: Pays Price +   │ • Buyer: Signs    │
│ • Facilitator/Seller pays│   Gas Surcharge in USDC │   interim states  │
│ • Zero agent friction    │ • Facilitator breaks    │ • Aggregated      │
│ • Optimal for L2 micro-tx│   even on native gas    │   on-chain commit │
│                          │ • Optimal for L1 / heavy│ • Optimal for sub-│
│                          │   or low-margin APIs    │   cent / streaming│
└──────────────────────────┴─────────────────────────┴───────────────────┘
```

---

## 3. Specification of the Three Gas Models

### 3.1 Mode 1: Seller-Subsidized Gasless Settlement (当前已实现 / 2026 基线)

* **Architecture Role:**
  * **Buyer (AI Agent):** 100% Gasless. Only holds USDC.
  * **Seller Gateway / Facilitator:** Maintains an operational wallet with native tokens (e.g., Arbitrum Sepolia ETH / Base ETH) to sponsor execution gas.
* **Mechanism:**
  1. Gateway issues standard `x402` challenge specifying exact base price (e.g. `1000` = 0.001 USDC).
  2. Buyer signs EIP-712 `TransferWithAuthorization` for `1000` atomic units.
  3. Facilitator submits `transferWithAuthorization` with `msg.sender = Facilitator`, paying L2 gas (approx. $0.001 - $0.005 on Arbitrum/Base).
  4. The 0.001 USDC is transferred entirely from Buyer to Merchant Payee.
* **Economic Rationale:**
  * On Layer 2 rollups, gas costs are negligible compared to SaaS API value. Sellers treat gas as infrastructure overhead (similar to AWS bandwidth costs) or incorporate it into base API pricing.
* **Implementation Status:** Supported in AgentPay v0.1.0 (`agentpay-gateway`, `EvmRpcClient`, Arbitrum Sepolia Facilitator).

#### 3.1.1 Facilitator 链上广播执行规范：EIP-1559 动态费率与抗拒防波动机制

在 L2 Rollup（如 Arbitrum Nitro、Base）以及以太坊主网出块结算时，底层网络的 `baseFee` 随区块交易密度动态变动。为了确保 Facilitator 履约网关在异步广播 `transferWithAuthorization` 交易时**绝不因区块 BaseFee 毫秒级微幅上涨被节点拒绝**（如 `RPC error -32000: max fee per gas less than block base fee`），AgentPay Facilitator 链上结算执行引擎规范了以下费率管理机制：

##### 1. 采用 EIP-1559（Type 2）标准交易
* 链上广播默认采用 **EIP-1559（Type 2）交易结构**，显式指定 `maxFeePerGas` 与 `maxPriorityFeePerGas`，彻底摒弃存在瞬时波动竞争缺陷的固定费率 Legacy Type 0 结构。

##### 2. 1.5x BaseFee 安全缓冲公式
Facilitator 在组装链上结算交易前，通过 RPC 查询最新区块头 `baseFee`、优先费 `priorityFee` 与全局 `gasPrice`，按以下公式计算动态费率上限：

$$\text{maxFeePerGas} = \max\left(\lfloor \text{baseFee} \times 1.5 \rfloor + \text{priorityFee},\; \lfloor \text{gasPrice} \times 1.35 \rfloor + \text{priorityFee}\right)$$

* **优先费保底**：`priorityFee` 默认至少保留 $0.001\text{ Gwei}$（$1{,}000{,}000\text{ wei}$）优先级缓冲。
* **自愈基准**：当 RPC 节点未暴露区块 `baseFee` 或为 0 时，自动平滑退化为当前 `gasPrice` 作为基准值代入公式计算。

> [!NOTE]
> **经济学原理与 Gas 零额外开销保证**：  
> 在 EIP-1559 机制下，`maxFeePerGas` 仅代表交易发送者愿意承受的**费率上限**，链上实际扣除的 Gas 费用依然是出块时的**实际 baseFee + priorityFee**。因此，将费率上限上浮 1.5 倍可以 100% 免疫出块波动带来的交易拒绝，且**不会多花任何一分 Gas**。

##### 3. 自适应降级兼容（Adaptive Legacy Fallback）
若遇到不支持 Type 2 交易的私有链节点、特定开发网或古老 EVM 节点（捕获包含 `not supported` 或 `invalid transaction type` 的 RPC 错误）：
* 执行引擎**自动降级**为带 1.5x 缓冲的 **Legacy Type 0 交易**；
* 降级后的 Legacy `gasPrice` 同样设定为 $\max(\text{maxFeePerGas},\; \lfloor \text{gasPrice} \times 1.35 \rfloor + 1{,}000{,}000)$，确保在 Legacy 链上同样具备防御费率波动的韧性。

---

### 3.2 Mode 2: Dynamic Gas Surcharge & Reimbursement (规划中 / 2027 Q1 v0.2.0)

* **Architecture Role:**
  * **Buyer (AI Agent):** Pays for both the API service and the required gas in a single token (USDC), still needing zero native ETH.
  * **Seller Gateway:** Dynamically estimates gas consumption and converts gas cost into USDC atomic units using a price oracle.
  * **Facilitator:** Sponsoring node is reimbursed for gas outlay via the surcharge.
* **Mechanism & Formula:**
  $$\text{Total Amount} = \text{Base Price} + \text{Gas Surcharge}$$
  $$\text{Gas Surcharge (USDC)} = \left( \text{Estimated Gas} \times \text{Effective Gas Price} \times \text{ETH/USD Oracle} \right) \times (1 + \text{Slippage Margin})$$
  1. When constructing the `x402` challenge, Gateway queries `EvmRpcClient.estimate_gas()` and current gas price.
  2. Gateway appends the surcharge into the challenge `amount` (or optional challenge extension `gas_surcharge: {"amount": "...", "asset": "USDC"}`).
  3. Buyer's Financial Firewall (Policy Engine) validates that `total_amount <= policy.max_amount_per_tx`.
  4. Buyer signs authorization for `total_amount`.
  5. Facilitator broadcasts transaction; contract splits or distributes funds, or Facilitator collects the surcharge as reimbursement for native gas expenditure.
* **Applicability:**
  * High-gas chains (Ethereum Mainnet L1), network congestion spikes, low-margin or pass-through compute APIs (e.g., heavy GPU inference where $0.05 gas cannot be absorbed).

---

### 3.3 Mode 3: Batch Aggregation & State Channel Delayed Settlement (规划中 / 2027 Q2 v0.3.0)

* **Architecture Role:**
  * **Buyer (AI Agent):** Issues high-frequency micro-authorizations.
  * **Seller Gateway & Facilitator:** Aggregates and settles multiple payments in batches or state channels, drastically amortizing per-request gas costs.
* **Mechanism:**
  * **Sub-approach A (Batch Execution):**
    * Gateway accepts and validates EIP-3009 signatures off-chain, verifying nonces against local Redis/SQLite cache.
    * Gateway delivers API responses immediately upon signature verification.
    * A batch settlement worker periodically (e.g., every 100 requests or every 5 minutes) packages authorizations into a `batchTransferWithAuthorization` contract call, amortizing the base 21,000 EVM transaction overhead across $N$ micro-payments ($Gas/tx \approx 30,000$ instead of $80,000$).
  * **Sub-approach B (State Channel / Allowance Streaming):**
    * Buyer locks a budget into an escrow / channel contract or approves a state-channel manager.
    * For each API call or generated token stream, Buyer signs incremental state updates.
    * Only upon channel close or budget expiry is a single net settlement transaction submitted to the blockchain.
* **Applicability:**
  * Ultra-low-latency, sub-cent micro-payments (e.g., $0.0001 per LLM token output, streaming telemetry, per-query search ranking).

---

## 4. Architectural Comparison & Selection Matrix

| Dimension | Mode 1: Seller-Subsidized | Mode 2: Dynamic Surcharge | Mode 3: Batch Aggregation |
| :--- | :--- | :--- | :--- |
| **Buyer Gas Token Need** | Zero (USDC only) | Zero (USDC only) | Zero (USDC only) |
| **Buyer Cost** | Base price only | Base price + estimated gas | Base price + amortized fraction |
| **Seller Gas Expense** | Pays L2 gas per tx | Fully reimbursed | Substantially amortized (<5%) |
| **Settlement Latency** | Per-request (1-2s) | Per-request (1-2s) | Periodic / Batched |
| **Smart Contract Footprint**| Standard EIP-3009 (Circle USDC)| Standard EIP-3009 or Fee Splitter | Batch Registry / Channel Contract |
| **Release Target** | **v0.1.0 (2026)** | **v0.2.0 (2027 Q1)** | **v0.3.0 (2027 Q2)** |

---

## 5. Gateway Configuration Schema Evolution (Draft 2027)

```yaml
version: "2.0"

server:
  host: "0.0.0.0"
  port: 8000

gas_policy:
  mode: "dynamic_surcharge" # options: "subsidized", "dynamic_surcharge", "batch_aggregated"
  
  # Configuration for Mode 2 (Dynamic Surcharge)
  surcharge:
    gas_limit: 85000
    slippage_buffer_pct: 10
    oracle_source: "chainlink"
    oracle_address: "0x..."
    max_acceptable_gas_usd: 0.05
    
  # Configuration for Mode 3 (Batch Aggregation)
  batch:
    max_batch_size: 50
    flush_interval_seconds: 60
    min_accumulated_usd: 5.00
```
