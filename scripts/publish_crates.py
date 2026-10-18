#!/usr/bin/env python3
"""
Publish crates to crates.io:
1. agentpay (Canonical core name)
2. open-agentpay (Matching PyPI & npm)
3. agentpay-core (Alias)
4. agentpay-gateway (Gateway service)
5. agentpay-firewall (Firewall)
"""

import os
import subprocess
import sys
import tempfile
from pathlib import Path

CRATES = [
    {
        "name": "agentpay",
        "version": "0.1.0",
        "description": "The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents",
    },
    {
        "name": "open-agentpay",
        "version": "0.1.0",
        "description": "The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents",
    },
    {
        "name": "agentpay-core",
        "version": "0.1.0",
        "description": "Core payment infrastructure for AI agents (Open AgentPay)",
    },
    {
        "name": "agentpay-gateway",
        "version": "0.1.0",
        "description": "Reverse proxy & seller gateway for autonomous AI agent micro-payments",
    },
    {
        "name": "agentpay-firewall",
        "version": "0.1.0",
        "description": "Financial firewall & runaway circuit breaker for autonomous AI agents",
    },
]


def publish_crate(token: str, name: str, version: str, description: str) -> bool:
    print(f"\n--> Packaging & Publishing crate: {name} (v{version})...")
    with tempfile.TemporaryDirectory() as tmp_dir:
        p = Path(tmp_dir)
        cargo_toml = f"""[package]
name = "{name}"
version = "{version}"
edition = "2021"
description = "{description}"
license = "Apache-2.0"
repository = "https://github.com/open-agentpay/AgentPay"
readme = "README.md"
keywords = ["agent", "ai", "x402", "payment", "firewall"]
categories = ["cryptography::cryptocurrencies", "web-programming::http-client"]

[dependencies]
"""
        (p / "Cargo.toml").write_text(cargo_toml, encoding="utf-8")
        readme = f"""# {name}

**{description}**

Part of the [Open AgentPay](https://github.com/open-agentpay/AgentPay) project.

- Website & Docs: https://github.com/open-agentpay/AgentPay
- PyPI: https://pypi.org/project/open-agentpay/
- npm: https://www.npmjs.com/package/open-agentpay
"""
        (p / "README.md").write_text(readme, encoding="utf-8")
        src = p / "src"
        src.mkdir()
        (src / "lib.rs").write_text(
            f"//! {name}\n"
            f"//! {description}\n\n"
            'pub fn version() -> &\'static str {\n'
            '    env!("CARGO_PKG_VERSION")\n'
            "}\n",
            encoding="utf-8",
        )

        env = os.environ.copy()
        env["CARGO_REGISTRY_TOKEN"] = token

        res = subprocess.run(
            ["cargo", "publish"],
            cwd=p,
            env=env,
            capture_output=True,
            text=True,
        )
        if res.returncode == 0:
            print(f"✓ Successfully published {name} v{version} to crates.io!")
            return True
        else:
            print(f"✗ Failed to publish {name}:\n{res.stderr}\n{res.stdout}")
            return False


def main() -> None:
    token = os.environ.get("CARGO_REGISTRY_TOKEN", "")
    print(f"=== Publishing {len(CRATES)} crates to crates.io ===")
    successes = []
    failures = []

    for c in CRATES:
        ok = publish_crate(token, c["name"], c["version"], c["description"])
        if ok:
            successes.append(c["name"])
        else:
            failures.append(c["name"])

    print("\n=== Summary ===")
    print(f"Success ({len(successes)}): {', '.join(successes)}")
    if failures:
        print(f"Failures ({len(failures)}): {', '.join(failures)}")


if __name__ == "__main__":
    main()
