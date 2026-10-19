# Good First Issues for AgentPay Contributors

Welcome to AgentPay! We have compiled a list of **curated, high-impact, beginner-friendly tasks** designed for developers making their first contribution to the project.

Each task is scoped to be achievable within a few hours to a weekend, with clearly defined goals, relevant file pointers, and acceptance criteria.

---

## 🧭 How to Pick a Task

1. Read through the tasks below and choose one that matches your interests (CLI tools, database queries, networking, or observability).
2. Check the [AgentPay Issues](https://github.com/open-agentpay/AgentPay/issues) on GitHub to ensure no one has claimed it yet.
3. Leave a comment on the issue saying: *"I would like to work on this!"* to get assigned.
4. Review our [Contributing Guide](https://github.com/open-agentpay/AgentPay/blob/main/CONTRIBUTING.md) for development environment setup and make sure `make check` passes locally.

---

## 📌 Issue #1: Policy Config CLI Validator (`agentpay policy validate`)

- **Component**: `python/packages/agentpay`
- **Difficulty**: ⭐ Easy
- **Tags**: `good first issue`, `cli`, `developer-experience`

### Background
Users and CI/CD deployment pipelines configure AgentPay's financial firewall via YAML files (specifying `max_amount_per_tx`, `rolling_budget_24h`, whitelisted payees, and circuit breaker limits). Currently, if a configuration has a syntax typo or invalid Ethereum address, the error is only caught when initializing `SQLitePolicyEngine` at runtime.

### Objective
Create a CLI command:
```bash
python -m agentpay.cli policy validate path/to/policy.yaml
```
that loads and validates the YAML against `PolicyConfig`, reporting clear human-readable errors without needing to initialize the full SQLite database.

### Implementation Guide
1. **File Location**: `python/packages/agentpay/src/agentpay/cli.py` (create or extend).
2. **Schema**: Use `PolicyConfig.model_validate(...)` (Pydantic v2).
3. **Exit Codes**:
   - Exit `0`: Configuration is valid. Print a clean summary table of active limits.
   - Exit `1`: Configuration is invalid. Print line numbers / fields and validation reasons.
4. **Tests**: Add unit tests under `python/tests/unit/test_cli.py` testing valid YAML, malformed YAML, and schema violation cases.

---

## 📌 Issue #2: SQLite Ledger Audit Exporter (`agentpay audit export`)

- **Component**: `python/packages/agentpay`
- **Difficulty**: ⭐ Easy
- **Tags**: `good first issue`, `sqlite`, `accounting`

### Background
`SQLitePolicyEngine` maintains an append-only audit trail in `policy_ledger.db`, tracking every payment reservation, settlement, and failure. Developers and finance teams need a quick command to export this audit trail into structured JSON or CSV for external accounting and reconciliation.

### Objective
Implement an export utility callable programmatically and via CLI:
```bash
python -m agentpay.cli audit export --db ./policy_ledger.db --format json --output audit.json
python -m agentpay.cli audit export --db ./policy_ledger.db --format csv --output audit.csv
```

### Implementation Guide
1. **Query**: Select records from `transactions` and `reservations` in `SQLitePolicyEngine`.
2. **Fields to Export**: `intent_id`, `tx_hash`, `amount`, `asset`, `payee`, `network`, `status`, `timestamp`, `created_at`.
3. **Masking**: Ensure no private keys or internal memory references are included.
4. **Tests**: Add integration tests under `python/tests/unit/test_audit_export.py` verifying that records inserted into a temporary SQLite DB export accurately in both JSON and CSV formats.

---

## 📌 Issue #3: Gateway Dynamic Route Reload on SIGHUP

- **Component**: `python/packages/agentpay-gateway`
- **Difficulty**: ⭐⭐ Medium
- **Tags**: `good first issue`, `gateway`, `devops`

### Background
When deploying `agentpay-gateway` in production or staging, operators may update prices, add new API routes, or change upstream targets in `gateway.yaml`. Currently, updating routes requires restarting the gateway process, which interrupts active client connections.

### Objective
On POSIX platforms (Linux / macOS), allow the gateway to reload `gateway.yaml` on receipt of a `SIGHUP` signal without terminating the running server.

### Implementation Guide
1. **File Location**: `python/packages/agentpay-gateway/src/agentpay_gateway/server.py`.
2. **Signal Handling**: Use Python's standard `signal.signal(signal.SIGHUP, handler)` or Starlette lifespan hooks.
3. **Thread Safety**: Safely reload and swap the active `GatewayConfig.routes` mapping under a lock.
4. **Logging**: Log at `INFO` level: `"Reloaded gateway configuration: X active routes"`.
5. **Tests**: Add a test under `python/tests/integration/test_gateway_reload.py` verifying that modifying the YAML file and sending SIGHUP updates the routes served by the test client.

---

## 📌 Issue #4: Standard CAIP-2 Network Registry & Block Explorer Helpers

- **Component**: `python/packages/agentpay`
- **Difficulty**: ⭐ Easy
- **Tags**: `good first issue`, `web3`, `utility`

### Background
AgentPay adheres strictly to the [CAIP-2](https://github.com/ChainAgnostic/CAIPs/blob/master/CAIPs/caip-2.md) standard (e.g., `eip155:421614` for Arbitrum Sepolia). When inspecting logs or debugging payment disputes, developers frequently want to format explorer links (e.g., `https://sepolia.arbiscan.io/tx/0x...`) and human-readable network names.

### Objective
Provide a clean helper module `agentpay.core.networks` with:
- Registry of standard Layer-2 testnets and mainnets (Arbitrum One, Arbitrum Sepolia, Base, Base Sepolia, Optimism, Polygon).
- Helper function `get_network_name(caip2_id: str) -> str`.
- Helper function `get_explorer_tx_url(caip2_id: str, tx_hash: str) -> str | None`.
- Helper function `get_explorer_address_url(caip2_id: str, address: str) -> str | None`.

### Implementation Guide
1. **File Location**: `python/packages/agentpay/src/agentpay/core/networks.py`.
2. **Export**: Export public functions in `agentpay` top-level `__init__.py`.
3. **Tests**: Add unit tests in `python/tests/unit/test_networks.py` verifying correct URL formatting and graceful handling of unknown CAIP-2 networks.

---

## 📌 Issue #5: Gateway Prometheus-Compatible `/metrics` Endpoint

- **Component**: `python/packages/agentpay-gateway`
- **Difficulty**: ⭐⭐ Medium
- **Tags**: `good first issue`, `observability`, `metrics`

### Background
Platform teams running the AgentPay gateway need visibility into challenge rates, payment authorizations, and upstream response times.

### Objective
Expose a `/metrics` HTTP endpoint on the gateway serving Prometheus text-based exposition metrics:
- `agentpay_gateway_requests_total{route, status}` (Counter)
- `agentpay_gateway_challenges_issued_total{route}` (Counter)
- `agentpay_gateway_payments_verified_total{route}` (Counter)
- `agentpay_gateway_upstream_duration_seconds` (Histogram/Summary)

### Implementation Guide
1. **File Location**: `python/packages/agentpay-gateway/src/agentpay_gateway/server.py`.
2. **Zero Bloat**: Use a lightweight in-memory metric counter class or optional integration so external dependencies remain minimal.
3. **Tests**: Add integration tests verifying `GET /metrics` returns status `200` with `text/plain; version=0.0.4` and accurately increments after sample requests.

---

## 💬 Need Assistance?

If you have any questions or get stuck while implementing a task:
- Join the discussion in [GitHub Discussions](https://github.com/open-agentpay/AgentPay/discussions).
- Ask in the comment thread of your assigned issue.
- Maintainers are happy to help you with architecture questions, test setup, or code reviews!
