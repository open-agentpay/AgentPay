"""
Unit tests for T1.3: Cross-Language Protocol Consistency & Error Taxonomy.

Validates adherence to:
  - specs/protocol-consistency-spec.md
  - specs/conformance/protocol-errors.json
  - specs/test-vectors/dual-path-protocol.json
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

ERRORS_PATH = Path(__file__).resolve().parents[3] / "specs" / "conformance" / "protocol-errors.json"
VECTORS_PATH = Path(__file__).resolve().parents[3] / "specs" / "test-vectors" / "dual-path-protocol.json"


@pytest.fixture(scope="session")
def error_taxonomy() -> dict[str, Any]:
    assert ERRORS_PATH.exists(), f"Error taxonomy missing at {ERRORS_PATH}"
    with open(ERRORS_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


@pytest.fixture(scope="session")
def dual_path_vectors() -> dict[str, Any]:
    assert VECTORS_PATH.exists(), f"Dual-path vectors missing at {VECTORS_PATH}"
    with open(VECTORS_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


class TestProtocolConformanceVectors:
    """Verifies that Python and future Go Gateway share identical protocol rules."""

    def test_canonical_errors_integrity(self, error_taxonomy: dict[str, Any]) -> None:
        """Validates that all expected canonical error codes exist and have valid HTTP statuses."""
        assert error_taxonomy["version"] == "2.0.0"
        errors = error_taxonomy["errors"]
        assert len(errors) >= 17, "Taxonomy must define all 17 canonical error codes"

        code_map = {e["code"]: e for e in errors}

        # Core wallet error checks
        assert "ERR_WALLET_INSUFFICIENT_BALANCE" in code_map
        assert code_map["ERR_WALLET_INSUFFICIENT_BALANCE"]["status"] == 402
        assert code_map["ERR_WALLET_INSUFFICIENT_BALANCE"]["domain"] == "WALLET"

        assert "ERR_WALLET_FROZEN" in code_map
        assert code_map["ERR_WALLET_FROZEN"]["status"] == 403

        # Routing & Policy error checks
        assert "ERR_PAYMENT_METHOD_NOT_ALLOWED" in code_map
        assert code_map["ERR_PAYMENT_METHOD_NOT_ALLOWED"]["status"] == 403

        # Idempotency error checks
        assert "ERR_IDEMPOTENCY_CONFLICT" in code_map
        assert code_map["ERR_IDEMPOTENCY_CONFLICT"]["status"] == 409

    def test_dual_path_vectors_execution(
        self, dual_path_vectors: dict[str, Any], error_taxonomy: dict[str, Any]
    ) -> None:
        """Executes each test case in dual-path-protocol.json against deterministic mock assertions."""
        test_cases = dual_path_vectors["testCases"]
        error_codes = {e["code"] for e in error_taxonomy["errors"]}

        for tc in test_cases:
            tc_id = tc["id"]
            expected = tc["expected"]

            if tc_id == "TC_PROTO_01_ONCHAIN_CHALLENGE_FORMAT":
                assert expected["http_status"] == 402
                assert expected["challenge_header"] == "PAYMENT-REQUIRED"
                assert expected["challenge_payload"]["version"] == "2.0"
                assert expected["challenge_payload"]["scheme"] == "EIP-3009"

            elif tc_id == "TC_PROTO_02_WALLET_APIKEY_SELECTION":
                assert expected["selected_payment_method"] == "wallet"
                assert expected["auto_downgrade"] is False
                assert expected["target_status"] == "held"

            elif tc_id == "TC_PROTO_03_WALLET_INSUFFICIENT_NO_DOWNGRADE":
                # Strict check on No Auto-Downgrade Invariant
                assert expected["http_status"] == 402
                assert expected["error_code"] == "ERR_WALLET_INSUFFICIENT_BALANCE"
                assert expected["auto_downgrade_to_onchain"] is False
                assert expected["error_code"] in error_codes

            elif tc_id == "TC_PROTO_04_WALLET_FROZEN_ARREARS":
                assert expected["http_status"] == 403
                assert expected["error_code"] == "ERR_WALLET_FROZEN"
                assert expected["error_code"] in error_codes

            elif tc_id == "TC_PROTO_05_PAYMENT_METHOD_MISMATCH":
                assert expected["http_status"] == 403
                assert expected["error_code"] == "ERR_PAYMENT_METHOD_NOT_ALLOWED"
                assert expected["error_code"] in error_codes

            elif tc_id == "TC_PROTO_06_IDEMPOTENCY_CONFLICT":
                assert expected["http_status"] == 409
                assert expected["error_code"] == "ERR_IDEMPOTENCY_CONFLICT"
                assert expected["error_code"] in error_codes
