#!/usr/bin/env bash
# ==============================================================================
# AgentPay One-Click Gateway Deployment Script (Caddy Edge + Full Stack)
# Usage:
#   chmod +x scripts/deploy-gateway.sh
#   ./scripts/deploy-gateway.sh
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "============================================================"
echo "🚀 AgentPay One-Click Deployment (with Caddy Edge Gateway)"
echo "============================================================"

# 1. Check Docker & Docker Compose
if ! command -v docker &> /dev/null; then
  echo "❌ Error: Docker is not installed on this server."
  echo "👉 Run: curl -fsSL https://get.docker.com | bash"
  exit 1
fi

if ! docker compose version &> /dev/null; then
  echo "❌ Error: docker compose plugin is not found."
  exit 1
fi

# 2. Check and generate .env
cd "${ROOT_DIR}"

if [[ ! -f .env ]]; then
  echo "⚠️  .env file not found. Generating from .env.example..."
  cp .env.example .env

  # Generate secure random secrets
  POSTGRES_PASS=$(openssl rand -hex 16 2>/dev/null || date +%s%N | sha256sum | head -c 32)
  JWT_SECRET=$(openssl rand -base64 32 2>/dev/null || date +%s%N | sha256sum | head -c 44)
  MASTER_KEY=$(openssl rand -hex 32 2>/dev/null || date +%s%N | sha256sum | head -c 64)
  FACILITATOR_KEY=$(openssl rand -hex 16 2>/dev/null || date +%s%N | sha256sum | head -c 32)

  sed -i.bak "s|POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${POSTGRES_PASS}|" .env
  sed -i.bak "s|AGENTPAY_JWT_SECRET=.*|AGENTPAY_JWT_SECRET=${JWT_SECRET}|" .env
  sed -i.bak "s|AGENTPAY_UPSTREAM_MASTER_KEY=.*|AGENTPAY_UPSTREAM_MASTER_KEY=${MASTER_KEY}|" .env
  sed -i.bak "s|AGENTPAY_FACILITATOR_API_KEY=.*|AGENTPAY_FACILITATOR_API_KEY=${FACILITATOR_KEY}|" .env
  sed -i.bak "s|FACILITATOR_API_KEY=.*|FACILITATOR_API_KEY=${FACILITATOR_KEY}|" .env
  rm -f .env.bak
  echo "✅ Automatically generated production-ready secrets in .env"
fi

# 3. Pull / Launch containers
echo ""
echo "📦 Pulling latest pre-built container images..."
docker compose -f docker-compose.nginx.yml pull || true

echo ""
echo "🚀 Starting containers in detached mode..."
docker compose -f docker-compose.nginx.yml up -d --remove-orphans

echo ""
echo "============================================================"
echo "🎉 AgentPay Deployed Successfully!"
echo "============================================================"
docker compose -f docker-compose.nginx.yml ps

echo ""
echo "Useful Commands:"
echo "  - View Proxy Logs:   docker compose -f docker-compose.nginx.yml logs -f caddy"
echo "  - View All Logs:     docker compose -f docker-compose.nginx.yml logs -f"
echo "  - Restart Gateway:   docker compose -f docker-compose.nginx.yml restart caddy"
echo "  - Stop All:          docker compose -f docker-compose.nginx.yml down"
