#!/usr/bin/env python3
"""
Build and publish npm packages for AgentPay:
1. open-agentpay (Canonical core package)
2. agentpay-core (Alias package)
3. agent-pay (Alias package)
4. agentpay-firewall (Alias package)
"""

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent
NPM_DIST = ROOT_DIR / "npm_dist"

NPM_TOKEN = os.environ.get("NPM_TOKEN", "")
OTP_CODE = os.environ.get("NPM_OTP", None)

PACKAGES = [
    {
        "name": "open-agentpay",
        "version": "0.1.0",
        "description": "The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents",
        "is_primary": True,
    },
    {
        "name": "agentpay-core",
        "version": "0.1.0",
        "description": "Core payment infrastructure for AI agents (Official alias for open-agentpay)",
        "is_primary": False,
    },
    {
        "name": "agentpay-firewall",
        "version": "0.1.0",
        "description": "Financial firewall & runaway circuit breaker for AI agents (Official alias for open-agentpay)",
        "is_primary": False,
    },
]


def prepare_packages() -> None:
    if NPM_DIST.exists():
        shutil.rmtree(NPM_DIST)
    NPM_DIST.mkdir(parents=True, exist_ok=True)

    for pkg in PACKAGES:
        pkg_dir = NPM_DIST / pkg["name"]
        pkg_dir.mkdir(parents=True, exist_ok=True)

        pkg_json = {
            "name": pkg["name"],
            "version": pkg["version"],
            "description": pkg["description"],
            "main": "index.js",
            "types": "index.d.ts",
            "author": "AgentPay Contributors",
            "license": "Apache-2.0",
            "repository": {
                "type": "git",
                "url": "https://github.com/open-agentpay/AgentPay.git",
            },
            "homepage": "https://github.com/open-agentpay/AgentPay#readme",
            "keywords": [
                "agent",
                "ai",
                "llm",
                "x402",
                "usdc",
                "eip-712",
                "financial-firewall",
                "arbitrum",
            ],
        }

        if not pkg["is_primary"]:
            pkg_json["dependencies"] = {"open-agentpay": "^0.1.0"}

        pkg_json["publishConfig"] = {
            "registry": "https://registry.npmjs.org/",
            "access": "public",
        }

        (pkg_dir / "package.json").write_text(json.dumps(pkg_json, indent=2) + "\n", encoding="utf-8")

        # Write index.js & index.d.ts
        if pkg["is_primary"]:
            (pkg_dir / "index.js").write_text(
                "/**\n"
                " * open-agentpay\n"
                " * The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents\n"
                " */\n"
                "module.exports = {\n"
                "  version: '0.1.0',\n"
                "  description: 'The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents'\n"
                "};\n",
                encoding="utf-8",
            )
            (pkg_dir / "index.d.ts").write_text(
                "export const version: string;\nexport const description: string;\n",
                encoding="utf-8",
            )
            (pkg_dir / "README.md").write_text(
                f"# {pkg['name']}\n\n"
                "**The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents.**\n\n"
                "AgentPay equips autonomous AI agents with gasless micro-payments (Circle USDC on Arbitrum Sepolia & Base) "
                "governed by an in-process, non-custodial **Financial Firewall**.\n\n"
                "## Installation\n\n"
                "```bash\n"
                f"npm install {pkg['name']}\n"
                "```\n\n"
                "For documentation, visit https://github.com/open-agentpay/AgentPay\n",
                encoding="utf-8",
            )
        else:
            (pkg_dir / "index.js").write_text("module.exports = require('open-agentpay');\n", encoding="utf-8")
            (pkg_dir / "index.d.ts").write_text("export * from 'open-agentpay';\n", encoding="utf-8")
            (pkg_dir / "README.md").write_text(
                f"# {pkg['name']}\n\n"
                f"This package is an official alias and wrapper for [open-agentpay](https://www.npmjs.com/package/open-agentpay).\n\n"
                "Installing this package automatically installs `open-agentpay`.\n\n"
                "```bash\n"
                f"npm install {pkg['name']}\n"
                "```\n\n"
                "For documentation and full source code, visit: https://github.com/open-agentpay/AgentPay\n",
                encoding="utf-8",
            )


def publish_packages() -> None:
    # Publish primary first
    order = [p["name"] for p in PACKAGES if p["is_primary"]] + [p["name"] for p in PACKAGES if not p["is_primary"]]

    for name in order:
        pkg_dir = NPM_DIST / name
        print(f"\n--> Publishing {name} to npm...")
        cmd = [
            "npm",
            "publish",
            "--access",
            "public",
            "--registry",
            "https://registry.npmjs.org/",
        ]
        res = subprocess.run(
            cmd,
            cwd=pkg_dir,
            capture_output=True,
            text=True,
        )
        if res.returncode == 0:
            print(f"✓ Successfully published {name}!")
        else:
            print(f"✗ Failed to publish {name}:\n{res.stderr}\n{res.stdout}")


def main() -> None:
    print("=== Step 1: Generating npm package definitions ===")
    prepare_packages()

    print("\n=== Step 2: Publishing to npm Registry ===")
    publish_packages()

    print("\n=== All Done ===")


if __name__ == "__main__":
    main()
