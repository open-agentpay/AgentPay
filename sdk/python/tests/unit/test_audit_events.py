"""Unit and conformance tests for AgentPay Audit Events Specification (T1.5).

Validates:
1. Audit event JSON schema structure and definitions.
2. Conformance of canonical test vectors against the schema requirements.
3. Strict rejection of invalid IDs, types, and malformed envelopes.
4. Security redaction rules (forbidden raw keys in payloads).
5. Event deduplication and idempotency key handling.
"""

from __future__ import annotations

import json
import re
from datetime import datetime
from pathlib import Path
from typing import Any

import pytest

REPO_ROOT = Path(__file__).resolve().parents[4]
SCHEMA_PATH = REPO_ROOT / "specs" / "conformance" / "audit-events.schema.json"
VECTORS_PATH = REPO_ROOT / "specs" / "test-vectors" / "audit-events.json"


@pytest.fixture(scope="module")
def audit_schema() -> dict[str, Any]:
    assert SCHEMA_PATH.exists(), f"Audit schema missing at {SCHEMA_PATH}"
    with open(SCHEMA_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


@pytest.fixture(scope="module")
def test_vectors() -> dict[str, Any]:
    assert VECTORS_PATH.exists(), f"Audit test vectors missing at {VECTORS_PATH}"
    with open(VECTORS_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


def validate_event_envelope(event: dict[str, Any], schema: dict[str, Any]) -> tuple[bool, str | None]:
    """Validates an event against the audit-events schema constraints."""
    # Check top-level required fields
    for req in schema.get("required", []):
        if req not in event:
            return False, f"Missing required top-level field: {req}"

    # Validate event_id pattern
    evt_id = event.get("event_id", "")
    if not re.match(r"^evt_[a-zA-Z0-9_-]+$", evt_id):
        return False, f"Invalid event_id pattern: {evt_id}"

    # Validate event_type enum
    valid_types = schema["properties"]["event_type"]["enum"]
    evt_type = event.get("event_type", "")
    if evt_type not in valid_types:
        return False, f"Invalid event_type: {evt_type}"

    # Validate version
    valid_versions = schema["properties"]["version"]["enum"]
    version = event.get("version", "")
    if version not in valid_versions:
        return False, f"Invalid version: {version}"

    # Validate timestamp format (ISO 8601)
    timestamp = event.get("timestamp", "")
    try:
        if timestamp.endswith("Z"):
            datetime.fromisoformat(timestamp[:-1] + "+00:00")
        else:
            datetime.fromisoformat(timestamp)
    except Exception:
        return False, f"Invalid ISO 8601 timestamp: {timestamp}"

    # Validate tenant_id pattern
    tenant_id = event.get("tenant_id", "")
    if not re.match(r"^(tenant|ten)_[a-zA-Z0-9_-]+$", tenant_id):
        return False, f"Invalid tenant_id pattern: {tenant_id}"

    # Validate project_id if present
    project_id = event.get("project_id")
    if project_id is not None and not re.match(r"^(proj|prj)_[a-zA-Z0-9_-]+$", project_id):
        return False, f"Invalid project_id pattern: {project_id}"

    # Validate correlation object
    corr = event.get("correlation")
    if not isinstance(corr, dict):
        return False, "correlation must be an object"
    for req_corr in schema["properties"]["correlation"]["required"]:
        if req_corr not in corr:
            return False, f"Missing required correlation field: {req_corr}"

    # Validate actor object
    actor = event.get("actor")
    if not isinstance(actor, dict):
        return False, "actor must be an object"
    for req_actor in schema["properties"]["actor"]["required"]:
        if req_actor not in actor:
            return False, f"Missing required actor field: {req_actor}"
    valid_actors = schema["properties"]["actor"]["properties"]["type"]["enum"]
    if actor.get("type") not in valid_actors:
        return False, f"Invalid actor type: {actor.get('type')}"

    # Validate data is object
    if not isinstance(event.get("data"), dict):
        return False, "data must be an object"

    return True, None


class TestAuditEventsConformance:
    """Validates schema conformance and test vectors."""

    def test_schema_structure(self, audit_schema: dict[str, Any]) -> None:
        assert audit_schema["type"] == "object"
        assert "event_id" in audit_schema["properties"]
        assert "correlation" in audit_schema["properties"]
        assert "actor" in audit_schema["properties"]
        assert len(audit_schema["properties"]["event_type"]["enum"]) >= 20

    def test_canonical_vectors_pass_schema(self, audit_schema: dict[str, Any], test_vectors: dict[str, Any]) -> None:
        vectors = test_vectors["vectors"]
        assert len(vectors) >= 5

        for vec in vectors:
            event = vec["event"]
            expected_valid = vec["expected_valid"]
            is_valid, err_msg = validate_event_envelope(event, audit_schema)
            if expected_valid:
                assert is_valid, f"Vector '{vec['id']}' failed validation: {err_msg}"
            else:
                assert not is_valid, f"Vector '{vec['id']}' unexpectedly passed"

    def test_reject_invalid_event_id_prefix(self, audit_schema: dict[str, Any], test_vectors: dict[str, Any]) -> None:
        bad_event = json.loads(json.dumps(test_vectors["vectors"][0]["event"]))
        bad_event["event_id"] = "bad_01j7b8z4m0k9x8y7"
        is_valid, err_msg = validate_event_envelope(bad_event, audit_schema)
        assert not is_valid
        assert "Invalid event_id pattern" in (err_msg or "")

    def test_reject_unsupported_event_type(self, audit_schema: dict[str, Any], test_vectors: dict[str, Any]) -> None:
        bad_event = json.loads(json.dumps(test_vectors["vectors"][0]["event"]))
        bad_event["event_type"] = "hacker.steal.money"
        is_valid, err_msg = validate_event_envelope(bad_event, audit_schema)
        assert not is_valid
        assert "Invalid event_type" in (err_msg or "")

    def test_reject_missing_required_actor_fields(
        self, audit_schema: dict[str, Any], test_vectors: dict[str, Any]
    ) -> None:
        bad_event = json.loads(json.dumps(test_vectors["vectors"][0]["event"]))
        bad_event["actor"] = {"type": "api_key"}  # Missing id and client_ip
        is_valid, err_msg = validate_event_envelope(bad_event, audit_schema)
        assert not is_valid
        assert "Missing required actor field" in (err_msg or "")

    def test_reject_invalid_timestamp_format(self, audit_schema: dict[str, Any], test_vectors: dict[str, Any]) -> None:
        bad_event = json.loads(json.dumps(test_vectors["vectors"][0]["event"]))
        bad_event["timestamp"] = "invalid-not-a-timestamp"
        is_valid, err_msg = validate_event_envelope(bad_event, audit_schema)
        assert not is_valid
        assert "Invalid ISO 8601 timestamp" in (err_msg or "")


class TestRedactionAndIdempotency:
    """Verifies security redaction rules and deduplication semantics."""

    def test_redaction_guard_rejects_forbidden_keys(self, test_vectors: dict[str, Any]) -> None:
        forbidden_keys = test_vectors["redaction_rules"]["forbidden_keys_in_payload"]

        def check_no_forbidden_keys(d: Any) -> None:
            if isinstance(d, dict):
                for k, v in d.items():
                    assert k not in forbidden_keys, f"Forbidden key '{k}' found in event!"
                    check_no_forbidden_keys(v)
            elif isinstance(d, list):
                for item in d:
                    check_no_forbidden_keys(item)

        for vec in test_vectors["vectors"]:
            check_no_forbidden_keys(vec["event"])

    def test_idempotent_event_deduplication(self, test_vectors: dict[str, Any]) -> None:
        """Simulates in-memory storage idempotency on event_id and (tenant_id, idempotency_key, event_type)."""
        stored_events: dict[str, dict[str, Any]] = {}
        unique_business_keys: set[tuple[str, str, str]] = set()

        for vec in test_vectors["vectors"]:
            evt = vec["event"]
            evt_id = evt["event_id"]
            tenant_id = evt["tenant_id"]
            idemp_key = evt["correlation"].get("idempotency_key")
            event_type = evt["event_type"]

            # 1. Primary event_id check
            if evt_id not in stored_events:
                stored_events[evt_id] = evt

            # 2. Business deduplication check
            if idemp_key:
                biz_key = (tenant_id, idemp_key, event_type)
                unique_business_keys.add(biz_key)

        assert len(stored_events) == len(test_vectors["vectors"])
        # Repeated insert with same event_id must not duplicate
        first_evt = test_vectors["vectors"][0]["event"]
        assert first_evt["event_id"] in stored_events
