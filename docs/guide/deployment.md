---
title: Deployment Guide
description: Containerized deployment, cloud orchestration, environment variable reference, reverse proxy setup, and operational troubleshooting for AgentPay.
---

# Deployment Guide

This guide explains how to deploy the core AgentPay stack — **Go Gateway (`services/gateway`)**, **Go Facilitator (`services/facilitator`)**, **Management Console (`apps/console`)**, **Marketplace & GEO Portal (`apps/marketplace-web`)**, and **PostgreSQL** — using Docker Compose on a local machine or cloud server (AWS EC2, Alibaba Cloud ECS, Tencent Cloud CVM, DigitalOcean, etc.).

::: warning Experimental Release Notice
Version 0.1.x is an experimental open-source release and has **not** undergone an independent third-party security audit. Deploy only on testnets (such as Arbitrum Sepolia or Base Sepolia) using test tokens (Circle testnet USDC). **Do not connect it to mainnet funds.**
:::

---

## 1. Deployment Topologies & Architecture

AgentPay supports two deployment topologies tailored to local development and cloud-hosted multi-tenant operation:

| Dimension | Local Embedded Sandbox (`make demo`) | Cloud-Native Containerized (`docker compose`) |
| --- | --- | --- |
| **Use cases** | Local feature checks, unit tests, CI/CD pipelines | Shared team sandbox, testnet integration, multi-tenant cloud hosting |
| **Network transport** | In-process memory channel (zero port binding) | Isolated container network `agentpay-net` + standard HTTP/HTTPS |
| **Data persistence** | In-memory or local SQLite file | PostgreSQL 15 persistent volume (`postgres_data`) |
| **On-chain settlement** | Local EVM RPC simulator (zero gas cost) | Dedicated `agentpay-facilitator` node connected to live testnet RPC |
| **Buyer private key** | Local test key (`LocalSigner`) | Strictly stays in the buyer agent's local environment / KMS; **never uploaded** |

### Container Orchestration Overview

```text
  [Operators / Tenant Admins]       [Developers / AI Crawlers]        [Buyer AI Agents / SDK Clients]
               │                                 │                                   │
               ▼ :3000                           ▼ :3001                             ▼ :8000
      ┌─────────────────┐               ┌─────────────────────┐             ┌─────────────────┐
      │ agentpay-console│               │agentpay-marketplace │             │ agentpay-gateway│
      │  (Vue3 + Nginx) │               │(Next.js Standalone) │─ Internal ─▶│    (Go Core)    │
      └────────┬────────┘               └─────────────────────┘             └────────┬────────┘
               │                                                                     │
               └──────────────────────── /api/* Reverse Proxy ───────────────────────┤
                                                                                     │
                                    ┌────────────────────────────────────────────────┼────────────┐
                                    ▼ :5432                                          ▼ :8001      ▼
                           ┌─────────────────┐                              ┌────────────────┐  ┌──────────────┐
                           │agentpay-postgres│                              │   agentpay-    │  │  Protected   │
                           │ (PostgreSQL 15) │                              │  facilitator   │  │ Upstream API │
                           └─────────────────┘                              └────────┬───────┘  └──────────────┘
                                                                                     │
                                                                                     ▼ HTTPS JSON-RPC
                                                                            [Arbitrum / Base Testnet]
```

### Container Services Reference

The root `docker-compose.yml` and `docker-compose.prod.yml` orchestrate the following services:

| Service | Container Name | Default Image | Container Port | Default Host Port | Core Responsibility | Health Check |
| --- | --- | --- | --- | --- | --- | --- |
| `postgres` | `agentpay-postgres` | `postgres:15-alpine` | `5432` | `5432` (`127.0.0.1:5432` in prod) | Stores tenants, projects, agents, API key hashes, virtual wallets, double-entry ledger, routes, and audit events; auto-runs `infra/postgres/schema.sql` on first boot | `pg_isready` |
| `gateway` | `agentpay-gateway` | `openagentpay/gateway:latest` | `8000` | `8000` | Go reverse proxy & control plane: x402 challenges, wallet Hold/Capture/Release, zero-downtime RCU hot reload, idempotency, and `/api/v1/*` Admin API | `http://127.0.0.1:8000/healthz` |
| `facilitator` | `agentpay-facilitator` | `openagentpay/facilitator:latest` | `8001` | `8001` | Go settlement node: off-chain EIP-712 / EIP-3009 signature verification (`/v1/verify`), nonce replay protection, and gas-sponsored EIP-1559 broadcasts (`/v1/settle`) | `http://127.0.0.1:8001/health` |
| `console` | `agentpay-console` | `openagentpay/console:latest` | `80` | `3000` | Vue 3 + Element Plus multi-tenant dashboard; embedded Nginx proxies `/api/*` and `/healthz` to `gateway:8000` | `http://127.0.0.1:80/` |
| `marketplace` | `agentpay-marketplace` | `openagentpay/marketplace:latest` | `3000` | `3001` | Next.js 15 public API storefront & GEO (Generative Engine Optimization) portal serving published route catalogs and `/llms.txt` | `http://127.0.0.1:3000/` |
| `upstream-api` | `agentpay-upstream-api` | `python:3.12-alpine` | `9000` | Internal network only | Included in `docker-compose.yml` only; provides a lightweight mock upstream server inside `agentpay-net` for local testing | — |

---

## 2. System Requirements & Port Planning

### Hardware & Software Requirements

| Resource | Minimum (Dev / Sandbox) | Recommended (Cloud Staging / Multi-Tenant Testnet) |
| --- | --- | --- |
| **CPU / Memory** | 2 vCPU / 2 GB RAM | 4 vCPU / 8 GB RAM |
| **Disk Space** | 10 GB free | 40 GB+ SSD (for PostgreSQL volumes and audit logs) |
| **Operating System** | Linux / macOS / Windows (WSL 2) | Ubuntu 22.04 / 24.04 LTS or Debian 12 |
| **Docker** | Docker 24.0+ | Docker 24.0+ with log rotation enabled |
| **Docker Compose** | Compose v2 (`docker compose`) | Compose v2.20+ |

### Port & Firewall Rules

Follow the principle of least privilege when configuring cloud security groups or host firewalls (e.g., `ufw`):

| Port | Service | Recommended Exposure | Notes |
| --- | --- | --- | --- |
| `443` / `80` | Front Nginx / Caddy / ALB | Public (`0.0.0.0/0`) | Terminate TLS at a reverse proxy or load balancer and route by hostname to internal container ports |
| `8000` | `agentpay-gateway` | Public or reverse-proxy only | Entrypoint for AI Agent paid API traffic and `/api/v1/*` control plane APIs |
| `3000` | `agentpay-console` | Restricted or via reverse proxy | Tenant and administrator web console |
| `3001` | `agentpay-marketplace` | Public or via reverse proxy | Public API storefront and `/llms.txt` discovery endpoint |
| `8001` | `agentpay-facilitator` | **Internal / Localhost only (`127.0.0.1`)** | Holds the gas-sponsoring key; only needs to be reached by `gateway` inside `agentpay-net` |
| `5432` | `agentpay-postgres` | **Never expose to public internet** | Database port; restrict to `agentpay-net` and localhost maintenance |

---

## 3. Option 1: Full-Stack Deployment from Source (`docker-compose.yml`)

Use the root `docker-compose.yml` when you have cloned the repository and want to build container images directly from local source code.

### Step 1: Clone the Repository and Create `.env`

```bash
git clone https://github.com/open-agentpay/AgentPay.git
cd AgentPay

cp .env.example .env
```

Review `.env` and update `POSTGRES_PASSWORD` and `AGENTPAY_JWT_SECRET` if the machine is reachable outside localhost.

### Step 2: Build and Start All Containers

Use the `Makefile` shortcut or run `docker compose` directly:

```bash
# Option A: via Makefile (runs docker compose up -d --build)
make compose-up

# Option B: via Docker Compose directly
docker compose up -d --build
```

### Step 3: Verify Container Status

```bash
docker compose ps
```

Every core service should report `Up (healthy)` (`upstream-api` reports `Up`):

```text
NAME                   IMAGE                            SERVICE        STATUS          PORTS
agentpay-console       openagentpay/console:latest      console        Up (healthy)    0.0.0.0:3000->80/tcp
agentpay-facilitator   openagentpay/facilitator:latest  facilitator    Up (healthy)    0.0.0.0:8001->8001/tcp
agentpay-gateway       openagentpay/gateway:latest      gateway        Up (healthy)    0.0.0.0:8000->8000/tcp
agentpay-marketplace   openagentpay/marketplace:latest  marketplace    Up (healthy)    0.0.0.0:3001->3000/tcp
agentpay-postgres      postgres:15-alpine               postgres       Up (healthy)    0.0.0.0:5432->5432/tcp
agentpay-upstream-api  python:3.12-alpine               upstream-api   Up              9000/tcp
```

---

## 4. Option 2: Lightweight Cloud Deployment (`docker-compose.prod.yml`)

When deploying to a cloud server, **you do not need to copy the full source tree or install Go/Node.js toolchains on the host**. `docker-compose.prod.yml` pulls pre-built images directly from the container registry.

### Step 1: Install Docker and Compose on the Cloud Host

On Ubuntu / Debian:

```bash
curl -fsSL https://get.docker.com | bash
sudo systemctl enable --now docker
sudo apt-get update && sudo apt-get install -y docker-compose-plugin
```

### Step 2: Upload the Minimal Deployment Bundle

Only 4 configuration files are required on the server:

```text
~/agentpay/
├── docker-compose.yml               # Copied from local docker-compose.prod.yml
├── .env                             # Copied and customized from .env.example
├── infra/postgres/schema.sql        # PostgreSQL initialization DDL (mounted read-only)
└── services/gateway/configs/gateway.yaml  # Go gateway bootstrap config (mounted read-only)
```

From your workstation, copy these files to the server (replace `user@your-server-ip` with your server address):

```bash
ssh user@your-server-ip "mkdir -p ~/agentpay/infra/postgres ~/agentpay/services/gateway/configs"

scp docker-compose.prod.yml user@your-server-ip:~/agentpay/docker-compose.yml
scp .env.example user@your-server-ip:~/agentpay/.env
scp infra/postgres/schema.sql user@your-server-ip:~/agentpay/infra/postgres/
scp services/gateway/configs/gateway.yaml user@your-server-ip:~/agentpay/services/gateway/configs/
```

### Step 3: Configure Production `.env`

SSH into the server and edit `~/agentpay/.env`:

```bash
cd ~/agentpay
vim .env
```

Replace the default database password and JWT secret (`AGENTPAY_JWT_SECRET` should be at least 32 characters; generate one with `openssl rand -hex 32`):

```ini
# Image Registry & Tag
DOCKER_REGISTRY=openagentpay
IMAGE_TAG=latest

# PostgreSQL (bound to 127.0.0.1:5432 by default in docker-compose.prod.yml)
POSTGRES_DB=agentpay
POSTGRES_USER=agentpay
POSTGRES_PASSWORD=ReplaceWithStrongRandomDBPassword_2026!
POSTGRES_PORT=127.0.0.1:5432

# Go Gateway
GATEWAY_PORT=8000
AGENTPAY_JWT_SECRET=ReplaceWith64CharHexHighEntropyJWTSecretKeyGeneratedByOpenSSL!
LOG_LEVEL=info

# Console & Marketplace Ports
CONSOLE_PORT=3000
MARKETPLACE_PORT=3001

# Marketplace Public URLs (replace with your public domains or IP)
NEXT_PUBLIC_GATEWAY_URL=https://gate.example.com
NEXT_PUBLIC_CONSOLE_URL=https://console.example.com
NEXT_PUBLIC_SITE_URL=https://market.example.com

# Facilitator Node (Arbitrum Sepolia Testnet)
FACILITATOR_PORT=127.0.0.1:8001
RPC_URL=https://sepolia-rollup.arbitrum.io/rpc
CHAIN_ID=421614
USDC_ADDRESS=0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d
FACILITATOR_PRIVATE_KEY=
```

::: tip About `FACILITATOR_PRIVATE_KEY`
- If you only use **Path A (API Key Virtual Wallet)** or local mock verification, leave `FACILITATOR_PRIVATE_KEY` empty.
- If you enable **Path B (On-Chain x402 / EIP-3009 live testnet settlement)**, provide the private key of an operational wallet funded with a small amount of Arbitrum Sepolia (or target testnet) ETH. This wallet only sponsors L2 gas and never holds buyer or seller USDC.
:::

### Step 4: Pull Images and Start

```bash
cd ~/agentpay
docker compose pull
docker compose up -d
docker compose ps
```

---

## 5. Environment Variables & Configuration Reference

### 5.1 `.env` Compose Variables

| Variable | Default | Scope | Description |
| --- | --- | --- | --- |
| `DOCKER_REGISTRY` | `openagentpay` | All images | Container registry prefix (e.g., Docker Hub org, GHCR, or private registry) |
| `IMAGE_TAG` | `latest` | All images | Image version tag |
| `POSTGRES_DB` | `agentpay` | `postgres` / `gateway` | PostgreSQL database name |
| `POSTGRES_USER` | `agentpay` | `postgres` / `gateway` | PostgreSQL database user |
| `POSTGRES_PASSWORD` | `agentpay_strong_...` | `postgres` / `gateway` | PostgreSQL password; **must be changed before deployment** |
| `POSTGRES_PORT` | `5432` (`127.0.0.1:5432` in prod) | `postgres` | Host port mapping for PostgreSQL |
| `GATEWAY_PORT` | `8000` | `gateway` | Host port mapping for the Go Gateway |
| `AGENTPAY_JWT_SECRET` | Example secret | `gateway` | JWT signing key for console auth & Admin API; **gateway refuses to start if empty** |
| `LOG_LEVEL` | `info` | `gateway` / `facilitator` | Log level: `debug`, `info`, `warn`, `error` |
| `CONSOLE_PORT` | `3000` | `console` | Host port mapping for the Web Console |
| `MARKETPLACE_PORT` | `3001` | `marketplace` | Host port mapping for the Marketplace portal |
| `NEXT_PUBLIC_GATEWAY_URL` | `http://localhost:8000` | `marketplace` | Public gateway URL displayed in Marketplace code snippets |
| `NEXT_PUBLIC_CONSOLE_URL` | `http://localhost:3000` | `marketplace` | Public console URL linked from Marketplace pages |
| `NEXT_PUBLIC_SITE_URL` | `http://localhost:3001` | `marketplace` | Canonical site URL used for SEO metadata and `/llms.txt` |
| `FACILITATOR_PORT` | `8001` | `facilitator` | Host port mapping for the Facilitator service |
| `RPC_URL` | `https://sepolia-rollup.arbitrum.io/rpc` | `facilitator` | EVM JSON-RPC endpoint URL |
| `CHAIN_ID` | `421614` | `facilitator` | Target EVM Chain ID (`421614` for Arbitrum Sepolia, `84532` for Base Sepolia) |
| `USDC_ADDRESS` | `0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d` | `facilitator` | Circle USDC token contract address on the target chain |
| `FACILITATOR_PRIVATE_KEY` | Empty | `facilitator` | Gas-sponsoring private key used to broadcast `transferWithAuthorization` transactions |

### 5.2 Go Gateway Environment Overrides

On startup, `agentpay-gateway` loads `-config /etc/agentpay/gateway.yaml` and then applies environment variable overrides (environment variables take precedence over YAML values):

| Container Env Var | Overrides `gateway.yaml` Field | Description |
| --- | --- | --- |
| `AGENTPAY_HOST` | `server.host` | Listener bind address (`0.0.0.0` inside container) |
| `AGENTPAY_PORT` | `server.port` | Listener port (`8000` inside container) |
| `AGENTPAY_POSTGRES_URL` | `database.postgres_url` | PostgreSQL connection string; if unreachable at boot, the gateway logs a warning and falls back to in-memory mode |
| `AGENTPAY_FACILITATOR_URL` | `facilitator.url` | Facilitator base URL (set to `http://facilitator:8001` inside Compose) |
| `AGENTPAY_JWT_SECRET` | `auth.jwt_secret` | Control plane JWT signing secret |
| `AGENTPAY_LOG_LEVEL` | `logger.level` | `debug`, `info`, `warn`, or `error` |
| `AGENTPAY_LOG_FORMAT` | `logger.format` | `json` (recommended for log aggregation) or `text` |

::: details Full `services/gateway/configs/gateway.yaml` Production Template
```yaml
version: "2.0.0"

server:
  host: "0.0.0.0"
  port: 8000
  read_timeout_seconds: 15.0
  write_timeout_seconds: 30.0
  idle_timeout_seconds: 60.0
  shutdown_timeout_seconds: 10.0

auth:
  jwt_secret: "replace-via-AGENTPAY_JWT_SECRET-env-var-in-production"
  token_duration_hours: 24

database:
  postgres_url: "postgres://agentpay:password@postgres:5432/agentpay?sslmode=disable"
  max_open_conns: 25
  max_idle_conns: 10
  conn_max_lifetime_seconds: 300

facilitator:
  url: "http://facilitator:8001"
  timeout_seconds: 5.0

settlement:
  default_network: "eip155:421614"                          # Arbitrum Sepolia
  default_asset: "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d"   # Circle USDC
  default_asset_symbol: "USDC"
  default_asset_decimals: 6
  default_pay_to: "0x70997970C51812dc3A010C7d01b50e0d17dc79C8" # Default seller payout address

logger:
  level: "info"
  format: "json"

metrics:
  enabled: true
  path: "/metrics"
  health_path: "/healthz"
  ready_path: "/readyz"

routes:
  - route_id: "rou_market_data"
    path: "/v1/market-data"
    upstream: "https://httpbin.org/get"
    price_base_units: 1000                                  # 0.001 USDC
    asset: "USDC"
    payee_address: "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
    allowed_payment_methods: "both"
    timeout_seconds: 10.0
    rate_limit_rpm: 600
```
See [Gateway Configuration](/guide/gateway) and [Policy Configuration](/guide/policy) for detailed route and policy options.
:::

---

## 6. Building & Pushing Custom Multi-Arch Images

If you customize the gateway, console, or marketplace, use `scripts/docker-build-push.sh` to build and publish images to your own container registry.

### Build Images Locally

```bash
# Build all 4 service images with default openagentpay/*:latest tags
make docker-build

# Or specify a custom tag
./scripts/docker-build-push.sh --tag v0.1.2
```

### Cross-Compile and Push Multi-Arch Images

When building on Apple Silicon (`arm64`) for `x86_64` (`amd64`) cloud servers, pass `--platform` to invoke `docker buildx`:

```bash
# 1. Authenticate with your container registry
docker login

# 2. Build and push linux/amd64 + linux/arm64 images
./scripts/docker-build-push.sh \
  --push \
  --registry registry.example.com/myorg \
  --tag v0.1.2 \
  --platform linux/amd64,linux/arm64
```

The script automatically injects the Git commit hash, build timestamp, and version tag into the `gateway` binary via `-ldflags`, exposed at `/version`.

---

## 7. Post-Deployment Verification Checklist

Run the following checks in order after starting the stack (replace `127.0.0.1` with your server IP or domain when testing remotely):

### 1. Check Gateway Liveness, Readiness, and Version Metadata

```bash
curl -s http://127.0.0.1:8000/healthz
curl -s http://127.0.0.1:8000/readyz
curl -s http://127.0.0.1:8000/version
```

- `/healthz` returns `200 OK` when the gateway process is alive.
- `/readyz` returns `200 OK` when the RCU configuration snapshot is loaded and ready to serve traffic.
- `/version` returns the image version, Git commit, and build timestamp.

### 2. Confirm PostgreSQL Persistence Mode in Logs

Verify that the gateway connected to PostgreSQL rather than falling back to in-memory storage:

```bash
docker compose logs gateway | grep -i "persistence mode"
```

Expected output:

```text
AgentPay Control Plane operating in PostgreSQL persistence mode
AgentPay Usage & Event Recording operating in PostgreSQL persistence mode
AgentPay Payment Correlation operating in PostgreSQL persistence mode
AgentPay Agent Behavior Timeline operating in PostgreSQL persistence mode
```

### 3. Check Facilitator Health

```bash
curl -s http://127.0.0.1:8001/health
```

### 4. Verify `402 Payment Required` Route Interception

Call the sample paid route `/v1/market-data` without credentials:

```bash
curl -i http://127.0.0.1:8000/v1/market-data
```

You should receive `HTTP/1.1 402 Payment Required` along with a `PAYMENT-REQUIRED` challenge header and an `X-Config-Version` header.

### 5. Verify Console & Marketplace Portals

- **Management Console**: Open `http://<server-ip>:3000/` in a browser and confirm the login/registration screen loads.
- **Marketplace Portal**: Open `http://<server-ip>:3001/` and `http://<server-ip>:3001/llms.txt` to verify storefront rendering and AI discovery output.

---

## 8. Reverse Proxy & HTTPS Setup

Never transmit API keys (`apk_live_...`) or console JWTs over plain HTTP on public networks. Place **Nginx**, **Caddy**, or a cloud load balancer (ALB / Cloudflare) in front of the containers to terminate TLS 1.2/1.3.

### Recommended Subdomain Layout

| Domain Example | Target Container | Purpose |
| --- | --- | --- |
| `gate.example.com` | `http://127.0.0.1:8000` | Gateway data plane (AI Agent traffic) & public API |
| `console.example.com` | `http://127.0.0.1:3000` | Multi-tenant Management Console |
| `market.example.com` | `http://127.0.0.1:3001` | Public Marketplace & `/llms.txt` portal |

### Sample Nginx Configuration

```nginx
# 1. AgentPay Gateway Data Plane (gate.example.com)
server {
    listen 443 ssl http2;
    server_name gate.example.com;

    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Accommodate long-running LLM inference routes
        proxy_connect_timeout 15s;
        proxy_read_timeout 120s;
        proxy_send_timeout 60s;
    }
}

# 2. AgentPay Management Console (console.example.com)
server {
    listen 443 ssl http2;
    server_name console.example.com;

    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# 3. AgentPay Marketplace Portal (market.example.com)
server {
    listen 443 ssl http2;
    server_name market.example.com;

    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

When using a host reverse proxy, bind container ports to loopback in `.env` (`GATEWAY_PORT=127.0.0.1:8000`, `CONSOLE_PORT=127.0.0.1:3000`, `MARKETPLACE_PORT=127.0.0.1:3001`) so external traffic cannot bypass TLS.

---

## 9. Operations, Upgrades & Backups

### Common Operational Commands

```bash
# Follow real-time service logs
docker compose logs -f gateway
docker compose logs -f facilitator
docker compose logs -f console
docker compose logs -f postgres

# Scrape Prometheus metrics
curl -s http://127.0.0.1:8000/metrics

# Gracefully restart the gateway (sends SIGTERM and drains active requests)
docker compose restart gateway

# Pull updated images and perform a rolling recreate
docker compose pull
docker compose up -d

# Stop containers while preserving the postgres_data volume
docker compose down
```

::: danger Avoid `docker compose down -v` in Shared Environments
Passing `-v` deletes the `postgres_data` persistent volume, permanently wiping all tenants, wallet balances, double-entry ledger entries, API keys, and audit logs. Use `-v` only when intentionally resetting a local development database.
:::

### PostgreSQL Backup & Restore

Virtual wallet balances, append-only ledger entries (`ledger_entries`), and payment state records live in PostgreSQL. Schedule regular backups:

```bash
# Dump a compressed database snapshot
docker exec agentpay-postgres pg_dump -U agentpay -d agentpay | gzip > agentpay_backup_$(date +%Y%m%d_%H%M%S).sql.gz

# Restore from a snapshot
gunzip -c agentpay_backup_20261024_120000.sql.gz | docker exec -i agentpay-postgres psql -U agentpay -d agentpay
```

---

## 10. Security Baseline Checklist

Before opening a deployment to your team or external integrations, verify every item below:

- [ ] **Rotate default secrets**: Replace `POSTGRES_PASSWORD` and `AGENTPAY_JWT_SECRET` in `.env` with high-entropy random strings and restrict file permissions (`chmod 600 .env`).
- [ ] **Isolate database & Facilitator ports**: Confirm ports `5432` (PostgreSQL) and `8001` (Facilitator) are blocked from public ingress and reachable only via `agentpay-net` or `127.0.0.1`.
- [ ] **Enforce HTTPS**: Serve all external gateway, console, and marketplace traffic over TLS so `Authorization: Bearer apk_live_...` headers and admin tokens are never sent in cleartext.
- [ ] **Keep buyer keys on the buyer side**: Never place a buyer agent's signing key (`AGENT_PRIVATE_KEY`) on the gateway server.
- [ ] **Cap Facilitator hot-wallet balance**: Fund the `FACILITATOR_PRIVATE_KEY` address only with minimal testnet ETH needed for L2 gas sponsorship.
- [ ] **Inject upstream secrets at the gateway**: Configure upstream API keys (e.g., internal service tokens or LLM provider keys) via gateway route `upstream_headers` or console upstream auth rather than distributing them to buyer agents.

---

## 11. Troubleshooting

| Symptom | Likely Cause | Resolution |
| --- | --- | --- |
| `gateway` container restarts repeatedly with `jwt secret key cannot be empty` | Both `auth.jwt_secret` and `AGENTPAY_JWT_SECRET` are empty | The Go gateway fails closed when no JWT secret is configured. Set a non-empty `AGENTPAY_JWT_SECRET` in `.env` |
| Tenants or routes created in the Console disappear after restarting `gateway` | Gateway could not reach PostgreSQL at boot and fell back to `in-memory mode` | Check `docker compose logs gateway` for DB connection errors and verify `AGENTPAY_POSTGRES_URL` credentials match the `postgres` container |
| Changing `POSTGRES_PASSWORD` in `.env` causes authentication failures | The existing `postgres_data` volume was initialized with the old password | Connect with the old password and run `ALTER USER`, or reset a disposable dev volume with `docker compose down -v && docker compose up -d` |
| On-chain x402 verification fails at the gateway | `gateway` cannot reach `facilitator`, or chain/token settings mismatch | Set `facilitator.url` to `http://facilitator:8001` (or `AGENTPAY_FACILITATOR_URL=http://facilitator:8001`) and ensure route `network` / `asset` match `CHAIN_ID` / `USDC_ADDRESS` |
| Paid route returns `502 Bad Gateway` | Gateway container cannot reach the configured `upstream` URL | Inside a container, `127.0.0.1` refers to the container itself. Use the host's LAN IP or `http://host.docker.internal:<port>` for services running on the host |
| Console UI loads, but login or API calls fail with `502` / `504` | Nginx inside `console` cannot reach `http://gateway:8000` | Run `docker compose ps` and `docker compose logs gateway` to verify `gateway` is `healthy` and on the `agentpay-net` network |

---

## Where to Go Next

| I want to… | Read |
| --- | --- |
| Run the full local flow & circuit breaker demo in 5 minutes | [5-Minute Quickstart](/guide/quickstart) |
| Configure routes, pricing, and upstream authentication | [Gateway Configuration](/guide/gateway) |
| Set per-transaction caps, rolling budgets, and circuit breakers | [Policy Configuration](/guide/policy) |
| Manage tenants, agents, wallets, and config snapshots in the UI | [Management Console](/guide/console) |
| Understand the dual-rail architecture and trust boundaries | [Architecture Specification](/specs/architecture) |
