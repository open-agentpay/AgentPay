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

### 3.1 Mode 1: Seller-Subsidized Gasless Settlement (Implemented / 2026 Baseline)

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

#### 3.1.1 Facilitator On-Chain Broadcast Execution Spec: EIP-1559 Dynamic Fee Pricing and Volatility Defense

During block settlement on L2 Rollups (such as Arbitrum Nitro, Base) and Ethereum mainnet, the underlying network's `baseFee` fluctuates dynamically with block transaction density. To ensure that the Facilitator fulfillment gateway, when asynchronously broadcasting `transferWithAuthorization` transactions, **is never rejected by nodes due to sub-second micro-spikes in block BaseFee** (e.g., `RPC error -32000: max fee per gas less than block base fee`), the AgentPay Facilitator on-chain settlement execution engine standardizes the following fee management mechanisms:

##### 1. Mandatory EIP-1559 (Type 2) Standard Transactions
* On-chain broadcasts default to the **EIP-1559 (Type 2) transaction format**, explicitly specifying `maxFeePerGas` and `maxPriorityFeePerGas`, entirely abandoning the fixed-rate Legacy Type 0 format prone to transient fee-spike rejections.

##### 2. 1.5x BaseFee Safety Buffer Formula
Before assembling on-chain settlement transactions, the Facilitator queries the latest block header `baseFee`, priority fee `priorityFee`, and global `gasPrice` via JSON-RPC, computing the dynamic fee cap using the following formula:

$$\text{maxFeePerGas} = \max\left(\lfloor \text{baseFee} \times 1.5 \rfloor + \text{priorityFee},\; \lfloor \text{gasPrice} \times 1.35 \rfloor + \text{priorityFee}\right)$$

* **Guaranteed Priority Tip**: `priorityFee` defaults to maintaining at least a $0.001\text{ Gwei}$ ($1{,}000{,}000\text{ wei}$) priority incentive buffer.
* **Self-Healing Baseline**: If an RPC node does not expose `baseFee` or returns 0, the engine smoothly falls back to using the current `gasPrice` as the baseline in the formula.

> [!NOTE]
> **Economic Rationale and Zero Gas Overhead Guarantee**:  
> Under EIP-1559 mechanics, `maxFeePerGas` represents only the **maximum price ceiling** the transaction sender is willing to pay. The actual gas amount burned and debited on-chain is strictly determined by the mined block's **actual baseFee + priorityFee**. Therefore, applying a 1.5x safety buffer to the fee ceiling provides 100% immunity against sudden block fee spikes **without spending a single extra wei of gas**.

##### 3. Adaptive Legacy Fallback
When interacting with private chains, custom devnets, or older EVM nodes that do not support Type 2 transactions (catching RPC errors containing `not supported` or `invalid transaction type`):
* The execution engine **automatically falls back** to a **Legacy Type 0 transaction** with a 1.5x buffer.
* The fallback legacy `gasPrice` is similarly calculated as $\max(\text{maxFeePerGas},\; \lfloor \text{gasPrice} \times 1.35 \rfloor + 1{,}000{,}000)$, preserving resilience against fee volatility even on legacy chains.

---

### 3.2 Mode 2: Dynamic Gas Surcharge & Reimbursement (Planned / 2027 Q1 v0.2.0)

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

### 3.3 Mode 3: Batch Aggregation & State Channel Delayed Settlement (Planned / 2027 Q2 v0.3.0)

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
