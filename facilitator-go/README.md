# AgentPay Facilitator (Go)

High-performance EVM clearing and EIP-712 cryptographic verification daemon for the AgentPay payment gateway.

## Features

- **Blazing Fast Cryptographic Verification**: Verifies EIP-712 / EIP-3009 typed data signatures using native `secp256k1` curves via `go-ethereum/crypto` (>10,000 verifications/sec per core).
- **Dual Anti-Replay Guard**: Combines thread-safe in-memory Nonce tracking with on-chain `authorizationState(authorizer, nonce)` verification over EVM JSON-RPC.
- **Gasless On-Chain Settlement**: Generates EIP-3009 `transferWithAuthorization` calldata (selector `0xe3ee160e`), sponsors Gas with the Facilitator's private key, and broadcasts EIP-1559 DynamicFee transactions directly to Arbitrum, Base, and other EVM rollups.
- **Async Receipt Reconciliation**: Background goroutines automatically poll transaction receipts, verifying confirmations or handling on-chain reverts with automatic status tracking.
- **Zero Protocol Friction**: Fully adheres to AgentPay API Specs §2 and is 100% drop-in compatible with `agentpay-gateway-go` (`gate.openagentpay.dev`).

## Architecture & API Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/verify` | Off-chain & on-chain verification of x402 challenge & signature |
| `POST` | `/v1/settle` | Settle verified payment on-chain via EIP-3009 gas sponsorship |
| `GET` | `/v1/settle/{id}` | Poll settlement transaction status & block receipt |
| `GET` | `/v1/nonce` | Query whether a given nonce is already consumed |
| `GET` | `/health` | Health & liveness check with Facilitator address |

## Quickstart

### 1. Build & Run Locally

```bash
cd facilitator-go
go build -o bin/facilitator ./cmd/facilitator
./bin/facilitator -config configs/facilitator.yaml
```

### 2. Environment Variables

| Variable | Description | Default |
|---|---|---|
| `FACILITATOR_PORT` / `PORT` | Listening HTTP port | `8001` |
| `FACILITATOR_HOST` | Listening interface | `0.0.0.0` |
| `RPC_URL` | EVM JSON-RPC endpoint URL | `https://sepolia-rollup.arbitrum.io/rpc` |
| `CHAIN_ID` | EVM Chain ID | `421614` (Arbitrum Sepolia) |
| `USDC_ADDRESS` | Token contract address | `0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d` |
| `FACILITATOR_PRIVATE_KEY` | Private key for transaction signing & gas | `""` (ephemeral key for local dev) |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |

### 3. Docker Build

```bash
docker build -t openagentpay/facilitator:latest ./facilitator-go
docker run -p 8001:8001 openagentpay/facilitator:latest
```
