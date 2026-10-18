# Paid API Gateway Example

This example demonstrates how an API provider protects a normal upstream REST service with the `agentpay-gateway`.

## Key Capabilities Demonstrated:
- Reverse proxy routing (`gateway.yaml`)
- Fixed pricing and automatic 402 challenge emission
- Facilitator verification & on-chain settlement
- Idempotency caching and free replay on upstream 5xx

## Usage:
```bash
docker compose up gateway
```
