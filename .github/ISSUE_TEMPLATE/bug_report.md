---
name: "Bug Report"
about: "Report a bug, unexpected error, or unintended behavior in AgentPay"
title: "[Bug]: "
labels: ["bug", "needs-triage"]
assignees: []
---

## 🐛 Bug Description
<!-- A clear and concise description of what the bug is. -->

## 📋 Environment
- **AgentPay Version**: (e.g., `0.1.0` or git commit SHA)
- **Python Version**: (e.g., `3.11.5`)
- **Operating System**: (e.g., macOS 14.5 / Ubuntu 22.04 / Windows 11)
- **Deployment Mode**: (e.g., Local Python venv / Docker / FastAPI Gateway)

## 🔄 Steps to Reproduce
1. Configure policy with `...`
2. Run client call `client.post(...)`
3. See error

```python
# Provide a minimal runnable Python code snippet reproducing the issue:
from agentpay.client import AgentPayClient
# ...
```

## 🎯 Expected Behavior
<!-- A clear and concise description of what you expected to happen. -->

## 💥 Actual Behavior & Traceback
<!-- Include full traceback or gateway error log. Please mask any private keys or sensitive tokens before posting! -->
```text
Paste logs or terminal output here
```

## 🔒 Policy Configuration (if relevant)
<!-- If this relates to budget, whitelist, or circuit breaker, paste your policy YAML or dict below (mask sensitive data): -->
```yaml
wallet: "0x..."
max_amount_per_tx: "1.000000"
# ...
```

## 💡 Additional Context
<!-- Add any other context about the problem here (e.g., network latency, high concurrency, specific upstream API). -->
