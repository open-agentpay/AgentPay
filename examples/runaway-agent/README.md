# Runaway Agent Circuit Breaker & Financial Firewall Demo

This example demonstrates how **AgentPay** provides a local financial firewall and autonomous circuit breaker for AI agents, preventing wallet drain from hallucinated or runaway loops while clearing real micro-payments on **Arbitrum Sepolia**.

---

## 3 Core Capabilities Demonstrated:

1. **Phase 1: 正常微支付结算与链上出块 (Normal Payment & On-Chain Settlement)**
   - Agent executes a paid API call via LangChain `@paid_tool` (0.05 USDC).
   - Satisfies HTTP 402 challenge via EIP-3009 offline signing (0 gas paid by buyer).
   - Gateway routes settlement to Facilitator on **Arbitrum Sepolia**.
   - Emits structured log with **on-chain `txHash`** and **Arbiscan Explorer link**.
2. **Phase 2: 单笔超额本地主动防御 (Single-Tx Cap Defense)**
   - Agent attempts to call an expensive enterprise compute endpoint requiring 2.50 USDC.
   - Buyer policy allows max 0.10 USDC per transaction.
   - Financial firewall **immediately aborts** payment locally: 0 funds leave wallet, 0 gas spent.
   - Clean feedback observation returned to LangChain agent.
3. **Phase 3: 失控死循环与熔断器介入 (Runaway Infinite Loop Tripped Circuit Breaker)**
   - Agent gets stuck in a rapid tool-calling loop (limit: 5 requests per minute).
   - Exceeding the rate limit immediately trips the circuit breaker: `CLOSED` -> `OPEN`.
   - All subsequent loop attempts are fast-failed locally with `CIRCUIT_BREAKER_OPEN`.
   - Protects the user's wallet from infinite financial bleeding.

---

---

## Usage Modes

### Mode 1: Default Simulation (`make demo`)
* **Zero Network Overhead & Zero Gas:** Uses a built-in EVM mock JSON-RPC handler running locally.
* Simulates EIP-712 / EIP-3009 offline signing, challenge verification, and produces deterministic 32-byte simulated tx hashes (e.g., `0x7f9a12c8...01`).
* **Note:** Because this mode runs completely locally without touching the public Arbitrum network, simulated tx hashes will **not** exist on `sepolia.arbiscan.io`.
```bash
make demo
```

### Mode 2: Live Arbitrum Sepolia Testnet (`make demo-live`)
* Broadcasts real raw transactions to the public Arbitrum Sepolia testnet JSON-RPC node.
* Settled transactions are permanently recorded on-chain and directly viewable on [Arbitrum Sepolia Arbiscan](https://sepolia.arbiscan.io).
* **Prerequisites:**
  1. **Facilitator Wallet:** Needs testnet ETH on Arbitrum Sepolia for L2 gas. (Get testnet ETH from [QuickNode Arbitrum Sepolia Faucet](https://faucet.quicknode.com/arbitrum/sepolia) or [Alchemy Faucet](https://www.alchemy.com/faucets/arbitrum-sepolia)).
  2. **Buyer Wallet:** Needs testnet Circle USDC (`0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d`) on Arbitrum Sepolia. (Get testnet USDC from [Circle Web3 Faucet](https://faucet.circle.com/)).
```bash
export ARBITRUM_SEPOLIA_RPC_URL="https://sepolia-rollup.arbitrum.io/rpc"
export FACILITATOR_PRIVATE_KEY="0x..." # Private key with testnet ETH
export BUYER_PRIVATE_KEY="0x..."       # Private key with testnet USDC
make demo-live
```

---

## Sample Structured Log Output (with `txHash`)

```json
[Facilitator] ON_CHAIN_TRANSACTION_CONFIRMED
{
  "timestamp": "2026-10-18T00:38:27.064060+00:00",
  "level": "INFO",
  "component": "Facilitator",
  "event": "ON_CHAIN_TRANSACTION_CONFIRMED",
  "network": "Arbitrum Sepolia (eip155:421614)",
  "txHash": "0x7f9a12c8402f1a6012e831293a10385923985712984572918237912837192801",
  "blockNumber": 12345679,
  "arbiscanUrl": "https://sepolia.arbiscan.io/tx/0x7f9a12c8402f1a6012e831293a10385923985712984572918237912837192801",
  "payer": "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
  "payee": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
  "amountUsdc": "0.05",
  "gasSponsoredBy": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
  "gasChargedToBuyer": "0.0000 ETH (Free EIP-3009 Transfer)",
  "status": "CONFIRMED"
}
```

```json
[BuyerFirewall] CIRCUIT_BREAKER_TRIPPED
{
  "timestamp": "2026-10-18T00:38:27.166677+00:00",
  "level": "ERROR",
  "component": "BuyerFirewall",
  "event": "CIRCUIT_BREAKER_TRIPPED",
  "iteration": 5,
  "breakerState": "OPEN",
  "reason": "CIRCUIT_MAX_RPM_EXCEEDED (Max 5 RPM exceeded)",
  "cooldownSeconds": 300,
  "action": "LOOP_HALTED_WALLET_FROZEN",
  "fundsDrainedThisCall": "0.00 USDC"
}
```
