## 📌 Pull Request Overview

<!-- Brief summary of the purpose of this PR and what changes it introduces. -->

### 🔗 Related Issues
<!-- Link the issue(s) resolved by this PR (e.g., Fixes #123, Closes #456) -->
- Fixes #

---

### 🛠️ Type of Change
- [ ] 🐛 Bug fix (non-breaking change which fixes an issue)
- [ ] ✨ New feature (non-breaking change which adds functionality)
- [ ] 💥 Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] 📝 Documentation update (no code changes)
- [ ] 🧪 Testing or conformance test vectors
- [ ] ⚡ Performance optimization or internal refactoring

---

### 🔍 Key Implementation Details
<!-- Highlight key components, architectural decisions, or non-obvious code changes: -->
- 

---

### 🔒 Financial Firewall & Security Invariants Checklist
<!-- AgentPay manages autonomous payments and financial keys. Please verify: -->
- [ ] **Fail-Closed Principle**: Any unexpected error or budget limit breach results in an immediate abort (zero funds spent).
- [ ] **Non-Custodial Integrity**: Private keys remain isolated in `LocalSigner` and are never exposed in logs or network payloads.
- [ ] **No Custom Crypto**: Cryptographic signing and hashing use `eth-account` and standard verified libraries only.
- [ ] **No Secret Leaks**: No private keys, RPC keys, or credentials are checked in or logged.

---

### 🧪 Verification & Testing
<!-- Please confirm all checks pass before submitting: -->
- [ ] Run `make lint` (Ruff linter passes with 0 errors)
- [ ] Run `make format-check` (Ruff formatting is compliant)
- [ ] Run `make vectors` (Language-agnostic conformance test vectors pass)
- [ ] Run `make test` (All unit, integration, and e2e tests pass)
- [ ] Or run `make check` (All of the above executed in one command)

---

### 📖 Documentation Checklist
- [ ] Documentation updated (`README.md`, `specs/`, or inline docstrings)
- [ ] Changes match [Conventional Commits](https://www.conventionalcommits.org/) standards (`feat(...)`, `fix(...)`, `docs(...)`)
