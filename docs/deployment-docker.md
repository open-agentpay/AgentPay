# AgentPay Docker 容器化与云服务器部署指南

本文档介绍如何将 AgentPay **Golang 服务端 (`services/gateway`)**、**前端控制台 (`apps/console`)**、**功能商店与 GEO 门户 (`apps/marketplace-web`)** 以及 **PostgreSQL 15** 打包为 Docker 镜像，并通过 `docker-compose.yml` 快速部署到云服务器（如阿里云 ECS、腾讯云 CVM、AWS EC2、DigitalOcean 等）。

---

## 1. 部署架构拓扑

```
  [客户端浏览器 / 运营人员]    [开发者 / 探索者 / AI 搜索引擎]        [AI Agent / 开发者 API 请求]
             │                               │                                     │
             ▼ :3000                         ▼ :3001                               ▼ :8000
    ┌─────────────────┐             ┌─────────────────────┐               ┌─────────────────┐
    │ agentpay-console│             │agentpay-marketplace │               │ agentpay-gateway│
    │  (Vue3 + Nginx) │             │ (Next.js 15 Stand.) │ ── 内部查询 ─>│  (Golang Core)  │
    └────────┬────────┘             └─────────────────────┘               └────────┬────────┘
             │                                                                     │
             └─────────────────────── /api/ 代理 ──────────────────────────────────┘
                                                                                   │
                                                                                   ▼ :5432
                                                                          ┌─────────────────┐
                                                                          │agentpay-postgres│
                                                                          │ (PostgreSQL 15) │
                                                                          └─────────────────┘
```

- **`agentpay-console` (端口 3000 / 80)**：静态 Web 控制台，由 Nginx 托管；内置反向代理将 `/api/*` 请求透明转发给后端网关。
- **`agentpay-marketplace` (端口 3001)**：公开功能商店与 GEO (Generative Engine Optimization) 门户，基于 Next.js 15 独立轻量容器运行，提供 API 发现、GEO 页面与 `/llms.txt`。
- **`agentpay-gateway` (端口 8000)**：Golang 高性能网关核心服务，负责 x402 支付校验、RCU 动态路由、额度风控与 Admin API。
- **`agentpay-postgres` (端口 5432)**：持久化存储租户、项目、虚拟钱包、账本与审计日志，容器启动时自动执行 `schema.sql` 初始化。

---

## 2. 核心文件结构

```
AgentPay/
├── services/gateway/
│   ├── Dockerfile             # Golang 网关多阶段构建镜像文件
│   ├── .dockerignore          # 排除本地编译与临时文件
│   └── configs/gateway.yaml   # 网关运行参数配置
├── apps/console/
│   ├── Dockerfile             # 前端控制台多阶段构建镜像文件 (Node.js -> Nginx)
│   ├── .dockerignore          # 排除 node_modules 等
│   └── nginx.conf             # Nginx 反向代理与 SPA 路由配置
├── apps/marketplace-web/
│   ├── Dockerfile             # 功能商店 Next.js 15 Standalone 多阶段构建镜像文件
│   ├── .dockerignore          # 排除 node_modules、.next 等
│   └── next.config.mjs        # Next.js 独立输出构建配置
├── infra/postgres/
│   └── schema.sql             # PostgreSQL 初始化 DDL 表结构
├── scripts/
│   └── docker-build-push.sh   # 一键构建与推送镜像脚本
├── .env.example               # 生产环境变量模板
├── docker-compose.yml         # 完整部署（支持源码构建与镜像拉取）
└── docker-compose.prod.yml    # 云服务器轻量独立部署（仅拉取镜像）
```

---

## 3. 本地构建与推送 Docker 镜像

在开发机或 CI/CD 流水线（GitHub Actions、GitLab CI）中打包镜像并推送到镜像仓库：

### 3.1 本地快速构建
```bash
# 使用 Makefile 一键构建本地镜像（默认镜像名：openagentpay/gateway:latest, openagentpay/console:latest, openagentpay/marketplace:latest）
make docker-build

# 或直接运行脚本并指定版本 Tag
./scripts/docker-build-push.sh --tag v2.0.0
```

### 3.2 构建并推送到 Docker Hub (openagentpay)
```bash
# 登录 Docker Hub 账号
docker login

# 构建并推送镜像到 openagentpay 仓库（构建多架构兼容云服务器主流的 linux/amd64 与 ARM 架构）
./scripts/docker-build-push.sh \
  --push \
  --registry openagentpay \
  --tag latest \
  --platform linux/amd64,linux/arm64
```

---

## 4. 云服务器部署步骤

### 第一步：在云服务器上安装 Docker 与 Docker Compose
如果云服务器尚未安装 Docker：
```bash
# Ubuntu / Debian
curl -fsSL https://get.docker.com | bash
sudo systemctl enable --now docker

# 安装 Docker Compose 插件
sudo apt-get update && sudo apt-get install -y docker-compose-plugin
```

### 第二步：将部署文件传输至云服务器
只需复制部署所需的轻量文件，无需将所有源码复制到服务器：
- `docker-compose.yml`（或 `docker-compose.prod.yml`）
- `.env.example`
- `infra/postgres/schema.sql`
- `services/gateway/configs/gateway.yaml`

可使用 `rsync` 或 `scp`：
```bash
ssh user@your-server-ip "mkdir -p ~/agentpay/infra/postgres ~/agentpay/services/gateway/configs"

scp docker-compose.prod.yml user@your-server-ip:~/agentpay/docker-compose.yml
scp .env.example user@your-server-ip:~/agentpay/.env
scp infra/postgres/schema.sql user@your-server-ip:~/agentpay/infra/postgres/
scp services/gateway/configs/gateway.yaml user@your-server-ip:~/agentpay/services/gateway/configs/
```

### 第三步：配置环境变量 `.env`
在云服务器上的 `~/agentpay/` 目录：
```bash
cd ~/agentpay
cp .env.example .env
vim .env
```
重点修改以下生产环境配置：
```ini
# Docker Hub 镜像仓库与版本（默认即为 openagentpay）
DOCKER_REGISTRY=openagentpay
IMAGE_TAG=latest

# 生产环境数据库强密码
POSTGRES_DB=agentpay
POSTGRES_USER=agentpay
POSTGRES_PASSWORD=SetAStrongRandomPasswordHere_2026!

# JWT 生产签名密钥（必须 >= 32 个字符）
AGENTPAY_JWT_SECRET=production-secret-token-please-change-to-a-high-entropy-string-32b!

# 端口配置
CONSOLE_PORT=3000
MARKETPLACE_PORT=3001
GATEWAY_PORT=8000
```

### 第四步：一键启动服务
```bash
# 启动所有容器（后台运行）
docker compose up -d

# 查看各容器健康运行状态
docker compose ps
```
正常启动后输出如下：
```text
NAME                 IMAGE                          COMMAND                  SERVICE        STATUS                    PORTS
agentpay-console     openagentpay/console:latest     "/docker-entrypoint.…"   console        Up (healthy)              0.0.0.0:3000->80/tcp
agentpay-marketplace openagentpay/marketplace:latest "docker-entrypoint.s…"   marketplace    Up (healthy)              0.0.0.0:3001->3000/tcp
agentpay-gateway     openagentpay/gateway:latest     "/app/gateway -confi…"   gateway        Up (healthy)              0.0.0.0:8000->8000/tcp
agentpay-postgres    postgres:15-alpine             "docker-entrypoint.s…"   postgres       Up (healthy)              0.0.0.0:5432->5432/tcp
```

---

## 5. 验证与访问

1. **Web 控制台访问**：
   打开浏览器访问：`http://<云服务器公网IP>:3000/`
   - 可以点击“立即注册”创建首个租户与管理员账号；
   - 也可以通过界面上的角色切换体验各模块功能。

2. **功能商店与 GEO 门户访问**：
   打开浏览器访问：`http://<云服务器公网IP>:3001/` 或 `http://<云服务器公网IP>:3001/llms.txt`
   - 浏览已发布的 x402 API 商店并查看实时 SLA、代码示例与价格；
   - AI 爬虫抓取将自动识别并返回 GEO Markdown 格式。

2. **网关健康检查与版本接口**：
   ```bash
   curl http://<云服务器公网IP>:8000/healthz
   curl http://<云服务器公网IP>:8000/version
   ```

3. **测试网关代理流量 (x402 支付拦截)**：
   ```bash
   # 测试市场数据路由 (未携带支付头应返回 HTTP 402 Payment Required)
   curl -i http://<云服务器公网IP>:8000/v1/market-data
   ```

---

## 6. 常用运维与管理命令

```bash
# 查看实时日志
docker compose logs -f gateway
docker compose logs -f console
docker compose logs -f postgres

# 平滑重启网关服务
docker compose restart gateway

# 更新服务至新版本镜像
docker compose pull
docker compose up -d

# 停止并移除容器（数据保留在卷 postgres_data 中）
docker compose down

# 彻底清理（注意：这会删除数据库卷中存储的真实数据！）
docker compose down -v
```

---

## 7. 生产环境安全建议

1. **安全组 / 防火墙策略**：
   - 云服务器安全组仅需对公网开放：`80`（HTTP / Web 控制台）与 `8000`（网关 API）。
   - `5432` 端口（PostgreSQL）**切勿对 0.0.0.0/0 开放**，在 compose 中已默认绑定内部网络。
2. **HTTPS / SSL 证书**：
   - 生产环境建议在前端前置一个 Nginx/Caddy 或云厂商负载均衡器（ALB/CLB）并配置 SSL 证书，启用 HTTP/2 和 HTTPS。
