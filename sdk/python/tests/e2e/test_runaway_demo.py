"""
End-to-End Test verifying the Runaway Agent & Circuit Breaker Demo (T6.4).

Ensures that the demo script runs to completion, executes all 3 phases:
- Phase 1: Successful payment & Arbitrum Sepolia on-chain settlement (with txHash)
- Phase 2: Single-transaction cap violation defense
- Phase 3: Runaway infinite loop circuit breaker tripping
"""

from __future__ import annotations

import importlib.util
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[4]
DEMO_PATH = PROJECT_ROOT / "examples" / "runaway-agent" / "demo.py"


def test_runaway_agent_demo_execution(capsys) -> None:
    """Verify that the runaway agent demo executes cleanly and outputs structured logs."""
    spec = importlib.util.spec_from_file_location("runaway_agent_demo", str(DEMO_PATH))
    assert spec is not None and spec.loader is not None
    demo_module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(demo_module)

    exit_code = demo_module.run_demo(live_mode=False)
    assert exit_code == 0

    captured = capsys.readouterr()
    stdout = captured.out

    # 1. Verify Phase 1: Normal payment & txHash
    assert "ON_CHAIN_TRANSACTION_CONFIRMED" in stdout
    assert "0x7f9a12c8" in stdout
    assert "https://sepolia.arbiscan.io/tx/0x" in stdout

    # 2. Verify Phase 2: Policy violation interception
    assert "POLICY_VIOLATION_INTERCEPTED" in stdout
    assert "SINGLE_TX_CAP_EXCEEDED" in stdout

    # 3. Verify Phase 3: Circuit breaker tripping
    assert "CIRCUIT_BREAKER_TRIPPED" in stdout
    assert "CIRCUIT_BREAKER_OPEN" in stdout

    # 4. Verify Final Report
    assert "DEMO AUDIT & SAFETY REPORT" in stdout
    assert "Total Funds Saved by AgentPay:" in stdout
