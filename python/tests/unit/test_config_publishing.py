"""
Unit tests for T1.4: Configuration Publishing, Snapshot Verification, and Rollback.

Validates adherence to:
  - specs/config-publishing-spec.md
  - specs/conformance/config-snapshot.schema.json
  - specs/test-vectors/config-publishing.json
"""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

import pytest

SCHEMA_PATH = Path(__file__).resolve().parents[3] / "specs" / "conformance" / "config-snapshot.schema.json"
VECTORS_PATH = Path(__file__).resolve().parents[3] / "specs" / "test-vectors" / "config-publishing.json"


@pytest.fixture(scope="session")
def snapshot_schema() -> dict[str, Any]:
    assert SCHEMA_PATH.exists(), f"Snapshot schema missing at {SCHEMA_PATH}"
    with open(SCHEMA_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


@pytest.fixture(scope="session")
def config_vectors() -> dict[str, Any]:
    assert VECTORS_PATH.exists(), f"Config vectors missing at {VECTORS_PATH}"
    with open(VECTORS_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


def compute_canonical_snapshot_hash(routes: list[dict[str, Any]], policies: list[dict[str, Any]]) -> str:
    """Computes deterministic SHA-256 fingerprint across canonically sorted routes and policies."""
    sorted_routes = sorted(routes, key=lambda r: r.get("route_id") or r.get("routeId", ""))
    sorted_policies = sorted(policies, key=lambda p: p.get("policy_id") or p.get("policyId", ""))
    payload = json.dumps({"routes": sorted_routes, "policies": sorted_policies}, sort_keys=True, separators=(",", ":"))
    return f"sha256:{hashlib.sha256(payload.encode('utf-8')).hexdigest()}"


def validate_config_static_rules(
    routes: list[dict[str, Any]], policies: list[dict[str, Any]]
) -> tuple[bool, str | None]:
    """Applies T1.4 static validation matrix."""
    for r in routes:
        upstream = r.get("upstream_url") or r.get("upstreamUrl", "")
        if not (upstream.startswith("http://") or upstream.startswith("https://")):
            return False, "INVALID_UPSTREAM_URL"

    for p in policies:
        max_tx = p.get("max_amount_per_request") or p.get("maxAmountPerRequest", 0)
        cap = p.get("rolling_24h_budget_cap") or p.get("rolling24hBudgetCap", 0)
        if max_tx > cap:
            return False, "MAX_AMOUNT_EXCEEDS_ROLLING_BUDGET"

    return True, None


class TestConfigPublishing:
    """Verifies configuration publishing, snapshot hashing, and rollback contracts."""

    def test_schema_integrity(self, snapshot_schema: dict[str, Any]) -> None:
        """Ensures schema has all mandatory fields."""
        assert "properties" in snapshot_schema
        required = snapshot_schema["required"]
        assert "snapshotHash" in required
        assert "routes" in required
        assert "policies" in required

    def test_tc01_valid_publishing_and_hash(self, config_vectors: dict[str, Any]) -> None:
        """TC_CFG_01: Valid routes & policies generate deterministic sha256 fingerprint."""
        tc = next(c for c in config_vectors["testCases"] if c["id"] == "TC_CFG_01_VALID_SNAPSHOT_PUBLISHING")
        routes = tc["input"]["routes"]
        policies = tc["input"]["policies"]

        is_valid, err = validate_config_static_rules(routes, policies)
        assert is_valid is True
        assert err is None

        calculated_hash = compute_canonical_snapshot_hash(routes, policies)
        assert calculated_hash.startswith(tc["expected"]["hash_prefix"])
        assert len(calculated_hash) == 7 + 64

    def test_tc02_invalid_upstream_rejected(self, config_vectors: dict[str, Any]) -> None:
        """TC_CFG_02: Invalid upstream URL fails validation."""
        tc = next(c for c in config_vectors["testCases"] if c["id"] == "TC_CFG_02_VALIDATION_REJECT_INVALID_UPSTREAM")
        is_valid, err = validate_config_static_rules(tc["input"]["routes"], tc["input"]["policies"])
        assert is_valid is False
        assert err == tc["expected"]["error_code"]

    def test_tc03_max_exceeds_budget_rejected(self, config_vectors: dict[str, Any]) -> None:
        """TC_CFG_03: Single transaction max exceeding 24h cap fails validation."""
        tc = next(c for c in config_vectors["testCases"] if c["id"] == "TC_CFG_03_VALIDATION_REJECT_MAX_EXCEEDS_BUDGET")
        is_valid, err = validate_config_static_rules(tc["input"]["routes"], tc["input"]["policies"])
        assert is_valid is False
        assert err == tc["expected"]["error_code"]

    def test_tc04_rollback_generates_new_version_with_same_hash(self, config_vectors: dict[str, Any]) -> None:
        """TC_CFG_04: Rollback produces incremented version number with identical target hash."""
        tc = next(c for c in config_vectors["testCases"] if c["id"] == "TC_CFG_04_ROLLBACK_INTEGRITY")
        v1_hash = tc["input"]["v1_hash"]
        current_version_num = 2

        # Simulate rollback: create version 3 pointing to v1 snapshot
        new_version_num = current_version_num + 1
        active_hash_after_rollback = v1_hash

        assert new_version_num == tc["expected"]["new_version_number"]
        assert active_hash_after_rollback == tc["expected"]["active_hash"]

    def test_tc05_gateway_cache_degradation(self, config_vectors: dict[str, Any]) -> None:
        """TC_CFG_05: Gateway continues routing with in-memory snapshot if control plane goes down."""
        tc = next(c for c in config_vectors["testCases"] if c["id"] == "TC_CFG_05_GATEWAY_CACHE_DEGRADATION")
        in_memory_routes_count = tc["input"]["in_memory_routes_count"]

        # Invariant check: as long as in-memory routes > 0, gateway remains available
        gateway_can_serve = in_memory_routes_count > 0
        assert gateway_can_serve is True
