.PHONY: test test-unit test-integration test-e2e vectors lint format format-check check clean demo

PYTHON ?= python3

# Run all test suites across the Monorepo
test:
	PYTHONPATH=python/packages/agentpay/src:python/packages/agentpay-gateway/src:python $(PYTHON) -m pytest -v python/tests

# Run unit tests only
test-unit:
	PYTHONPATH=python/packages/agentpay/src:python/packages/agentpay-gateway/src:python $(PYTHON) -m pytest -v python/tests/unit

# Run integration tests only
test-integration:
	PYTHONPATH=python/packages/agentpay/src:python/packages/agentpay-gateway/src:python $(PYTHON) -m pytest -v python/tests/integration

# Run e2e tests only
test-e2e:
	PYTHONPATH=python/packages/agentpay/src:python/packages/agentpay-gateway/src:python $(PYTHON) -m pytest -v python/tests/e2e

# Validate language-agnostic conformance test vectors
vectors:
	$(PYTHON) scripts/validate_vectors.py specs/test-vectors

# Check code linting with ruff
lint:
	$(PYTHON) -m ruff check python/

# Format code with ruff
format:
	$(PYTHON) -m ruff format python/

# Check code formatting without applying changes
format-check:
	$(PYTHON) -m ruff format --check python/

# Run complete verification: lint, format-check, test-vectors, and pytest
check: lint format-check vectors test

# Clean build artifacts and test caches
clean:
	find . -type d -name "__pycache__" -exec rm -rf {} +
	find . -type d -name ".pytest_cache" -exec rm -rf {} +
	find . -type d -name ".ruff_cache" -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete

# Runaway agent circuit breaker demo (Local simulation)
demo:
	PYTHONPATH=python/packages/agentpay/src:python/packages/agentpay-gateway/src:python $(PYTHON) examples/runaway-agent/demo.py

# Runaway agent circuit breaker demo (Live Arbitrum Sepolia testnet)
demo-live:
	PYTHONPATH=python/packages/agentpay/src:python/packages/agentpay-gateway/src:python $(PYTHON) examples/runaway-agent/demo.py --live
