"""
Unit tests for Dual SQLite Database Schemas (Buyer & Gateway).

Tests adherence to specs/payment-state-machine.md and specs/domain-model.md:
  - Buyer Schema: ap_policy, ap_budget_ledger, ap_buyer_intent
  - Gateway Schema: ap_gateway_settlement, ap_gateway_fulfillment, ap_idempotency_cache, ap_audit_event
  - P0-7 Unique nonce replay defense
  - P1-5 Idempotency lease scoping
  - Strict Foreign Keys and Check Constraints on all four orthogonal state enums
"""

from __future__ import annotations

import sqlite3

import pytest
from agentpay.core.state import (
    BudgetLedgerState,
    BuyerIntentRecord,
    FulfillmentState,
    GatewayFulfillmentRecord,
    GatewaySettlementRecord,
    IdempotencyRecord,
    PaymentIntentState,
    SettlementState,
)
from agentpay.policy.db import create_buyer_connection, init_buyer_database
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database


class TestBuyerDatabaseSchema:
    """Tests for Buyer Database (buyer_agentpay.db)."""

    @pytest.fixture
    def conn(self) -> sqlite3.Connection:
        connection = create_buyer_connection(":memory:")
        init_buyer_database(connection)
        return connection

    def test_tables_and_indices_created(self, conn: sqlite3.Connection) -> None:
        cursor = conn.cursor()
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table';")
        tables = {row[0] for row in cursor.fetchall()}
        assert "ap_policy" in tables
        assert "ap_budget_ledger" in tables
        assert "ap_buyer_intent" in tables

        cursor.execute("SELECT name FROM sqlite_master WHERE type='index';")
        indices = {row[0] for row in cursor.fetchall()}
        assert "idx_budget_rolling_sum" in indices
        assert "uq_buyer_intent_idempotency" in indices

    def test_foreign_key_enforcement(self, conn: sqlite3.Connection) -> None:
        # Inserting into ap_budget_ledger with non-existent policy_id must fail
        with pytest.raises(sqlite3.IntegrityError, match="FOREIGN KEY constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_budget_ledger (
                    id, policy_id, wallet_address, intent_id, amount_base_units, state, created_at, expires_at, updated_at
                ) VALUES ('res_1', 'non_existent_policy', '0x123', 'int_1', 100, 'RESERVED', 1000, 2000, 1000);
                """
            )

    def test_budget_ledger_state_check_constraint(self, conn: sqlite3.Connection) -> None:
        # Create valid policy
        conn.execute(
            """
            INSERT INTO ap_policy (
                id, name, wallet_address, max_per_tx_base_units, rolling_budget_24h_base_units, created_at, updated_at
            ) VALUES ('pol_1', 'Test Policy', '0x123', 1000, 10000, 1000, 1000);
            """
        )

        # Valid states: RESERVED, COMMITTED, RELEASED
        for state in (
            BudgetLedgerState.RESERVED.value,
            BudgetLedgerState.COMMITTED.value,
            BudgetLedgerState.RELEASED.value,
        ):
            conn.execute(
                """
                INSERT INTO ap_budget_ledger (
                    id, policy_id, wallet_address, intent_id, amount_base_units, state, created_at, expires_at, updated_at
                ) VALUES (?, 'pol_1', '0x123', ?, 100, ?, 1000, 2000, 1000);
                """,
                (f"res_{state}", f"int_{state}", state),
            )

        # Invalid state fails CHECK constraint
        with pytest.raises(sqlite3.IntegrityError, match="CHECK constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_budget_ledger (
                    id, policy_id, wallet_address, intent_id, amount_base_units, state, created_at, expires_at, updated_at
                ) VALUES ('res_invalid', 'pol_1', '0x123', 'int_invalid', 100, 'PENDING', 1000, 2000, 1000);
                """
            )

    def test_buyer_intent_state_check_constraint(self, conn: sqlite3.Connection) -> None:
        conn.execute(
            """
            INSERT INTO ap_policy (
                id, name, wallet_address, max_per_tx_base_units, rolling_budget_24h_base_units, created_at, updated_at
            ) VALUES ('pol_1', 'Test', '0x123', 1000, 10000, 1000, 1000);
            """
        )

        # Valid states
        valid_states = [
            PaymentIntentState.CREATED.value,
            PaymentIntentState.AUTHORIZED.value,
            PaymentIntentState.PROCESSING.value,
            PaymentIntentState.SIGNATURE_SENT.value,
            PaymentIntentState.COMPLETED.value,
            PaymentIntentState.SUCCEEDED.value,
            PaymentIntentState.REJECTED.value,
            PaymentIntentState.EXPIRED.value,
            PaymentIntentState.CANCELLED.value,
            PaymentIntentState.RECOVERY_HOLD.value,
        ]

        for s in valid_states:
            conn.execute(
                """
                INSERT INTO ap_buyer_intent (
                    intent_id, policy_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                    amount_base_units, status, created_at, updated_at
                ) VALUES (?, 'pol_1', ?, '0x123', '0x456', '0x789', 'eip155:84532', 100, ?, 1000, 1000);
                """,
                (f"intent_{s}", f"key_{s}", s),
            )

        # Invalid status fails
        with pytest.raises(sqlite3.IntegrityError, match="CHECK constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_buyer_intent (
                    intent_id, policy_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                    amount_base_units, status, created_at, updated_at
                ) VALUES ('intent_bad', 'pol_1', 'key_bad', '0x123', '0x456', '0x789', 'eip155:84532', 100, 'UNKNOWN_STATUS', 1000, 1000);
                """
            )

    def test_buyer_intent_idempotency_unique_index(self, conn: sqlite3.Connection) -> None:
        conn.execute(
            """
            INSERT INTO ap_policy (
                id, name, wallet_address, max_per_tx_base_units, rolling_budget_24h_base_units, created_at, updated_at
            ) VALUES ('pol_1', 'Test', '0x123', 1000, 10000, 1000, 1000);
            """
        )
        conn.execute(
            """
            INSERT INTO ap_buyer_intent (
                intent_id, policy_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                amount_base_units, status, created_at, updated_at
            ) VALUES ('intent_1', 'pol_1', 'same_key', '0x123', '0x456', '0x789', 'eip155:84532', 100, 'CREATED', 1000, 1000);
            """
        )

        # Duplicate (payer_address, idempotency_key) must fail
        with pytest.raises(sqlite3.IntegrityError, match="UNIQUE constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_buyer_intent (
                    intent_id, policy_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                    amount_base_units, status, created_at, updated_at
                ) VALUES ('intent_2', 'pol_1', 'same_key', '0x123', '0x456', '0x789', 'eip155:84532', 100, 'CREATED', 1000, 1000);
                """
            )

    def test_buyer_intent_pydantic_record_roundtrip(self) -> None:
        record = BuyerIntentRecord(
            intentId="int_123",
            policyId="pol_1",
            idempotencyKey="idem_xyz",
            payerAddress="0x123",
            payeeAddress="0x456",
            assetAddress="0x789",
            networkId="eip155:84532",
            amountBaseUnits=50000,
            status=PaymentIntentState.PROCESSING,
            nonce="0x" + "a" * 64,
            validAfter=1000,
            validBefore=2000,
            rawSignature="0x" + "b" * 130,
            createdAt=1000,
            updatedAt=1050,
        )
        assert record.intent_id == "int_123"
        assert record.status == PaymentIntentState.PROCESSING
        assert record.amount_base_units == 50000


class TestGatewayDatabaseSchema:
    """Tests for Gateway Database (gateway_agentpay.db)."""

    @pytest.fixture
    def conn(self) -> sqlite3.Connection:
        connection = create_gateway_connection(":memory:")
        init_gateway_database(connection)
        return connection

    def test_tables_and_indices_created(self, conn: sqlite3.Connection) -> None:
        cursor = conn.cursor()
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table';")
        tables = {row[0] for row in cursor.fetchall()}
        assert "ap_gateway_settlement" in tables
        assert "ap_gateway_fulfillment" in tables
        assert "ap_idempotency_cache" in tables
        assert "ap_audit_event" in tables

        cursor.execute("SELECT name FROM sqlite_master WHERE type='index';")
        indices = {row[0] for row in cursor.fetchall()}
        assert "idx_settlement_lookup" in indices
        assert "idx_idempotency_lease" in indices

    def test_p0_7_nonce_replay_unique_constraint(self, conn: sqlite3.Connection) -> None:
        """
        P0-7 Invariant:
        UNIQUE(network_id, asset_address, payer_address, nonce) prevents duplicate
        on-chain clearing submissions of the same EIP-3009 authorization.
        """
        conn.execute(
            """
            INSERT INTO ap_gateway_settlement (
                settlement_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                amount_base_units, nonce, valid_after, valid_before, raw_signature, status, created_at, updated_at
            ) VALUES (
                'stl_1', 'idem_1', '0xpayer', '0xpayee', '0xasset', 'eip155:84532',
                50000, '0xnonce_abc', 1000, 2000, '0xsig', 'SUBMITTED', 1000, 1000
            );
            """
        )

        # Duplicate (network_id, asset_address, payer_address, nonce) must fail
        with pytest.raises(sqlite3.IntegrityError, match="UNIQUE constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_gateway_settlement (
                    settlement_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                    amount_base_units, nonce, valid_after, valid_before, raw_signature, status, created_at, updated_at
                ) VALUES (
                    'stl_2', 'idem_2', '0xpayer', '0xpayee', '0xasset', 'eip155:84532',
                    50000, '0xnonce_abc', 1000, 2000, '0xsig', 'SUBMITTED', 1000, 1000
                );
                """
            )

    def test_p1_5_idempotency_cache_primary_key(self, conn: sqlite3.Connection) -> None:
        """
        P1-5 Invariant:
        PRIMARY KEY (payer_address, idempotency_key) guarantees strict once-and-only-once execution.
        """
        conn.execute(
            """
            INSERT INTO ap_idempotency_cache (
                payer_address, idempotency_key, status, request_hash, locked_until, expires_at, created_at
            ) VALUES ('0xpayer', 'idem_key_1', 'IN_FLIGHT', 'hash123', 2000, 5000, 1000);
            """
        )

        with pytest.raises(sqlite3.IntegrityError, match="UNIQUE constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_idempotency_cache (
                    payer_address, idempotency_key, status, request_hash, locked_until, expires_at, created_at
                ) VALUES ('0xpayer', 'idem_key_1', 'IN_FLIGHT', 'hash456', 2000, 5000, 1000);
                """
            )

    def test_gateway_foreign_key_fulfillment(self, conn: sqlite3.Connection) -> None:
        # Fulfillment referencing non-existent settlement_id must fail
        with pytest.raises(sqlite3.IntegrityError, match="FOREIGN KEY constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_gateway_fulfillment (
                    fulfillment_id, settlement_id, status, created_at, updated_at
                ) VALUES ('ful_1', 'non_existent_settlement', 'PROCESSING', 1000, 1000);
                """
            )

    def test_settlement_status_check_constraint(self, conn: sqlite3.Connection) -> None:
        valid_statuses = [
            SettlementState.NOT_STARTED.value,
            SettlementState.NOT_SUBMITTED.value,
            SettlementState.SUBMITTED.value,
            SettlementState.PENDING.value,
            SettlementState.CONFIRMED.value,
            SettlementState.SUCCESS.value,
            SettlementState.FAILED.value,
            SettlementState.REORGED.value,
            SettlementState.UNKNOWN.value,
        ]

        for s in valid_statuses:
            conn.execute(
                """
                INSERT INTO ap_gateway_settlement (
                    settlement_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                    amount_base_units, nonce, valid_after, valid_before, raw_signature, status, created_at, updated_at
                ) VALUES (
                    ?, ?, '0xpayer', '0xpayee', '0xasset', 'eip155:84532',
                    50000, ?, 1000, 2000, '0xsig', ?, 1000, 1000
                );
                """,
                (f"stl_{s}", f"idem_{s}", f"nonce_{s}", s),
            )

        # Invalid settlement status fails
        with pytest.raises(sqlite3.IntegrityError, match="CHECK constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_gateway_settlement (
                    settlement_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                    amount_base_units, nonce, valid_after, valid_before, raw_signature, status, created_at, updated_at
                ) VALUES (
                    'stl_bad', 'idem_bad', '0xpayer', '0xpayee', '0xasset', 'eip155:84532',
                    50000, 'nonce_bad', 1000, 2000, '0xsig', 'INVALID_STATUS', 1000, 1000
                );
                """
            )

    def test_fulfillment_status_check_constraint(self, conn: sqlite3.Connection) -> None:
        # Create a settlement parent row
        conn.execute(
            """
            INSERT INTO ap_gateway_settlement (
                settlement_id, idempotency_key, payer_address, payee_address, asset_address, network_id,
                amount_base_units, nonce, valid_after, valid_before, raw_signature, status, created_at, updated_at
            ) VALUES (
                'stl_parent', 'idem_p', '0xpayer', '0xpayee', '0xasset', 'eip155:84532',
                50000, 'nonce_p', 1000, 2000, '0xsig', 'SUCCESS', 1000, 1000
            );
            """
        )

        valid_fulfillment_statuses = [
            FulfillmentState.NOT_STARTED.value,
            FulfillmentState.NOT_DELIVERED.value,
            FulfillmentState.PROCESSING.value,
            FulfillmentState.DELIVERING.value,
            FulfillmentState.DELIVERED.value,
            FulfillmentState.FAILED.value,
            FulfillmentState.RECOVERY_REQUIRED.value,
        ]

        for fs in valid_fulfillment_statuses:
            conn.execute(
                """
                INSERT INTO ap_gateway_fulfillment (
                    fulfillment_id, settlement_id, status, created_at, updated_at
                ) VALUES (?, 'stl_parent', ?, 1000, 1000);
                """,
                (f"ful_{fs}", fs),
            )

        # Invalid fulfillment status fails
        with pytest.raises(sqlite3.IntegrityError, match="CHECK constraint failed"):
            conn.execute(
                """
                INSERT INTO ap_gateway_fulfillment (
                    fulfillment_id, settlement_id, status, created_at, updated_at
                ) VALUES ('ful_bad', 'stl_parent', 'BOGUS_STATUS', 1000, 1000);
                """
            )

    def test_gateway_pydantic_records_roundtrip(self) -> None:
        settlement = GatewaySettlementRecord(
            settlementId="stl_123",
            idempotencyKey="idem_key",
            payerAddress="0xpayer",
            payeeAddress="0xpayee",
            assetAddress="0xasset",
            networkId="eip155:84532",
            amountBaseUnits=50000,
            nonce="0x123456",
            validAfter=1000,
            validBefore=2000,
            rawSignature="0xsig",
            status=SettlementState.SUCCESS,
            txHash="0xhash",
            blockNumber=1234567,
        )
        assert settlement.settlement_id == "stl_123"
        assert settlement.status == SettlementState.SUCCESS

        fulfillment = GatewayFulfillmentRecord(
            fulfillmentId="ful_456",
            settlementId="stl_123",
            status=FulfillmentState.DELIVERED,
            upstreamStatusCode=200,
            responseHash="sha256_hash",
            retryCount=0,
            deliveredAt=1050,
        )
        assert fulfillment.fulfillment_id == "ful_456"
        assert fulfillment.status == FulfillmentState.DELIVERED

        idempotency = IdempotencyRecord(
            payerAddress="0xpayer",
            idempotencyKey="idem_key",
            settlementId="stl_123",
            status="COMPLETED",
            requestHash="req_sha256",
            responseStatusCode=200,
            responseHeadersJson='{"Content-Type": "application/json"}',
            responseBodyBlob=b'{"result": "ok"}',
            lockedUntil=2000,
            expiresAt=5000,
        )
        assert idempotency.payer_address == "0xpayer"
        assert idempotency.status == "COMPLETED"
        assert idempotency.response_body_blob == b'{"result": "ok"}'
