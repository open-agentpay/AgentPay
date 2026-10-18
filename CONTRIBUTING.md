# Contributing to AgentPay

Thank you for your interest in contributing to **AgentPay**! AgentPay is an open-source, non-custodial **Financial Firewall** and autonomous payment infrastructure for AI Agents based on HTTP 402 / x402 V2.

We welcome contributions from developers of all skill levels, from fixing typos in documentation to implementing core protocol features.

---

## Table of Contents

1. [Code of Conduct](#1-code-of-conduct)
2. [Repository Architecture](#2-repository-architecture)
3. [Development Setup](#3-development-setup)
4. [Development Workflow & Make Targets](#4-development-workflow--make-targets)
5. [Core Architectural Invariants](#5-core-architectural-invariants)
6. [Good First Issues](#6-good-first-issues)
7. [Git & Pull Request Guidelines](#7-git--pull-request-guidelines)
8. [Reporting Security Issues](#8-reporting-security-issues)

---

## 1. Code of Conduct

We are committed to providing a welcoming, inclusive, and harassment-free environment for all contributors. Please treat fellow maintainers and community members with mutual respect and professionalism.

---

## 2. Repository Architecture

The AgentPay repository is structured as a focused Python monorepo with language-agnostic protocol specifications:

```text
AgentPay/
├── specs/                          # Protocol specifications & conformance vectors (Single Source of Truth)
│   ├── x402-v2-specification.md   # HTTP 402 payment challenge/authorization protocol
│   ├── security-model.md          # Zero-trust threat model (T1-T7) & boundaries
│   ├── gateway-guide.md           # Self-hosted reverse proxy architecture
│   └── test-vectors/              # Language-agnostic JSON conformance test vectors
├── python/
│   ├── packages/
│   │   ├── agentpay/              # Core Client SDK & Financial Firewall
│   │   │   └── src/agentpay/
│   │   │       ├── client/        # httpx interceptor & AgentPayClient
│   │   │       ├── policy/        # SQLite budget engine, whitelist, circuit breaker
│   │   │       ├── signer/        # LocalSigner (EIP-712/3009 isolation)
│   │   │       ├── x402/          # V2 challenge/authorization codecs
│   │   │       └── integrations/  # LangChain tool wrappers
│   │   └── agentpay-gateway/      # Self-hosted reverse proxy (FastAPI/Starlette)
│   │       └── src/agentpay_gateway/
│   │           ├── server.py      # Reverse proxy app & YAML config loader
│   │           ├── challenge.py   # 402 challenge generation & EIP-712 verification
│   │           └── facilitator.py # Facilitator client (Mock & Arbitrum Sepolia)
│   └── tests/
│       ├── unit/                  # Unit tests (models, codecs, signer, policy)
│       ├── integration/           # Integration tests (SQLite engine, gateway proxy)
│       └── e2e/                   # End-to-end full payment & circuit breaker flows
├── examples/
│   └── runaway-agent/             # Runaway agent demo (Local simulation & Live testnet)
├── infra/
│   └── sqlite/schema.sql          # Canonical SQLite DDL for policy & ledger
└── scripts/                       # Packaging, vector validation, and utility scripts
```

---

## 3. Development Setup

### Prerequisites
- **Python**: `>= 3.10`
- **Git**
- Optional: [uv](https://github.com/astral-sh/uv) (recommended for rapid dependency management) or standard `venv`

### Setting Up Local Environment

```bash
# 1. Clone the repository
git clone https://github.com/open-agentpay/AgentPay.git
cd AgentPay

# 2. Create and activate a virtual environment
python3 -m venv .venv
source .venv/bin/activate  # On Windows: .venv\Scripts\activate

# 3. Install packages in editable mode along with development dependencies
pip install -e python/packages/agentpay -e python/packages/agentpay-gateway
pip install pytest pytest-asyncio ruff mypy eth-account httpx web3 langchain-core
```

---

## 4. Development Workflow & Make Targets

We use a standard `Makefile` to orchestrate testing, linting, formatting, and demos:

| Target | Description |
| :--- | :--- |
| `make check` | **Primary verification**: Runs lint, format check, conformance vectors, and all test suites. |
| `make test` | Runs all 340+ unit, integration, and e2e tests. |
| `make test-unit` | Runs fast in-memory unit tests only (`python/tests/unit`). |
| `make test-integration` | Runs integration tests with SQLite and FastAPI gateway (`python/tests/integration`). |
| `make test-e2e` | Runs end-to-end payment settlement and lifecycle tests (`python/tests/e2e`). |
| `make vectors` | Validates language-agnostic conformance test vectors in `specs/test-vectors/`. |
| `make lint` | Runs `ruff check python/` to catch linting errors. |
| `make format` | Runs `ruff format python/` to automatically format Python code. |
| `make format-check` | Checks code formatting without modifying files. |
| `make demo` | Runs the local runaway agent circuit breaker demonstration. |
| `make demo-live` | Runs the runaway agent demo against live Arbitrum Sepolia testnet. |
| `make clean` | Removes build artifacts, bytecode, and cache directories. |

> [!TIP]
> Always run `make check` before opening a pull request. All CI checks must pass.

---

## 5. Core Architectural Invariants

When contributing code to AgentPay, please adhere to our strict non-negotiable architectural principles:

1. **Zero-Trust Financial Firewall (Fail-Closed)**:
   - An autonomous LLM can propose payment transactions, but **never** approve or sign them directly.
   - Any policy violation, budget limit breach, or unexpected database/network error must trigger an **immediate abort** with **zero funds spent**.
2. **Non-Custodial Signer Isolation**:
   - Private keys reside strictly within `LocalSigner` memory in the local trust domain.
   - Private keys **must never** be logged, passed into HTTP headers, or exposed to the LLM agent context.
3. **Zero Custom Cryptography**:
   - **Never** implement custom cryptographic algorithms or hashing routines.
   - All cryptographic signing (EIP-712 typed data, EIP-3009 transfer authorizations, keccak256) must use audited libraries (e.g. `eth-account`).
4. **Three-State Decoupled State Machine**:
   - Intent Reservation (`RESERVED`), Chain Settlement (`SETTLED`), and Business Delivery (`DELIVERED`) are modeled separately.
   - Network timeouts do not equate to transaction failures; operations must be idempotent to prevent double-spending.

---

## 6. Good First Issues

If you are looking for a place to start contributing, check out our curated guide:
👉 [**Good First Issues Guide**](./docs/good-first-issues.md)

It contains several bite-sized, well-specified issues suitable for first-time contributors, covering CLI tools, audit exports, network helpers, and gateway metrics.

You can also filter GitHub issues by the [`good first issue`](https://github.com/open-agentpay/AgentPay/labels/good%20first%20issue) label.

---

## 7. Git & Pull Request Guidelines

### Branch Naming
Create feature branches from `main` using descriptive prefixes:
- `feat/add-cli-policy-validator`
- `fix/reconcile-timeout-handling`
- `docs/clarify-gasless-settlement`
- `spec/update-eip3009-vectors`
- `refactor/gateway-route-matching`

### Commit Message Standards
We follow [Conventional Commits](https://www.conventionalcommits.org/):
```text
<type>(<scope>): <subject>

[optional body]
```
Examples:
- `feat(gateway): add dynamic route reloading on SIGHUP`
- `fix(policy): prevent negative amount conversion in reservation token`
- `docs(specs): clarify CAIP-2 network identifier normalization`
- `test(signer): add test vector for EIP-712 domain separator hashing`

### Pull Request Lifecycle
1. **Open an Issue First** (for major features or protocol changes) to discuss architecture and design.
2. **Branch from `main`** and keep your commits atomic and well-documented.
3. **Verify Locally**:
   ```bash
   make check
   ```
4. **Submit PR**: Fill out the [Pull Request Template](.github/PULL_REQUEST_TEMPLATE.md), linking the relevant issue.
5. **Code Review**: A maintainer will review your code. Address any feedback and keep the conversation constructive.

---

## 8. Reporting Security Issues

Please **do not** report sensitive security vulnerabilities (such as private key exposure, policy engine bypasses, or double-spending exploits) via public GitHub issues.

Please refer to our [Security Policy](SECURITY.md) for instructions on confidential disclosure.
