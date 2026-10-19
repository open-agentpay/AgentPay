"""
Unit tests for T1.1: State Machine Specification & Matrix Conformance.

Verifies adherence to specs/payment-state-machine-v2.md and specs/conformance/state-machine.json:
  1. Table-driven test: All legal (state, event) transitions produce exact target states.
  2. Strict Rejection: All illegal/unregistered (state, event) transitions are rejected.
  3. Terminal State Immutability: Entities in terminal states reject any subsequent state-changing event.
  4. Dual-path dispatch guard:
     - Onchain path: requires settlement == confirmed.
     - Wallet path: requires settlement == held.
  5. Unknown Guard: Settlement == unknown strictly forbids re-submission.
  6. Upstream 5xx Guard: Upstream 5xx/timeout transitions Fulfillment to unknown, never directly failed.
  7. Wallet Hold Safety: Wallet hold transitions cleanly to confirmed (capture) or released (release).
  8. Composite view derivations: Accurate mapping of orthogonal states to high-level display status.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

SPEC_PATH = Path(__file__).resolve().parents[3] / "specs" / "conformance" / "state-machine.json"


@pytest.fixture(scope="session")
def state_machine_spec() -> dict[str, Any]:
    """Loads the canonical language-agnostic state machine specification."""
    assert SPEC_PATH.exists(), f"State machine specification not found at {SPEC_PATH}"
    with open(SPEC_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


class StateMachineEngine:
    """
    Lightweight, deterministic reference state machine engine driven by state-machine.json.
    Used to validate transitions and enforce invariants across runtimes.
    """

    def __init__(self, spec: dict[str, Any]):
        self.spec = spec
        self.entities = spec["entities"]

    def transition(self, entity_name: str, current_state: str, event: str) -> str:
        entity_spec = self.entities[entity_name]
        states = entity_spec["states"]

        if current_state not in states:
            raise ValueError(f"Unknown state '{current_state}' for entity '{entity_name}'")

        # Invariant: Terminal states reject state mutations
        if states[current_state]["is_terminal"]:
            raise RuntimeError(
                f"IllegalStateTransition: Entity '{entity_name}' in terminal state '{current_state}' "
                f"cannot process event '{event}'"
            )

        # Look up transition
        for t in entity_spec["transitions"]:
            if t["from"] == current_state and t["event"] == event:
                target_state = t["to"]
                return target_state

        raise ValueError(
            f"IllegalStateTransition: No legal transition for entity '{entity_name}' "
            f"from '{current_state}' with event '{event}'"
        )

    def can_dispatch_fulfillment(self, settlement_state: str, payment_method: str = "onchain") -> bool:
        """Enforces INV_1_DISPATCH_GUARD across both onchain and wallet paths."""
        if payment_method == "wallet":
            return settlement_state == "held"
        return settlement_state == "confirmed"

    def can_submit_settlement(self, settlement_state: str) -> bool:
        """Enforces INV_2_UNKNOWN_SUBMIT_GUARD: cannot submit if status is unknown or submitted."""
        return settlement_state == "not_started"

    def derive_composite_view(self, settlement_state: str, fulfillment_state: str) -> str:
        """Computes high-level composite view from orthogonal dimensions."""
        for view in self.spec["coordination"]["composite_views"]:
            if settlement_state in view["settlement"] and fulfillment_state in view["fulfillment"]:
                return view["display"]
        return "UNKNOWN_COMPOSITE"


class TestT11StateMachineMatrix:
    """Test suite directly validating T1.1 specification matrix and conformance invariants."""

    def test_spec_integrity(self, state_machine_spec: dict[str, Any]) -> None:
        """Validates that state-machine.json has all required schema definitions."""
        assert state_machine_spec["version"] == "2.1.0"
        assert state_machine_spec["supported_payment_methods"] == ["onchain", "wallet"]
        entities = state_machine_spec["entities"]
        assert "payment_intent" in entities
        assert "settlement" in entities
        assert "fulfillment" in entities

    def test_payment_intent_legal_transitions(self, state_machine_spec: dict[str, Any]) -> None:
        """Tests that all documented transitions for PaymentIntent succeed as specified."""
        engine = StateMachineEngine(state_machine_spec)
        entity_spec = state_machine_spec["entities"]["payment_intent"]

        for t in entity_spec["transitions"]:
            res = engine.transition("payment_intent", t["from"], t["event"])
            assert res == t["to"]

    def test_settlement_legal_transitions(self, state_machine_spec: dict[str, Any]) -> None:
        """Tests that all documented transitions for Settlement succeed as specified."""
        engine = StateMachineEngine(state_machine_spec)
        entity_spec = state_machine_spec["entities"]["settlement"]

        for t in entity_spec["transitions"]:
            res = engine.transition("settlement", t["from"], t["event"])
            assert res == t["to"]

    def test_fulfillment_legal_transitions(self, state_machine_spec: dict[str, Any]) -> None:
        """Tests that all documented transitions for Fulfillment succeed as specified."""
        engine = StateMachineEngine(state_machine_spec)
        entity_spec = state_machine_spec["entities"]["fulfillment"]

        for t in entity_spec["transitions"]:
            res = engine.transition("fulfillment", t["from"], t["event"])
            assert res == t["to"]

    def test_terminal_states_immutable(self, state_machine_spec: dict[str, Any]) -> None:
        """Ensures that no terminal state can ever transition, regardless of event."""
        engine = StateMachineEngine(state_machine_spec)

        for entity_name, entity_spec in state_machine_spec["entities"].items():
            terminal_states = [s for s, meta in entity_spec["states"].items() if meta["is_terminal"]]
            assert len(terminal_states) > 0, f"Entity {entity_name} should have terminal states"

            for term_state in terminal_states:
                for event in entity_spec["events"]:
                    with pytest.raises(RuntimeError) as exc_info:
                        engine.transition(entity_name, term_state, event)
                    assert "terminal state" in str(exc_info.value)

    def test_unregistered_transitions_rejected(self, state_machine_spec: dict[str, Any]) -> None:
        """Tests that state/event combinations outside the matrix are strictly rejected."""
        engine = StateMachineEngine(state_machine_spec)

        for entity_name, entity_spec in state_machine_spec["entities"].items():
            non_terminal_states = [s for s, meta in entity_spec["states"].items() if not meta["is_terminal"]]
            valid_pairs = {(t["from"], t["event"]) for t in entity_spec["transitions"]}

            for state in non_terminal_states:
                for event in entity_spec["events"]:
                    if (state, event) not in valid_pairs:
                        with pytest.raises(ValueError) as exc_info:
                            engine.transition(entity_name, state, event)
                        assert "IllegalStateTransition" in str(exc_info.value)

    def test_dual_path_fulfillment_dispatch_guards(self, state_machine_spec: dict[str, Any]) -> None:
        """Validates dispatch guards for onchain vs internal wallet paths."""
        engine = StateMachineEngine(state_machine_spec)

        # Onchain path: requires confirmed
        assert not engine.can_dispatch_fulfillment("not_started", payment_method="onchain")
        assert not engine.can_dispatch_fulfillment("held", payment_method="onchain")
        assert not engine.can_dispatch_fulfillment("pending", payment_method="onchain")
        assert engine.can_dispatch_fulfillment("confirmed", payment_method="onchain")

        # Wallet path: requires held (quota reserved in ledger)
        assert not engine.can_dispatch_fulfillment("not_started", payment_method="wallet")
        assert not engine.can_dispatch_fulfillment("failed", payment_method="wallet")
        assert engine.can_dispatch_fulfillment("held", payment_method="wallet")

    def test_wallet_hold_lifecycle(self, state_machine_spec: dict[str, Any]) -> None:
        """Tests the wallet settlement lifecycle: not_started -> held -> (confirmed | released)."""
        engine = StateMachineEngine(state_machine_spec)

        # Successful delivery -> capture
        s1 = engine.transition("settlement", "not_started", "hold_acquired")
        assert s1 == "held"
        s2 = engine.transition("settlement", s1, "capture")
        assert s2 == "confirmed"

        # Upstream failure -> release
        s3 = engine.transition("settlement", "not_started", "hold_acquired")
        assert s3 == "held"
        s4 = engine.transition("settlement", s3, "release")
        assert s4 == "released"

    def test_unknown_settlement_anti_blind_retry(self, state_machine_spec: dict[str, Any]) -> None:
        """Validates INV_2_UNKNOWN_SUBMIT_GUARD: Unknown state strictly forbids resubmission."""
        engine = StateMachineEngine(state_machine_spec)

        assert engine.can_submit_settlement("not_started")
        assert not engine.can_submit_settlement("unknown")
        assert not engine.can_submit_settlement("submitted")
        assert not engine.can_submit_settlement("pending")
        assert not engine.can_submit_settlement("confirmed")

    def test_upstream_5xx_never_fails_directly(self, state_machine_spec: dict[str, Any]) -> None:
        """Validates INV_4_UPSTREAM_5XX_UNKNOWN: upstream 5xx or timeout MUST go to unknown, not failed."""
        engine = StateMachineEngine(state_machine_spec)

        # 5xx -> unknown
        state_after_5xx = engine.transition("fulfillment", "in_progress", "upstream_5xx")
        assert state_after_5xx == "unknown"

        # timeout -> unknown
        state_after_timeout = engine.transition("fulfillment", "in_progress", "upstream_timeout")
        assert state_after_timeout == "unknown"

        # 4xx -> failed (explicit non-execution client error)
        state_after_4xx = engine.transition("fulfillment", "in_progress", "upstream_4xx")
        assert state_after_4xx == "failed"

    def test_composite_views_derivation(self, state_machine_spec: dict[str, Any]) -> None:
        """Validates composite display view mappings across all combinations."""
        engine = StateMachineEngine(state_machine_spec)

        assert engine.derive_composite_view("held", "not_started") == "WALLET_HOLD_PROCESSING"
        assert engine.derive_composite_view("pending", "not_started") == "SETTLEMENT_IN_PROGRESS"
        assert engine.derive_composite_view("unknown", "not_started") == "SETTLEMENT_IN_PROGRESS"
        assert engine.derive_composite_view("confirmed", "in_progress") == "FULFILLMENT_IN_PROGRESS"
        assert engine.derive_composite_view("confirmed", "succeeded") == "COMPLETED"
        assert engine.derive_composite_view("confirmed", "failed") == "SETTLED_FULFILLMENT_FAILED_REFUND_REQUIRED"
        assert engine.derive_composite_view("confirmed", "unknown") == "SETTLED_FULFILLMENT_UNKNOWN_NEEDS_REVIEW"
        assert engine.derive_composite_view("released", "failed") == "RELEASED_UNBILLED"
        assert engine.derive_composite_view("failed", "not_started") == "PAYMENT_FAILED"
