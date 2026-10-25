---
title: 部署指南
description: AgentPay 容器化部署、云服务器生产环境编排、环境变量参考、反向代理配置与运维排查完整指南。
---

# 部署指南

本指南介绍如何将 AgentPay 的核心服务——**Go 高性能网关（`gateway-go`）**、**链上清算节点（`facilitator-go`）**、**Web 管理控制台（`console`）**、**公开 API 商店与 GEO 门户（`marketplace-web`）** 以及 **PostgreSQL 数据库**——通过 Docker Compose 部署到本地服务器或云主机（AWS EC2、阿里云 ECS、腾讯云 CVM、DigitalOcean 等）。

::: warning 实验性版本提示
当前版本（v0.1.x）属于实验性开源版本，尚未经过独立第三方安全审计。仅建议在测试网（如 Arbitrum Sepolia、Base Sepolia）与测试代币（Circle 测试网 USDC）环境下部署演练，**切勿接入主网真实资金**。
:::

---

## 1. 部署拓扑与组件架构

AgentPay 支持两种部署形态，分别面向本地开发测试与多租户云端运行：

| 维度 | 本地嵌入式沙箱（`make demo`） | 云原生容器化部署（`docker compose`） |
| --- | --- | --- |
| **适用场景** | 本地功能验证、单元测试、CI/CD 流水线 | 团队共享沙箱、测试网联调、云端多租户托管 |
| **网络传输** | 进程内内存通道（零端口占用） | 独立容器网络 `agentpay-net` + 标准 HTTP/HTTPS |
| **数据存储** | 内存或本地 SQLite 文件 | PostgreSQL 15 持久化数据卷（`postgres_data`） |
| **链上结算** | 本地 EVM RPC 模拟器（零 Gas 消耗） | 独立 `agentpay-facilitator` 节点对接真实测试网 RPC |
| **买方私钥位置** | 本地测试密钥（`LocalSigner`） | 始终保存在买方 Agent 本地环境 / KMS，**绝不上传云端** |

### 云端容器编排全景

```text
  [运营/租户管理员浏览器]         [开发者/AI 搜索引擎]            [买方 AI Agent / SDK 客户端]
            │                             │                                   │
            ▼ :3000                       ▼ :3001                             ▼ :8000
   ┌─────────────────┐           ┌─────────────────────┐             ┌─────────────────┐
   │ agentpay-console│           │agentpay-marketplace │             │ agentpay-gateway│
   │  (Vue3 + Nginx) │           │(Next.js Standalone) │─ 内部拉取 ─▶│   (Go 网关核心)  │
   └────────┬────────┘           └─────────────────────┘             └────────┬────────┘
            │                                                                 │
            └────────────────────── /api/* 反向代理 ──────────────────────────┤
                                                                              │
                                 ┌────────────────────────────────────────────┼────────────┐
                                 ▼ :5432                                      ▼ :8001      ▼
                        ┌─────────────────┐                          ┌────────────────┐  ┌──────────────┐
                        │agentpay-postgres│                          │   agentpay-    │  │ 受保护的上游  │
                        │ (PostgreSQL 15) │                          │  facilitator   │  │ 业务微服务   │
                        └─────────────────┘                          └────────┬───────┘  └──────────────┘
                                                                              │
                                                                              ▼ HTTPS JSON-RPC
                                                                     [Arbitrum / Base 测试网]
```

### 容器服务一览

仓库根目录的 `docker-compose.yml` 与 `docker-compose.prod.yml` 编排了以下核心组件：

| 服务名 | 容器名 | 默认镜像 | 容器端口 | 默认宿主机端口 | 核心职责 | 健康检查端点 |
| --- | --- | --- | --- | --- | --- | --- |
| `postgres` | `agentpay-postgres` | `postgres:15-alpine` | `5432` | `5432`（生产模式仅绑 `127.0.0.1`） | 存储租户、项目、Agent、API Key 哈希、虚拟钱包、双录账本、路由配置与审计事件；首次启动自动执行 `infra/postgres/schema.sql` | `pg_isready` |
| `gateway` | `agentpay-gateway` | `openagentpay/gateway:latest` | `8000` | `8000` | Go 反向代理与控制面：x402 价格挑战、钱包冻结/扣款/释放、RCU 零停机热加载、幂等协调与 `/api/v1/*` 管理接口 | `http://127.0.0.1:8000/healthz` |
| `facilitator` | `agentpay-facilitator` | `openagentpay/facilitator:latest` | `8001` | `8001` | Go 链上结算节点：EIP-712 / EIP-3009 离线签名核验（`/v1/verify`）、Nonce 防重放检查与代付 Gas 的 EIP-1559 异步上链广播（`/v1/settle`） | `http://127.0.0.1:8001/health` |
| `console` | `agentpay-console` | `openagentpay/console:latest` | `80` | `3000` | Vue 3 + Element Plus 多租户管理控制台；内置 Nginx 将 `/api/*` 与 `/healthz` 转发至 `gateway:8000` | `http://127.0.0.1:80/` |
| `marketplace` | `agentpay-marketplace` | `openagentpay/marketplace:latest` | `3000` | `3001` | Next.js 15 公开 API 商店与 GEO（Generative Engine Optimization）门户，提供已发布路由目录与 `/llms.txt` | `http://127.0.0.1:3000/` |
| `upstream-api` | `agentpay-upstream-api` | `python:3.12-alpine` | `9000` | 仅容器内网暴露 | 仅包含在 `docker-compose.yml` 中，提供轻量级内网 Mock 上游服务便于联调测试 | — |

---

## 2. 环境要求与端口规划

### 硬件与软件要求

| 资源项 | 最低配置（开发 / 沙箱） | 推荐配置（云服务器联调 / 多租户测试） |
| --- | --- | --- |
| **CPU / 内存** | 2 vCPU / 2 GB RAM | 4 vCPU / 8 GB RAM |
| **磁盘空间** | 10 GB 可用空间 | 40 GB+ SSD（用于 PostgreSQL 数据卷与审计日志） |
| **操作系统** | Linux / macOS / Windows (WSL 2) | Ubuntu 22.04 / 24.04 LTS 或 Debian 12 |
| **Docker** | Docker 24.0+ | Docker 24.0+ 且启用日志轮转 |
| **Docker Compose** | Compose v2（`docker compose`） | Compose v2.20+ |

### 端口与防火墙规划

在云服务器安全组（Security Group）或宿主机防火墙（如 `ufw`）中，请遵循最小暴露原则：

| 端口 | 对应服务 | 推荐暴露范围 | 说明 |
| --- | --- | --- | --- |
| `443` / `80` | 前置 Nginx / Caddy / ALB | 公网开放（`0.0.0.0/0`） | 生产环境推荐由前置反向代理统一终止 TLS，再按域名分流至内部容器端口 |
| `8000` | `agentpay-gateway` | 公网开放或仅对反向代理开放 | AI Agent 调用付费 API 及控制台访问 `/api/v1/*` 的入口 |
| `3000` | `agentpay-console` | 按需开放或通过反向代理暴露 | 租户与管理员 Web 控制台 |
| `3001` | `agentpay-marketplace` | 公网开放或通过反向代理暴露 | 公开 API 商店与 `/llms.txt` 索引页 |
| `8001` | `agentpay-facilitator` | **仅限内网 / 本机（`127.0.0.1`）** | 持有代付 Gas 私钥的清算节点，仅需被 `gateway` 容器在 `agentpay-net` 内网调用 |
| `5432` | `agentpay-postgres` | **严禁对公网开放** | 数据库端口，仅限容器内网或本机运维访问 |

---

## 3. 方式一：本地或开发服务器一键部署（源码构建模式）

如果你已经克隆了完整代码仓库，或希望在修改代码后直接从本地源码构建镜像并启动全套服务，请使用仓库根目录的 `docker-compose.yml`。

### 步骤 1：获取源码并生成 `.env` 配置

```bash
git clone https://github.com/open-agentpay/AgentPay.git
cd AgentPay

cp .env.example .env
```

根据需要编辑 `.env` 文件（在本地实验时可直接保留默认值；若对外网暴露，请务必修改 `POSTGRES_PASSWORD` 与 `AGENTPAY_JWT_SECRET`）。

### 步骤 2：构建并启动所有容器

你可以使用 `Makefile` 快捷目标，或直接执行 `docker compose`：

```bash
# 方式 A：使用 Makefile（自动执行 docker compose up -d --build）
make compose-up

# 方式 B：直接使用 docker compose
docker compose up -d --build
```

### 步骤 3：检查容器健康状态

```bash
docker compose ps
```

所有服务的 `STATUS` 均应显示为 `Up (healthy)`（`upstream-api` 为 `Up`）：

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

## 4. 方式二：云服务器轻量部署（预构建镜像模式）

在云服务器上部署时，**无需上传整个代码仓库，也无需在服务器上安装 Go 或 Node.js 编译环境**。仓库提供了专门的 `docker-compose.prod.yml`，直接拉取预构建镜像运行。

### 步骤 1：在云服务器上安装 Docker 与 Compose 插件

如果目标服务器尚未安装 Docker（以 Ubuntu / Debian 为例）：

```bash
curl -fsSL https://get.docker.com | bash
sudo systemctl enable --now docker
sudo apt-get update && sudo apt-get install -y docker-compose-plugin
```

### 步骤 2：上传最小部署文件集

云端运行仅需 4 个配置文件：

```text
~/agentpay/
├── docker-compose.yml               # 由本地 docker-compose.prod.yml 复制而来
├── .env                             # 由本地 .env.example 复制并修改
├── infra/postgres/schema.sql        # PostgreSQL 初始化表结构（只读挂载）
└── gateway-go/configs/gateway.yaml  # Go 网关初始配置（只读挂载）
```

在本地开发机执行以下命令，将上述文件同步至云服务器（将 `user@your-server-ip` 替换为你的服务器地址）：

```bash
ssh user@your-server-ip "mkdir -p ~/agentpay/infra/postgres ~/agentpay/gateway-go/configs"

scp docker-compose.prod.yml user@your-server-ip:~/agentpay/docker-compose.yml
scp .env.example user@your-server-ip:~/agentpay/.env
scp infra/postgres/schema.sql user@your-server-ip:~/agentpay/infra/postgres/
scp gateway-go/configs/gateway.yaml user@your-server-ip:~/agentpay/gateway-go/configs/
```

### 步骤 3：配置云服务器 `.env`

SSH 登录到云服务器并编辑 `~/agentpay/.env`：

```bash
cd ~/agentpay
vim .env
```

请务必替换默认的数据库密码与 JWT 密钥（`AGENTPAY_JWT_SECRET` 长度建议不少于 32 字节，可使用 `openssl rand -hex 32` 生成）：

```ini
# 镜像仓库与版本标签
DOCKER_REGISTRY=openagentpay
IMAGE_TAG=latest

# PostgreSQL 数据库配置（生产模式下默认仅绑定本机 127.0.0.1:5432）
POSTGRES_DB=agentpay
POSTGRES_USER=agentpay
POSTGRES_PASSWORD=ReplaceWithStrongRandomDBPassword_2026!
POSTGRES_PORT=127.0.0.1:5432

# Go 网关配置
GATEWAY_PORT=8000
AGENTPAY_JWT_SECRET=ReplaceWith64CharHexHighEntropyJWTSecretKeyGeneratedByOpenSSL!
LOG_LEVEL=info

# Web 控制台与公开商店端口
CONSOLE_PORT=3000
MARKETPLACE_PORT=3001

# Marketplace 前端公开访问地址（请替换为实际域名或公网 IP）
NEXT_PUBLIC_GATEWAY_URL=https://gate.example.com
NEXT_PUBLIC_CONSOLE_URL=https://console.example.com
NEXT_PUBLIC_SITE_URL=https://market.example.com

# Facilitator 链上清算节点配置（Arbitrum Sepolia 测试网）
FACILITATOR_PORT=127.0.0.1:8001
RPC_URL=https://sepolia-rollup.arbitrum.io/rpc
CHAIN_ID=421614
USDC_ADDRESS=0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d
FACILITATOR_PRIVATE_KEY=
```

::: tip 关于 `FACILITATOR_PRIVATE_KEY`
- 如果你仅使用 **API Key 虚拟钱包路径**（Path A），或仅在本地 Mock 模式下验证签名，`FACILITATOR_PRIVATE_KEY` 可以留空。
- 如果你需要启用 **链上 x402（EIP-3009）真实测试网结算**（Path B），请填入一个在 Arbitrum Sepolia（或目标测试网）持有少量测试网 ETH 的代付钱包私钥。该钱包仅用于代付 L2 Gas，不托管买方或卖方的 USDC 资金。
:::

### 步骤 4：拉取镜像并启动

```bash
cd ~/agentpay
docker compose pull
docker compose up -d
docker compose ps
```

---

## 5. 环境变量与配置文件参考

### 5.1 `.env` 编排变量一览

| 变量名 | 默认值 | 作用范围 | 说明 |
| --- | --- | --- | --- |
| `DOCKER_REGISTRY` | `openagentpay` | 全局镜像 | Docker 镜像仓库前缀（可改为私有 Registry，如阿里云 ACR 或 GHCR） |
| `IMAGE_TAG` | `latest` | 全局镜像 | 镜像版本标签 |
| `POSTGRES_DB` | `agentpay` | `postgres` / `gateway` | PostgreSQL 数据库名称 |
| `POSTGRES_USER` | `agentpay` | `postgres` / `gateway` | PostgreSQL 数据库用户名 |
| `POSTGRES_PASSWORD` | `agentpay_strong_...` | `postgres` / `gateway` | PostgreSQL 密码，**部署时必须修改** |
| `POSTGRES_PORT` | `5432`（`prod` 为 `127.0.0.1:5432`） | `postgres` | 宿主机映射的 PostgreSQL 端口 |
| `GATEWAY_PORT` | `8000` | `gateway` | 宿主机映射的 Go 网关端口 |
| `AGENTPAY_JWT_SECRET` | 示例密钥 | `gateway` | 控制台登录与管理 API 的 JWT 签名密钥，**为空时网关将拒绝启动** |
| `LOG_LEVEL` | `info` | `gateway` / `facilitator` | 日志级别：`debug`、`info`、`warn`、`error` |
| `CONSOLE_PORT` | `3000` | `console` | Web 管理控制台宿主机端口 |
| `MARKETPLACE_PORT` | `3001` | `marketplace` | Marketplace 门户宿主机端口 |
| `NEXT_PUBLIC_GATEWAY_URL` | `http://localhost:8000` | `marketplace` | 商店页面展示给开发者的网关外部 URL |
| `NEXT_PUBLIC_CONSOLE_URL` | `http://localhost:3000` | `marketplace` | 商店页面跳转至管理控制台的外部 URL |
| `NEXT_PUBLIC_SITE_URL` | `http://localhost:3001` | `marketplace` | 商店站点自身的公开 URL（用于生成 SEO / `/llms.txt` 链接） |
| `FACILITATOR_PORT` | `8001` | `facilitator` | Facilitator 服务宿主机端口 |
| `RPC_URL` | `https://sepolia-rollup.arbitrum.io/rpc` | `facilitator` | EVM 区块链 JSON-RPC 节点地址 |
| `CHAIN_ID` | `421614` | `facilitator` | 目标 EVM 链 ID（Arbitrum Sepolia 为 `421614`，Base Sepolia 为 `84532`） |
| `USDC_ADDRESS` | `0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d` | `facilitator` | 目标链上的 Circle USDC 代币合约地址 |
| `FACILITATOR_PRIVATE_KEY` | 空 | `facilitator` | Facilitator 广播 `transferWithAuthorization` 交易的代付 Gas 私钥 |

### 5.2 Go 网关环境变量覆盖规则

`agentpay-gateway` 启动时会先读取 `-config /etc/agentpay/gateway.yaml`，随后用以下环境变量覆盖同名配置项（环境变量优先级高于 YAML 文件）：

| 网关容器环境变量 | 覆盖的 `gateway.yaml` 字段 | 说明 |
| --- | --- | --- |
| `AGENTPAY_HOST` | `server.host` | 监听地址，容器内固定为 `0.0.0.0` |
| `AGENTPAY_PORT` | `server.port` | 监听端口，容器内默认为 `8000` |
| `AGENTPAY_POSTGRES_URL` | `database.postgres_url` | PostgreSQL 连接串；若数据库不可达，网关会打印警告并回退至内存模式，**生产部署请确保数据库健康连接** |
| `AGENTPAY_FACILITATOR_URL` | `facilitator.url` | Facilitator 节点地址（在 Compose 内网中建议设为 `http://facilitator:8001`） |
| `AGENTPAY_JWT_SECRET` | `auth.jwt_secret` | 管理端 JWT 签名密钥 |
| `AGENTPAY_LOG_LEVEL` | `logger.level` | `debug` / `info` / `warn` / `error` |
| `AGENTPAY_LOG_FORMAT` | `logger.format` | `json`（推荐用于日志采集）或 `text` |

::: details 查看完整的 `gateway-go/configs/gateway.yaml` 生产配置模板
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
  default_pay_to: "0x70997970C51812dc3A010C7d01b50e0d17dc79C8" # 默认卖方收款地址

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
更多路由与策略字段说明请参阅[网关配置指南](/zh/guide/gateway)与[策略配置指南](/zh/guide/policy)。
:::

---

## 6. 构建与推送自定义镜像

如果你对网关、控制台或商店代码进行了二次开发，可以使用仓库内置的 `scripts/docker-build-push.sh` 脚本打包镜像并推送到自己的镜像仓库（如 Docker Hub、GitHub Container Registry 或阿里云容器镜像服务 ACR）。

### 本地构建所有镜像

```bash
# 使用默认前缀 openagentpay 与标签 latest 构建四个服务镜像
make docker-build

# 或指定自定义版本标签
./scripts/docker-build-push.sh --tag v0.1.2
```

### 跨平台构建并推送至远程镜像仓库

当开发机为 Apple Silicon（`arm64`）而云服务器为 `x86_64`（`amd64`）时，可通过 `--platform` 参数启用 `docker buildx` 构建多架构镜像并直接推送：

```bash
# 1. 登录目标镜像仓库
docker login

# 2. 构建 linux/amd64 与 linux/arm64 双架构镜像并推送
./scripts/docker-build-push.sh \
  --push \
  --registry registry.example.com/myorg \
  --tag v0.1.2 \
  --platform linux/amd64,linux/arm64
```

构建脚本会自动将当前 Git Commit、构建时间与版本号通过 `-ldflags` 注入 `gateway` 二进制文件中，可通过 `/version` 端点随时核验线上运行版本。

---

## 7. 部署后验证清单

容器全部启动后，请按顺序执行以下检查，确认数据面、控制面与前端门户均工作正常（以下假设在服务器本机执行；若从外部机器测试，请将 `127.0.0.1` 替换为服务器公网 IP 或域名）：

### 1. 验证网关存活、就绪与版本元数据

```bash
curl -s http://127.0.0.1:8000/healthz
curl -s http://127.0.0.1:8000/readyz
curl -s http://127.0.0.1:8000/version
```

- `/healthz` 返回 `200 OK` 表示网关进程存活。
- `/readyz` 返回 `200 OK` 表示 RCU 配置快照已成功加载，可接收代理流量。
- `/version` 返回当前运行的镜像版本号、Git Commit 与编译时间。

### 2. 验证日志中的 PostgreSQL 持久化模式

确认网关已成功连接 PostgreSQL，而非回退到内存模式：

```bash
docker compose logs gateway | grep -i "persistence mode"
```

预期应看到类似输出：

```text
AgentPay Control Plane operating in PostgreSQL persistence mode
AgentPay Usage & Event Recording operating in PostgreSQL persistence mode
AgentPay Payment Correlation operating in PostgreSQL persistence mode
AgentPay Agent Behavior Timeline operating in PostgreSQL persistence mode
```

### 3. 验证 Facilitator 清算节点健康状态

```bash
curl -s http://127.0.0.1:8001/health
```

### 4. 验证 `402 Payment Required` 付费路由拦截

不带支付凭证请求示例路由 `/v1/market-data`：

```bash
curl -i http://127.0.0.1:8000/v1/market-data
```

应返回 `HTTP/1.1 402 Payment Required`，且包含 `PAYMENT-REQUIRED` 挑战响应头与 `X-Config-Version` 配置版本头。

### 5. 验证 Web 管理控制台与 Marketplace 门户

- **管理控制台**：浏览器访问 `http://<服务器IP>:3000/`，确认登录/注册界面正常加载，且 `/healthz` 代理正常。
- **Marketplace 门户**：浏览器访问 `http://<服务器IP>:3001/` 与 `http://<服务器IP>:3001/llms.txt`，确认商店页面与 AI 索引文档可正常读取。

---

## 8. 域名反向代理与 HTTPS 配置

在公网环境中，切勿直接通过明文 HTTP 传输 API Key（`apk_live_...`）或管理端 JWT。推荐在宿主机前置 **Caddy** 或 **Nginx**（或使用云厂商 ALB / Cloudflare），将不同子域名分别反代到对应容器端口并强制启用 HTTPS（TLS 1.2/1.3）。

### 推荐域名规划

| 域名示例 | 目标容器地址 | 用途 |
| --- | --- | --- |
| `gate.example.com` | `http://127.0.0.1:8000` | 网关数据面（AI Agent 调用入口）与公开 API |
| `console.example.com` | `http://127.0.0.1:3000` | 企业多租户 Web 管理控制台 |
| `market.example.com` | `http://127.0.0.1:3001` | 公开 API 商店与 `/llms.txt` 门户 |

### Nginx 反向代理配置示例

```nginx
# 1. AgentPay Gateway 数据面 (gate.example.com)
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

        # 适配 LLM 推理等长耗时上游路由
        proxy_connect_timeout 15s;
        proxy_read_timeout 120s;
        proxy_send_timeout 60s;
    }
}

# 2. AgentPay Console 管理控制台 (console.example.com)
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

# 3. AgentPay Marketplace 门户 (market.example.com)
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

配置宿主机反向代理后，建议将 `.env` 中的端口改为仅绑定回环地址（如 `GATEWAY_PORT=127.0.0.1:8000`、`CONSOLE_PORT=127.0.0.1:3000`、`MARKETPLACE_PORT=127.0.0.1:3001`），避免外部绕过 HTTPS 直接访问容器端口。

---

## 9. 日常运维、升级与数据备份

### 常用运维命令

```bash
# 实时查看各组件日志
docker compose logs -f gateway
docker compose logs -f facilitator
docker compose logs -f console
docker compose logs -f postgres

# 抓取 Prometheus 监控指标
curl -s http://127.0.0.1:8000/metrics

# 平滑重启网关（触发 SIGTERM 优雅停机，等待进行中的请求完成）
docker compose restart gateway

# 拉取新版本镜像并滚动更新容器
docker compose pull
docker compose up -d

# 停止容器（数据库数据安全保留在 postgres_data 卷中）
docker compose down
```

::: danger 切勿随意执行 `docker compose down -v`
`-v` 参数会物理删除 `postgres_data` 持久化数据卷，导致所有租户、钱包账本、API Key 与审计流水永久丢失！仅在本地开发环境需要彻底重置为空库时才可使用。
:::

### PostgreSQL 数据库备份与恢复

虚拟钱包余额、双录账本（`ledger_entries`）与支付状态机均存储在 PostgreSQL 中，请定期执行备份：

```bash
# 导出全库一致性快照并压缩
docker exec agentpay-postgres pg_dump -U agentpay -d agentpay | gzip > agentpay_backup_$(date +%Y%m%d_%H%M%S).sql.gz

# 从备份文件恢复
gunzip -c agentpay_backup_20261024_120000.sql.gz | docker exec -i agentpay-postgres psql -U agentpay -d agentpay
```

---

## 10. 安全基线清单

在将环境开放给团队或外部联调前，请逐项核对以下安全要求：

- [ ] **修改默认凭据**：已将 `.env` 中的 `POSTGRES_PASSWORD` 与 `AGENTPAY_JWT_SECRET` 替换为随机高熵密钥，且 `.env` 文件权限已收紧（`chmod 600 .env`）。
- [ ] **数据库与清算节点网络隔离**：`5432`（PostgreSQL）与 `8001`（Facilitator）未暴露在公网安全组中，仅在 `agentpay-net` 内部网络或 `127.0.0.1` 可访问。
- [ ] **全链路启用 HTTPS**：所有公网流量均经过 TLS 加密，防止 `Authorization: Bearer apk_live_...` 与管理后台凭证被窃听。
- [ ] **买方私钥零上传**：买方 Agent 的签名私钥（`AGENT_PRIVATE_KEY`）仅保存在买方自己的运行环境与本地防火墙之后，切勿填入卖方网关或云端服务器。
- [ ] **Facilitator 热钱包限额**：`FACILITATOR_PRIVATE_KEY` 对应的链上地址仅存放满足日常 L2 Gas 消耗的少量测试网 ETH，不存放大额资产。
- [ ] **上游凭证托管**：上游微服务的内部鉴权密钥（如 OpenAI / 内部集群 Bearer Token）统一通过网关路由的 `upstream_headers` 或控制台上游鉴权模块注入，不对买方客户端暴露。

---

## 11. 常见故障排查

| 现象 | 可能原因 | 排查与解决方式 |
| --- | --- | --- |
| `gateway` 容器反复重启，日志报 `jwt secret key cannot be empty` | `auth.jwt_secret` 与 `AGENTPAY_JWT_SECRET` 均为空 | Go 网关遵循默认安全原则（fail-closed），拒绝使用空密钥启动。请在 `.env` 中设置 `AGENTPAY_JWT_SECRET` |
| 网关启动成功，但重启后在控制台创建的租户/路由消失 | 网关未能连接 PostgreSQL，自动回退到了 `in-memory mode` | 执行 `docker compose logs gateway` 检查数据库连接报错；确认 `AGENTPAY_POSTGRES_URL` 里的用户名、密码与 `postgres` 容器一致 |
| `postgres` 容器修改了 `.env` 中的密码后依然报密码错误 | `postgres_data` 数据卷在首次初始化时已固化旧密码，后续重启不会重新执行初始化 | 使用原密码登录并通过 `ALTER USER` 修改密码；若为全新测试环境无重要数据，可执行 `docker compose down -v && docker compose up -d` 重新初始化 |
| 链上路径支付报错，网关无法校验 EIP-712 签名 | `gateway` 容器无法访问 `facilitator`，或双方链配置不一致 | 检查 `gateway.yaml` 中 `facilitator.url`（在 Compose 中应设为 `http://facilitator:8001` 或注入环境变量 `AGENTPAY_FACILITATOR_URL=http://facilitator:8001`），并确认路由的 `network` / `asset` 与 `facilitator` 的 `CHAIN_ID` / `USDC_ADDRESS` 完全一致 |
| 请求付费路由返回 `502 Bad Gateway` | 网关容器无法连通路由配置的 `upstream` 地址 | 若上游服务运行在宿主机本机，容器内的 `127.0.0.1` 指向容器自身而非宿主机；请改为宿主机内网 IP 或 `http://host.docker.internal:<端口>` |
| 控制台页面可以打开，但登录或接口请求报 `502` / `504` | `console` 容器内的 Nginx 无法连通 `http://gateway:8000` | 执行 `docker compose ps` 与 `docker compose logs gateway` 确认 `gateway` 容器处于 `healthy` 状态，且两容器在同一 `agentpay-net` 网络中 |

---

## 下一步

| 我想… | 阅读 |
| --- | --- |
| 在本地 5 分钟跑通完整调用与熔断演示 | [5 分钟快速开始](/zh/guide/quickstart) |
| 配置网关路由、价格与上游鉴权 | [网关配置指南](/zh/guide/gateway) |
| 为 Agent 配置单笔上限、滚动预算与熔断器 | [策略配置指南](/zh/guide/policy) |
| 在可视化界面管理租户、Agent、钱包与发布快照 | [管理控制台](/zh/guide/console) |
| 了解双轨架构与安全边界设计 | [系统架构规范](/zh/specs/architecture) |
