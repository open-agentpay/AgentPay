#!/usr/bin/env bash
# ==============================================================================
# AgentPay Docker Image Build & Push Script
# Usage:
#   ./scripts/docker-build-push.sh [--push] [--registry <registry>] [--tag <tag>] [--platform <platform>]
#
# Examples:
#   ./scripts/docker-build-push.sh
#   ./scripts/docker-build-push.sh --push --registry registry.cn-hangzhou.aliyuncs.com/myorg --tag v2.0.0
#   ./scripts/docker-build-push.sh --push --platform linux/amd64,linux/arm64
# ==============================================================================

set -euo pipefail

# Default configuration
REGISTRY="${DOCKER_REGISTRY:-openagentpay}"
TAG="${IMAGE_TAG:-latest}"
PUSH=false
PLATFORM=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --push)
      PUSH=true
      shift
      ;;
    --registry)
      REGISTRY="$2"
      shift 2
      ;;
    --tag)
      TAG="$2"
      shift 2
      ;;
    --platform)
      PLATFORM="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [OPTIONS]"
      echo "Options:"
      echo "  --push                Push images to registry after building"
      echo "  --registry <name>     Image registry prefix (default: agentpay)"
      echo "  --tag <tag>           Image tag (default: latest)"
      echo "  --platform <platform> Target platforms (e.g., linux/amd64 or linux/amd64,linux/arm64)"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

GATEWAY_IMAGE="${REGISTRY}/gateway:${TAG}"
FACILITATOR_IMAGE="${REGISTRY}/facilitator:${TAG}"
CONSOLE_IMAGE="${REGISTRY}/console:${TAG}"
MARKETPLACE_IMAGE="${REGISTRY}/marketplace:${TAG}"

echo "============================================================"
echo "AgentPay Docker Image Builder"
echo "Gateway Image:     ${GATEWAY_IMAGE}"
echo "Facilitator Image: ${FACILITATOR_IMAGE}"
echo "Console Image:     ${CONSOLE_IMAGE}"
echo "Marketplace Image: ${MARKETPLACE_IMAGE}"
echo "Push to Registry:  ${PUSH}"
if [[ -n "${PLATFORM}" ]]; then
  echo "Target Platform:   ${PLATFORM}"
fi
echo "============================================================"

# Ensure docker is available
if ! command -v docker &> /dev/null; then
  echo "Error: docker command not found." >&2
  exit 1
fi

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
COMMIT="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo "unknown")"

# 1. Build Gateway Go image
echo ""
echo ">>> Building Go Gateway image: ${GATEWAY_IMAGE} ..."
if [[ -n "${PLATFORM}" ]]; then
  BUILD_CMD=(docker buildx build --platform "${PLATFORM}")
  if [[ "${PUSH}" == "true" ]]; then
    BUILD_CMD+=(--push)
  else
    BUILD_CMD+=(--load)
  fi
else
  BUILD_CMD=(docker build)
fi

"${BUILD_CMD[@]}" \
  --build-arg VERSION="${TAG}" \
  --build-arg COMMIT="${COMMIT}" \
  --build-arg BUILD_DATE="${BUILD_DATE}" \
  -t "${GATEWAY_IMAGE}" \
  -f "${ROOT_DIR}/services/gateway/Dockerfile" \
  "${ROOT_DIR}/services/gateway"

# 2. Build Facilitator Go image
echo ""
echo ">>> Building Go Facilitator image: ${FACILITATOR_IMAGE} ..."
"${BUILD_CMD[@]}" \
  -t "${FACILITATOR_IMAGE}" \
  -f "${ROOT_DIR}/services/facilitator/Dockerfile" \
  "${ROOT_DIR}/services/facilitator"

# 3. Build Console Frontend image
echo ""
echo ">>> Building Console Frontend image: ${CONSOLE_IMAGE} ..."
"${BUILD_CMD[@]}" \
  -t "${CONSOLE_IMAGE}" \
  -f "${ROOT_DIR}/apps/console/Dockerfile" \
  "${ROOT_DIR}/apps/console"

# 4. Build Marketplace Web image
echo ""
echo ">>> Building Marketplace Web image: ${MARKETPLACE_IMAGE} ..."

# Ensure docs are built and bundled into apps/marketplace-web/public/docs
if [[ ! -d "${ROOT_DIR}/apps/marketplace-web/public/docs" || ! -f "${ROOT_DIR}/apps/marketplace-web/public/docs/index.html" ]]; then
  echo ">>> Pre-bundling documentation into apps/marketplace-web/public/docs ..."
  node "${ROOT_DIR}/scripts/sync-specs.mjs"
  DOCS_BASE=/docs/ npx --yes vitepress build "${ROOT_DIR}/docs"
  rm -rf "${ROOT_DIR}/apps/marketplace-web/public/docs"
  cp -r "${ROOT_DIR}/docs/.vitepress/dist" "${ROOT_DIR}/apps/marketplace-web/public/docs"
fi

"${BUILD_CMD[@]}" \
  -t "${MARKETPLACE_IMAGE}" \
  -f "${ROOT_DIR}/apps/marketplace-web/Dockerfile" \
  "${ROOT_DIR}/apps/marketplace-web"

# 5. Push images if requested and not already pushed via buildx
if [[ "${PUSH}" == "true" && -z "${PLATFORM}" ]]; then
  echo ""
  echo ">>> Pushing images to registry..."
  docker push "${GATEWAY_IMAGE}"
  docker push "${FACILITATOR_IMAGE}"
  docker push "${CONSOLE_IMAGE}"
  docker push "${MARKETPLACE_IMAGE}"
fi

echo ""
echo "============================================================"
echo "Build finished successfully!"
echo "  - Gateway:     ${GATEWAY_IMAGE}"
echo "  - Facilitator: ${FACILITATOR_IMAGE}"
echo "  - Console:     ${CONSOLE_IMAGE}"
echo "  - Marketplace: ${MARKETPLACE_IMAGE}"
echo "============================================================"
