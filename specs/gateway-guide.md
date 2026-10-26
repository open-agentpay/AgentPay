# AgentPay Self-Hosted Gateway Guide & Configuration Reference

> **Document Version:** 1.0.0  
> **Target Service:** `agentpay-gateway` (Python FastAPI / ASGI Reverse Proxy)  
> **Status:** Production Ready (2026 Baseline)  
> **Audience:** API Providers, SaaS Merchants, DevOps Engineers, and Platform Administrators  

---

## 1. Overview & Architectural Role

The **AgentPay Gateway** (`agentpay-gateway`) is a self-hosted, cloud-native HTTP reverse proxy that sits directly in front of protected upstream APIs, microservices, or AI model providers. It acts as an autonomous billing and monetization gateway:

```mermaid
graph LR
    Client[AI Agent / Buyer Client] -->|1. Request without auth| Gateway[agentpay-gateway]
    Gateway -->|2. HTTP 402 + x402 Challenge| Client
    Client -->|3. Request + PAYMENT-SIGNATURE| Gateway
    Gateway -->|4. Verify Signature & Nonce| Facilitator[agentpay-facilitator]
    Facilitator -->|5. Signature OK| Gateway
    Gateway -->|6. Forward with Identity Headers| Upstream[Protected Backend API]
    Upstream -->|7. API Response 200 OK| Gateway
    Gateway -->|8. Deliver Response to Agent| Client
    Gateway -.->|9. Async Settle| Facilitator
```

### Core Responsibilities
1. **Zero-Code Monetization**: Converts any standard REST or HTTP microservice into a paid, pay-per-call API without changing upstream backend code.
2. **x402 V2 Protocol Handling**: Automatically issues standard `HTTP 402 Payment Required` challenges and verifies cryptographically signed `PAYMENT-SIGNATURE` headers.
3. **Idempotency & Double-Payment Protection**: Coordinates distributed transaction state (`GatewayStateCoordinator`) via SQLite or PostgreSQL, guaranteeing clients are never double-billed for retries or network interruptions.
4. **Optimistic Low-Latency Fulfillment**: Delivers responses immediately upon cryptographic signature verification, asynchronously batching and submitting blockchain settlements via the Facilitator service.
5. **Secure Upstream Identity Propagation**: Injects authenticated payment context (`X-Payer-Address`, `X-Payment-Amount`, `X-Payment-Tx-Hash`) into upstream requests, stripping untrusted or hop-by-hop client headers.

---

## 2. Quickstart: 60-Second Gateway Deployment

### 2.1 Installation
Install the gateway package from source or wheel:

```bash
# Clone and install in virtual environment
git clone https://github.com/open-agentpay/AgentPay.git
cd AgentPay
python3 -m venv .venv && source .venv/bin/activate
pip install -e sdk/python/packages/agentpay -e sdk/python/packages/agentpay-gateway
```

### 2.2 Minimal Configuration (`gateway.yaml`)
Create a minimal `gateway.yaml` defining your server and a protected route:

```yaml
version: "1.0"

server:
  host: "0.0.0.0"
  port: 8000
  db_path: "./data/gateway.db"
  default_timeout_seconds: 10.0

facilitator:
  url: "http://127.0.0.1:8001"
  timeout_seconds: 5.0

settlement:
  default_network: "eip155:421614"                        # Arbitrum Sepolia
  default_asset: "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d" # Circle USDC
  default_asset_symbol: "USDC"
  default_asset_decimals: 6
  default_pay_to: "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

routes:
  - path: "/v1/market-data"
    upstream: "https://httpbin.org/get"
    price: "0.001 USDC"
```

### 2.3 Starting the Gateway
Run the gateway server using the built-in CLI module:

```bash
python3 -m agentpay_gateway.server --config ./gateway.yaml
```

The gateway binds to `0.0.0.0:8000`. Any unauthenticated request to `http://localhost:8000/v1/market-data` instantly returns an `HTTP 402 Payment Required` challenge.

---

## 3. Comprehensive `gateway.yaml` Configuration Reference

### 3.1 Server Block (`server`)
Configures the network listener and local persistence parameters.

| Field | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `host` | `string` | `"0.0.0.0"` | Network interface to bind. Use `127.0.0.1` for local-only testing. |
| `port` | `integer` | `8000` | Port number to listen on (1–65535). |
| `db_path` | `string` | `"./data/gateway.db"` | Path to the SQLite coordinator database file. Directories are auto-created. |
| `default_timeout_seconds` | `float` | `10.0` | Default socket and response timeout for upstream requests. |
| `audit_log_path` | `string \| null` | `null` | Optional file path for structured JSON audit events. |

### 3.2 Facilitator Block (`facilitator`)
Configures connection parameters to the Facilitator verification and settlement service.

| Field | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `url` | `string` | `"http://127.0.0.1:8001"` | URL of the Facilitator service. Must start with `http://` or `https://`. |
| `timeout_seconds` | `float` | `5.0` | Client timeout when calling Facilitator endpoints (`/v1/verify`, `/v1/settle`). |

### 3.3 Settlement Defaults (`settlement`)
Defines fallback blockchain and payment parameters when not overridden at the route level.

| Field | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `default_network` | `string` | `"eip155:8453"` | Default CAIP-2 network identifier (e.g. `eip155:421614` for Arbitrum Sepolia). |
| `default_asset` | `string` | Base USDC address | Canonical ERC-20 token contract address (auto-normalized to lowercase). |
| `default_asset_symbol` | `string` | `"USDC"` | Human-readable token symbol for price parsing. |
| `default_asset_decimals` | `integer` | `6` | Number of token decimals (Circle USDC is 6). |
| `default_challenge_ttl_seconds` | `integer` | `120` | Expiration window in seconds for issued 402 challenges. |
| `safe_buffer_seconds` | `integer` | `60` | Extra reconciliation buffer before unlocking expired leases. |
| `default_pay_to` | `string \| null` | `null` | Default merchant receiving EVM address (0x format). |

### 3.4 Route Block (`routes`)
List of route rules mapping incoming path prefixes to protected upstream services.

| Field | Type | Required | Description |
| :--- | :---: | :---: | :--- |
| `path` | `string` | **Yes** | Path prefix to match (e.g. `"/v1/search"` or `"/api/v2/generate"`). |
| `upstream` | `string` | **Yes** | Target backend base URL (e.g. `"http://internal-api.local:3000"`). |
| `price` | `string \| int` | **Yes** | Price per request. Examples: `"0.001 USDC"`, `"0.05 USDC"`, or integer base units (`1000`). |
| `pay_to` | `string` | No | Overrides `settlement.default_pay_to` for this specific route. |
| `network` | `string` | No | Overrides `settlement.default_network` for this route. |
| `asset` | `string` | No | Overrides `settlement.default_asset` for this route. |
| `asset_symbol` | `string` | No | Asset symbol for price parsing (default `"USDC"`). |
| `asset_decimals` | `integer` | No | Asset decimals (default `6`). |
| `methods` | `list[str]` | No | Allowed HTTP methods. Default: `["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"]`. |
| `timeout_seconds` | `float` | No | Upstream timeout override for long-running endpoints (e.g. `60.0` for LLM inference). |
| `challenge_ttl_seconds` | `integer` | No | Challenge TTL override for this route. |
| `upstream_headers` | `dict` | No | Static headers injected into upstream requests (e.g. backend API keys or service secrets). |
| `strip_path_prefix` | `bool` | No | If `true`, strips the matched `path` prefix before proxying to `upstream`. |

---

## 4. Multi-Route Production Configuration Example

```yaml
version: "1.0"

server:
  host: "0.0.0.0"
  port: 8000
  db_path: "/var/lib/agentpay/gateway.db"
  default_timeout_seconds: 15.0
  audit_log_path: "/var/log/agentpay/audit.log"

facilitator:
  url: "http://facilitator.agentpay-cluster.internal:8001"
  timeout_seconds: 5.0

settlement:
  default_network: "eip155:421614"                          # Arbitrum Sepolia
  default_asset: "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d"   # Circle USDC
  default_asset_symbol: "USDC"
  default_asset_decimals: 6
  default_challenge_ttl_seconds: 180
  default_pay_to: "0x70997970c51812dc3a010c7d01b50e0d17dc79c8" # Primary treasury

routes:
  # Micro-pricing for fast telemetry and quotes
  - path: "/v1/market-data"
    upstream: "http://market-service.internal:5000"
    price: "0.001 USDC"
    methods: ["GET"]
    timeout_seconds: 3.0

  # Heavy compute / LLM inference endpoint
  - path: "/v1/inference/deepseek"
    upstream: "http://vllm-cluster.internal:8000"
    price: "0.05 USDC"
    methods: ["POST"]
    timeout_seconds: 120.0
    strip_path_prefix: true
    upstream_headers:
      Authorization: "Bearer internal-vllm-secret-key"

  # Dedicated payee for partner dataset
  - path: "/v1/datasets/satellite"
    upstream: "http://storage-service.internal:9000/data"
    price: "0.25 USDC"
    pay_to: "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc" # Partner payout wallet
```

---

## 5. Upstream Integration & Forwarded Headers

When a request passes payment verification, the Gateway forwards the request to your upstream service while enriching it with cryptographic identity metadata:

### 5.1 Injected Upstream Headers

| Injected Header | Description | Example Value |
| :--- | :--- | :--- |
| `X-Payer-Address` | Verified EVM address of the paying Agent | `0x6c7a9c5b4655075b6ff9292f71b822f56f833e56` |
| `X-Payment-Amount` | Paid amount in atomic token base units | `1000` (equal to 0.001 USDC) |
| `X-Payment-Asset` | ERC-20 contract address of the paid token | `0x75faf114eafb1bdbe2f0316df893fd58ce46aa4d` |
| `X-Payment-Network` | CAIP-2 network identifier | `eip155:421614` |
| `X-Payment-Tx-Hash` | Settlement transaction hash (or settlement ID) | `0x7f9a12c8...` or `stl_3cb150c9...` |
| `X-Payment-Idempotency-Key`| Client idempotency key bound to the payment | `idem_8f7b2c9e...` |

### 5.2 Header Stripping & Security Boundary
To prevent request spoofing, the Gateway automatically strips any incoming client headers matching:
- `X-Payer-*`
- `X-Payment-*`
- Hop-by-hop headers (`Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Transfer-Encoding`)

---

## 6. Docker & Kubernetes Deployment

### 6.1 Dockerfile
```dockerfile
FROM python:3.12-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY python/ /app/python/
RUN pip install --no-cache-dir \
    -e /app/sdk/python/packages/agentpay \
    -e /app/sdk/python/packages/agentpay-gateway

EXPOSE 8000

ENTRYPOINT ["python3", "-m", "agentpay_gateway.server"]
CMD ["--config", "/etc/agentpay/gateway.yaml"]
```

### 6.2 Docker Compose (`docker-compose.yaml`)
```yaml
version: "3.8"

services:
  agentpay-facilitator:
    build:
      context: .
      dockerfile: Dockerfile
    command: ["python3", "-m", "agentpay_gateway.facilitator.server", "--port", "8001"]
    environment:
      - FACILITATOR_PRIVATE_KEY=${FACILITATOR_PRIVATE_KEY}
      - ARBITRUM_SEPOLIA_RPC_URL=${ARBITRUM_SEPOLIA_RPC_URL:-https://sepolia-rollup.arbitrum.io/rpc}
    ports:
      - "8001:8001"
    restart: unless-stopped

  agentpay-gateway:
    build:
      context: .
      dockerfile: Dockerfile
    volumes:
      - ./gateway.yaml:/etc/agentpay/gateway.yaml:ro
      - gateway-data:/var/lib/agentpay
    ports:
      - "8000:8000"
    depends_on:
      - agentpay-facilitator
    restart: unless-stopped

volumes:
  gateway-data:
```

### 6.3 Kubernetes Manifest (`gateway-deployment.yaml`)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agentpay-gateway
  labels:
    app: agentpay-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agentpay-gateway
  template:
    metadata:
      labels:
        app: agentpay-gateway
    spec:
      containers:
        - name: gateway
          image: ghcr.io/open-agentpay/agentpay-gateway:v0.1.0
          args: ["--config", "/etc/agentpay/gateway.yaml"]
          ports:
            - containerPort: 8000
          volumeMounts:
            - name: config-volume
              mountPath: /etc/agentpay
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8000
            initialDelaySeconds: 2
            periodSeconds: 5
      volumes:
        - name: config-volume
          configMap:
            name: gateway-config
---
apiVersion: v1
kind: Service
metadata:
  name: agentpay-gateway-svc
spec:
  type: LoadBalancer
  ports:
    - port: 443
      targetPort: 8000
  selector:
    app: agentpay-gateway
```

---

## 7. Troubleshooting & Operational FAQ

### Q1: Upstream returns HTTP 502 Bad Gateway
- Check whether the upstream service URL specified in `routes[].upstream` is reachable from the gateway container.
- If upstream is running on the host machine, use `http://host.docker.internal:<port>` on macOS/Windows, or the host IP on Linux.

### Q2: 402 Challenge negotiation fails with "EIP-712 domain mismatch"
- Ensure that `network` (`eip155:421614`) and `asset` contract address in `gateway.yaml` precisely match the chain and token contract the Buyer client is signing for.

### Q3: How do I rotate the Facilitator Gas Key?
- Update `FACILITATOR_PRIVATE_KEY` in your environment or Secret Manager and restart the `agentpay-facilitator` service. No changes are required in `agentpay-gateway`.
