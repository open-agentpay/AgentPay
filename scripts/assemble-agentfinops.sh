#!/usr/bin/env bash
# ==============================================================================
# AgentFinOps Assembler & Compliance Sanitizer
# Assembles the domestic pure edition of AgentFinOps at:
#   /Users/wanghongjie/aiPoiject/AgentFinOps
# ==============================================================================

set -euo pipefail

SOURCE_DIR="/Users/wanghongjie/aiPoiject/AgentPay"
TARGET_DIR="/Users/wanghongjie/aiPoiject/AgentFinOps"

echo "============================================================"
echo "🚀 开始组装国内合规纯净版: AgentFinOps"
echo "目标路径: ${TARGET_DIR}"
echo "============================================================"

# 1. 确保目标目录存在
mkdir -p "${TARGET_DIR}"

# 2. 纯净代码同步（排除缓存、构建产物、原 Git 历史与链上模块）
echo "📦 [1/6] 同步核心服务 (Gateway, Console, Infra, 基建配置)..."
rsync -av --progress "${SOURCE_DIR}/" "${TARGET_DIR}/" \
  --exclude ".git" \
  --exclude "node_modules" \
  --exclude ".next" \
  --exclude ".venv" \
  --exclude ".ruff_cache" \
  --exclude "dist_all" \
  --exclude ".tmp-work" \
  --exclude ".DS_Store" \
  --exclude "services/facilitator" \
  --exclude "specs" \
  --exclude "docs" \
  --exclude "sdk"

cd "${TARGET_DIR}"

# 3. 彻底删除任何残留的 Web3/Facilitator 文件
echo "🧹 [2/6] 物理剔除链上相关模块..."
rm -rf services/facilitator 2>/dev/null || true
rm -rf infra/sqlite 2>/dev/null || true
rm -f scripts/publish_crates.py 2>/dev/null || true
rm -f scripts/bump_python_sdk_version.sh 2>/dev/null || true
rm -f scripts/validate_vectors.py 2>/dev/null || true

# 4. 净化与重命名项目文件 (package.json, Makefile, .env.example)
echo "📝 [3/6] 净化构建与配置文件..."
if [ -f package.json ]; then
  sed -i '' 's/"name": "open-agentpay"/"name": "agentfinops"/g' package.json
  sed -i '' 's/"name": "agentpay"/"name": "agentfinops"/g' package.json
  sed -i '' '/facilitator/d' package.json
fi

# 净化 Makefile
if [ -f Makefile ]; then
  sed -i '' '/facilitator/d' Makefile
fi

# 净化 .env.example
if [ -f .env.example ]; then
  sed -i '' '/FACILITATOR/d' .env.example
  sed -i '' '/RPC_URL/d' .env.example
  sed -i '' '/CHAIN_ID/d' .env.example
  sed -i '' '/USDC_ADDRESS/d' .env.example
  sed -i '' 's/openagentpay/agentfinops/g' .env.example
  sed -i '' 's/agentpay/agentfinops/g' .env.example
fi

# 净化 docker-compose 文件
for f in docker-compose*.yml; do
  if [ -f "$f" ]; then
    sed -i '' '/facilitator:/,/networks:/d' "$f" 2>/dev/null || true
    sed -i '' 's/openagentpay/agentfinops/g' "$f"
    sed -i '' 's/agentpay-/agentfinops-/g' "$f"
    sed -i '' 's/agentpay/agentfinops/g' "$f"
  fi
done

# 净化网关配置文件 gateway.yaml
for cfg in services/gateway/configs/*.yaml; do
  if [ -f "$cfg" ]; then
    sed -i '' '/facilitator:/,/^[^ ]/d' "$cfg" 2>/dev/null || true
    sed -i '' 's/USDC/CNY/g' "$cfg"
    sed -i '' 's/agentpay/agentfinops/g' "$cfg"
  fi
done

# 5. 前端控制台 (apps/console) 去代币化文案清洗
echo "🎨 [4/6] 前端控制台 (Console) 去代币化与中文本土化清洗..."
find apps/console/src -type f \( -name "*.vue" -o -name "*.ts" -o -name "*.js" \) -exec sed -i '' 's/USDC/算力点数 (Credits)/g' {} +
find apps/console/src -type f \( -name "*.vue" -o -name "*.ts" -o -name "*.js" \) -exec sed -i '' 's/usdc/credits/g' {} +
find apps/console/src -type f \( -name "*.vue" -o -name "*.ts" -o -name "*.js" \) -exec sed -i '' 's/钱包地址/租户组织编码/g' {} +
find apps/console/src -type f \( -name "*.vue" -o -name "*.ts" -o -name "*.js" \) -exec sed -i '' 's/钱包/组织账户/g' {} +
find apps/console/src -type f \( -name "*.vue" -o -name "*.ts" -o -name "*.js" \) -exec sed -i '' 's/代币/算力额度/g' {} +
find apps/console/src -type f \( -name "*.vue" -o -name "*.ts" -o -name "*.js" \) -exec sed -i '' 's/链上结算/用量账单/g' {} +
find apps/console/src -type f \( -name "*.vue" -o -name "*.ts" -o -name "*.js" \) -exec sed -i '' 's/交易哈希/审计流水号/g' {} +

# 6. 生成国内版专属全中文文档站 docs/
echo "📚 [5/6] 生成全新的国内版纯净文档 docs/..."
mkdir -p docs/guide docs/finops docs/deployment

cat << 'EOF' > docs/index.md
# AgentFinOps 官方开发与运营文档 🚀

欢迎使用 **AgentFinOps** —— 面向企业级大模型与 AI Agent 时代打造的统一路由网关与财务成本治理（FinOps）平台。

---

## 核心价值

- ⚡ **统一智能网关**：基于 Go 语言构建的极速反向代理，毫秒级流式转发，完全兼容 OpenAI / DeepSeek / 通义千问等协议规范。
- 📊 **毫秒级 Token 计量**：精准统计单次会话及全生命周期的 Prompt / Completion Token 消耗与核算。
- 🏢 **多租户与组织预算**：支持企业多部门、多项目组隔离，提供部门预算配额硬封顶与欠费自动熔断。
- 🔀 **模型智能降级路由**：根据成本、延迟和可用性，智能路由不同复杂度的 Agent 任务，显著降低算力采购成本。
- 🔒 **企业级合规与审计**：全链路请求审计流水，核心上游 Master API Key 集中加密管控，绝不落地泄露。

---

## 快速导航
- [快速开始与安装部署](deployment/quickstart.md)
- [FinOps 成本治理与预算控制](finops/budget-control.md)
- [OpenAI 兼容协议接入指南](guide/api-reference.md)
EOF

cat << 'EOF' > docs/deployment/quickstart.md
# 快速开始与部署指南

AgentFinOps 采用 Docker 容器化编排，内置 Caddy 边缘反向代理，支持自动申请 HTTPS 证书与 HTTP/3 极速访问。

## 生产环境一键启动

```bash
# 1. 复制环境变量模板
cp .env.example .env

# 2. 一键启动服务
docker compose -f docker-compose.nginx.yml up -d
```

## 服务端口规划
- `80 / 443`：Caddy 统一公网流量入口
- 内部网络：Gateway (8000), Console (80), Postgres (5432)
EOF

cat << 'EOF' > docs/finops/budget-control.md
# FinOps 成本治理与预算控制

## 1. 预算配额模型 (Budget Quota Model)
AgentFinOps 摒弃了传统的粗放式充值消费，采用现代云原生 FinOps 治理体系：
- **事前预算评估 (Pre-flight Authorization)**：Agent 发起请求时，预估 Token 上限并原子保留配额；
- **流式实时计量 (Real-time Token Metering)**：解析 LLM 响应内容与使用量指标；
- **事后对账归集 (Settlement Reconciliation)**：按真实 Token 消耗确认扣除，多余保留即刻释放。

## 2. 部门熔断机制 (Circuit Breaker)
当某个智能体由于死循环或异常递归导致单日调用超出预算时，网关将毫秒级熔断阻断，保护企业账户不被穿透。
EOF

# 7. 生成正式的国内版 README.md
echo "📄 [6/6] 生成全新的企业级 README.md..."
cat << 'EOF' > README.md
# AgentFinOps 🚀
### 企业级 AI Agent 统一网关与大模型算力成本治理平台 (AI Agent Gateway & FinOps)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)

> **AgentFinOps** 是专为大模型与 AI Agent 时代打造的企业级统一路由网关与财务成本治理（FinOps）平台。
> 提供毫秒级 Token 计量、多模型智能降级路由、部门/租户预算硬封顶与精细化财务核算，助力企业实现大模型与智能体应用的降本增效与合规治理。

---

## 🌟 核心特性

- ⚡ **AI Agent 高性能网关**：基于 Go 语言构建的极速反向代理网关，支持上万并发，统一接入 OpenAI、DeepSeek、通义千问等主流大模型。
- 📊 **毫秒级 Token 计量 (FinOps)**：实时统计 Prompt / Completion Token 消耗与成本，精确到单次请求和会话。
- 🏢 **多租户与组织配额治理**：支持企业多部门、多项目组租户隔离，提供部门预算配额管控与欠费自动熔断。
- 🔀 **智能降级与负载路由**：根据成本、延迟和可用性，智能路由不同难度的 Agent 任务（如优先分发至高性价比模型）。
- 🔒 **企业级安全与审计**：全链路请求审计日志，保护核心上游 Master API Key 不泄露，内置多级权限控制。
- 🇨🇳 **100% 国内合规开箱即用**：专为国内企业级私有化与云端部署设计，纯净合规，零海外不可控依赖。

---

## 🛠️ 快速开始

### 生产环境一键部署

```bash
# 1. 复制配置文件
cp .env.example .env

# 2. 一键启动网关与全部服务
docker compose -f docker-compose.nginx.yml up -d
```

---

## 📄 软著与合规信息
- **软件全称**：基于 AI Agent 的大模型算力成本精细化运营管理系统
- **软件简称**：AgentFinOps 智能网关系统
EOF

echo ""
echo "============================================================"
echo "🎉 AgentFinOps 纯净合规工程组装完毕！"
echo "仓库目录: ${TARGET_DIR}"
echo "============================================================"
