.PHONY: test test-unit test-integration test-e2e vectors lint format format-check check clean demo pypi-build pypi-upload

PYTHON ?= python3

# Run all test suites across the Monorepo
test:
	PYTHONPATH=sdk/python/packages/agentpay/src:sdk/python/packages/agentpay-gateway/src:sdk/python $(PYTHON) -m pytest -v sdk/python/tests

# Run unit tests only
test-unit:
	PYTHONPATH=sdk/python/packages/agentpay/src:sdk/python/packages/agentpay-gateway/src:sdk/python $(PYTHON) -m pytest -v sdk/python/tests/unit

# Run integration tests only
test-integration:
	PYTHONPATH=sdk/python/packages/agentpay/src:sdk/python/packages/agentpay-gateway/src:sdk/python $(PYTHON) -m pytest -v sdk/python/tests/integration

# Run e2e tests only
test-e2e:
	PYTHONPATH=sdk/python/packages/agentpay/src:sdk/python/packages/agentpay-gateway/src:sdk/python $(PYTHON) -m pytest -v sdk/python/tests/e2e

# Validate language-agnostic conformance test vectors
vectors:
	$(PYTHON) scripts/validate_vectors.py specs/test-vectors

# Check code linting with ruff
lint:
	$(PYTHON) -m ruff check sdk/python/

# Format code with ruff
format:
	$(PYTHON) -m ruff format sdk/python/

# Check code formatting without applying changes
format-check:
	$(PYTHON) -m ruff format --check sdk/python/

# Go Gateway targets
gateway-build:
	$(MAKE) -C services/gateway build

gateway-test:
	$(MAKE) -C services/gateway test

gateway-clean:
	$(MAKE) -C services/gateway clean

# Go Facilitator targets
facilitator-build:
	cd services/facilitator && go build -o bin/facilitator ./cmd/facilitator

facilitator-test:
	cd services/facilitator && go test -v ./...

facilitator-clean:
	rm -rf services/facilitator/bin

# Console targets (Vue 3 + Element Plus)
console-dev:
	pnpm --prefix apps/console run dev

console-build:
	pnpm --prefix apps/console run build

console-test:
	pnpm --prefix apps/console run test

# Marketplace targets (Next.js 15)
marketplace-dev:
	pnpm --prefix apps/marketplace-web run dev

marketplace-build:
	@echo "Building and bundling docs into apps/marketplace-web/public/docs..."
	node scripts/sync-specs.mjs && DOCS_BASE=/docs/ npx vitepress build docs && rm -rf apps/marketplace-web/public/docs && cp -r docs/.vitepress/dist apps/marketplace-web/public/docs
	pnpm --prefix apps/marketplace-web run build

marketplace-lint:
	pnpm --prefix apps/marketplace-web run lint

# Docker build & Compose targets
docker-build:
	./scripts/docker-build-push.sh

docker-push:
	./scripts/docker-build-push.sh --push

# Build all PyPI distribution packages (.whl & .tar.gz)
pypi-build:
	$(PYTHON) scripts/build_pypi_dist.py

# Publish all built packages to PyPI via twine
pypi-upload:
	twine upload dist_all/*

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

# Run complete verification: lint, format-check, test-vectors, pytest, and gateway-test
check: lint format-check vectors test gateway-test

# Clean build artifacts and test caches
clean: gateway-clean
	find . -type d -name "__pycache__" -exec rm -rf {} +
	find . -type d -name ".pytest_cache" -exec rm -rf {} +
	find . -type d -name ".ruff_cache" -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete

# Runaway agent circuit breaker demo (Local simulation)
demo:
	PYTHONPATH=sdk/python/packages/agentpay/src:sdk/python/packages/agentpay-gateway/src:sdk/python $(PYTHON) examples/runaway-agent/demo.py

# Runaway agent circuit breaker demo (Live Arbitrum Sepolia testnet)
demo-live:
	PYTHONPATH=sdk/python/packages/agentpay/src:sdk/python/packages/agentpay-gateway/src:sdk/python $(PYTHON) examples/runaway-agent/demo.py --live
