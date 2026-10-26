#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <version> (for example: $0 0.2.4)" >&2
  exit 2
fi
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "Invalid version: $VERSION" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

python3 - "$REPO_ROOT" "$VERSION" <<'PY'
from pathlib import Path
import re
import sys

root = Path(sys.argv[1])
version = sys.argv[2]
files = {
    "sdk/python/pyproject.toml": (r'^(version\s*=\s*")[^"]+("\s*)$', rf'\g<1>{version}\2'),
    "sdk/python/packages/agentpay/pyproject.toml": (r'^(version\s*=\s*")[^"]+("\s*)$', rf'\g<1>{version}\2'),
    "sdk/python/packages/agentpay-gateway/pyproject.toml": (r'^(version\s*=\s*")[^"]+("\s*)$', rf'\g<1>{version}\2'),
    "sdk/python/packages/agentpay/src/agentpay/__init__.py": (r'^(__version__\s*=\s*")[^"]+("\s*)$', rf'\g<1>{version}\2'),
    "sdk/python/packages/agentpay-gateway/src/agentpay_gateway/__init__.py": (r'^(__version__\s*=\s*")[^"]+("\s*)$', rf'\g<1>{version}\2'),
    "sdk/python/tests/unit/test_smoke.py": (r'^(\s*self\.assertEqual\(agentpay\.__version__,\s*")[^"]+("\)\s*)$', rf'\g<1>{version}\2'),
    "scripts/build_pypi_dist.py": (r'^(\s*return\s+")[^"]+("\s*)$', rf'\g<1>{version}\2'),
}

for relative_path, (pattern, replacement) in files.items():
    path = root / relative_path
    content = path.read_text(encoding="utf-8")
    updated, count = re.subn(pattern, replacement, content, count=1, flags=re.MULTILINE)
    if count != 1:
        raise SystemExit(f"Expected exactly one version field in {relative_path}, found {count}")
    if updated != content:
        path.write_text(updated, encoding="utf-8")
        print(f"Updated: {relative_path}")
    else:
        print(f"Already set: {relative_path}")

print(f"Python SDK package versions set to {version}.")
PY
