#!/usr/bin/env python3
"""
Build all PyPI distributions for AgentPay:
1. open-agentpay (Canonical SDK)
2. agentpay-gateway (Gateway Service)
3. agentpay-core (Metapackage alias)
4. agentpay-firewall (Metapackage alias)
"""

import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent
DIST_DIR = ROOT_DIR / "dist_all"

STUB_PACKAGES = [
    {
        "name": "agentpay-core",
        "description": "Core payment infrastructure for AI agents (Official alias for open-agentpay)",
    },
    {
        "name": "agentpay-firewall",
        "description": "Financial firewall & runaway circuit breaker for AI agents (Official alias for open-agentpay)",
    },
]


def get_version() -> str:
    init_py = ROOT_DIR / "python" / "packages" / "agentpay" / "src" / "agentpay" / "__init__.py"
    for line in init_py.read_text(encoding="utf-8").splitlines():
        if line.startswith("__version__"):
            return line.split("=")[1].strip().strip('"').strip("'")
    return "0.2.2"


def run(cmd: list[str], cwd: Path | None = None) -> None:
    print(f"--> Running: {' '.join(str(c) for c in cmd)} in {cwd or '.'}")
    res = subprocess.run(cmd, cwd=cwd, check=True)
    if res.returncode != 0:
        sys.exit(res.returncode)


def build_primary_packages() -> None:
    # 1. open-agentpay
    agentpay_pkg = ROOT_DIR / "python" / "packages" / "agentpay"
    run([sys.executable, "-m", "build", "--no-isolation", "--outdir", str(DIST_DIR)], cwd=agentpay_pkg)

    # 2. agentpay-gateway
    gateway_pkg = ROOT_DIR / "python" / "packages" / "agentpay-gateway"
    run([sys.executable, "-m", "build", "--no-isolation", "--outdir", str(DIST_DIR)], cwd=gateway_pkg)


def build_stub_packages(version: str) -> None:
    for pkg in STUB_PACKAGES:
        name = pkg["name"]
        desc = pkg["description"]
        print(f"\n--> Preparing alias package: {name} (v{version})")

        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)

            module_name = name.replace("-", "_")
            pkg_src = tmp_path / "src" / module_name
            pkg_src.mkdir(parents=True, exist_ok=True)
            (pkg_src / "__init__.py").write_text(f'"""Official alias for open-agentpay ({name})."""\n', encoding="utf-8")

            # Write pyproject.toml
            pyproject_content = f"""[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "{name}"
version = "{version}"
description = "{desc}"
readme = "README.md"
license = "Apache-2.0"
authors = [
    {{ name = "AgentPay Contributors" }}
]
requires-python = ">=3.10"
dependencies = [
    "open-agentpay>={version}"
]

[project.urls]
Homepage = "https://github.com/open-agentpay/AgentPay"
Documentation = "https://open-agentpay.github.io/AgentPay/"
Repository = "https://github.com/open-agentpay/AgentPay"

[tool.hatch.build.targets.wheel]
packages = ["src/{module_name}"]
"""
            (tmp_path / "pyproject.toml").write_text(pyproject_content, encoding="utf-8")

            # Write README.md
            readme_content = f"""# {name}

This package is an official alias and wrapper for [open-agentpay](https://pypi.org/project/open-agentpay/).

Installing this package automatically installs `open-agentpay`.

```bash
pip install {name}
```

In your Python code, import `agentpay` directly:

```python
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import SQLitePolicyEngine, PolicyConfig
```

For documentation and full source code, visit:
- **Official Documentation**: https://open-agentpay.github.io/AgentPay/
- **GitHub Repository**: https://github.com/open-agentpay/AgentPay
"""
            (tmp_path / "README.md").write_text(readme_content, encoding="utf-8")

            run([sys.executable, "-m", "build", "--no-isolation", "--outdir", str(DIST_DIR)], cwd=tmp_path)


def main() -> None:
    version = get_version()
    print(f"=== AgentPay Python Distribution Build (Target Version: v{version}) ===")

    if DIST_DIR.exists():
        shutil.rmtree(DIST_DIR)
    DIST_DIR.mkdir(parents=True, exist_ok=True)

    print("\n=== Step 1: Building primary packages (open-agentpay & agentpay-gateway) ===")
    build_primary_packages()

    print(f"\n=== Step 2: Building alias stub packages (v{version}) ===")
    build_stub_packages(version)

    print("\n=== SUCCESS: All packages built in dist_all/ ===")
    for f in sorted(DIST_DIR.iterdir()):
        print(f"  • {f.name} ({f.stat().st_size:,} bytes)")


if __name__ == "__main__":
    main()
