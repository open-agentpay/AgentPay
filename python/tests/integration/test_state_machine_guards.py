"""
Unit tests for T4.2: Four-Object State Machine Models, Transition Guards, and Coordinators.

Tests adherence to specs/payment-state-machine.md §2, §3, and §4:
  - Transition guard matrices for Intent, Settlement, Fulfillment, Ledger
  - Anti-Overspend Invariant: SIGNATURE_SENT/RECOVERY_HOLD cannot release or cancel
  - Anti-Rollback Invariant: Upstream 5xx never voids valid settlement
  - BuyerStateCoordinator & GatewayStateCoordinator SQLite atomic operations
  - Composite state whitelist validation
"""

from __future__ import annotations

import sqlite3

import pytest
from agentpay.core.state import (
    BudgetLedgerState,
    BuyerIntentRecord,
    FulfillmentState,
    GatewaySettlementRecord,
    PaymentIntentState,
    SettlementState,
)
from agentpay.policy.db import create_buyer_connection, init_buyer_database
from agentpay.state.buyer_coordinator import BuyerStateCoordinator
from agentpay.state.errors import (
    IllegalStateTransitionError,
    InvalidCompositeStateError,
    OrphanedSettlementRollbackForbiddenError,
    SignatureSentReleaseForbiddenError,
)
from agentpay.state.guards import (
    validate_composite_state,
    validate_fulfillment_transition,
    validate_intent_transition,
    validate_ledger_transition,
    validate_settlement_transition,
)
from agentpay_gateway.state.coordinator import GatewayStateCoordinator
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database


class TestStateTransitionGuards:
    """Tests for transition guard validators and error handling."""

    def test_legal_and_illegal_intent_transitions(self) -> None:
        # Legal
        validate_intent_transition(PaymentIntentState.CREATED, PaymentIntentState.PROCESSING)
        validate_intent_transition(PaymentIntentState.PROCESSING, PaymentIntentState.SIGNATURE_SENT)
        validate_intent_transition(PaymentIntentState.SIGNATURE_SENT, PaymentIntentState.SUCCEEDED)
        validate_intent_transition(PaymentIntentState.SIGNATURE_SENT, PaymentIntentState.RECOVERY_HOLD)

        # Illegal: Terminal state cannot transition
        with pytest.raises(IllegalStateTransitionError) as exc:
            validate_intent_transition(PaymentIntentState.SUCCEEDED, PaymentIntentState.PROCESSING)
        assert exc.value.entity_type == "PaymentIntent"

        # Illegal: Direct jump from CREATED to SUCCEEDED
        with pytest.raises(IllegalStateTransitionError):
            validate_intent_transition(PaymentIntentState.CREATED, PaymentIntentState.SUCCEEDED)

    def test_anti_overspend_intent_guard(self) -> None:
        """SIGNATURE_SENT and RECOVERY_HOLD must NEVER transition to CANCELLED."""
        with pytest.raises(SignatureSentReleaseForbiddenError):
            validate_intent_transition(
                PaymentIntentState.SIGNATURE_SENT,
                PaymentIntentState.CANCELLED,
                intent_id="int_123",
            )

        with pytest.raises(SignatureSentReleaseForbiddenError):
            validate_intent_transition(
                PaymentIntentState.RECOVERY_HOLD,
                PaymentIntentState.CANCELLED,
                intent_id="int_123",
            )

    def test_anti_rollback_settlement_guard(self) -> None:
        """Active settlement cannot be reset to NOT_STARTED."""
        with pytest.raises(OrphanedSettlementRollbackForbiddenError):
            validate_settlement_transition(
                SettlementState.SUBMITTED,
                SettlementState.NOT_STARTED,
                settlement_id="stl_123",
            )

        with pytest.raises(OrphanedSettlementRollbackForbiddenError):
            validate_settlement_transition(
                SettlementState.SUCCESS,
                SettlementState.NOT_STARTED,
                settlement_id="stl_123",
            )

    def test_fulfillment_transitions(self) -> None:
        # Legal flow including recovery retry
        validate_fulfillment_transition(FulfillmentState.NOT_DELIVERED, FulfillmentState.DELIVERING)
        validate_fulfillment_transition(FulfillmentState.DELIVERING, FulfillmentState.RECOVERY_REQUIRED)
        validate_fulfillment_transition(FulfillmentState.RECOVERY_REQUIRED, FulfillmentState.DELIVERING)
        validate_fulfillment_transition(FulfillmentState.DELIVERING, FulfillmentState.DELIVERED)

        # Illegal: DELIVERED cannot transition
        with pytest.raises(IllegalStateTransitionError):
            validate_fulfillment_transition(FulfillmentState.DELIVERED, FulfillmentState.RECOVERY_REQUIRED)

    def test_ledger_transitions(self) -> None:
        validate_ledger_transition(BudgetLedgerState.RESERVED, BudgetLedgerState.COMMITTED)
        validate_ledger_transition(BudgetLedgerState.RESERVED, BudgetLedgerState.RELEASED)

        # Illegal: COMMITTED cannot be released
        with pytest.raises(IllegalStateTransitionError):
            validate_ledger_transition(BudgetLedgerState.COMMITTED, BudgetLedgerState.RELEASED)


class TestCompositeStateWhitelist:
    """Tests for multi-entity composite state validation (specs §4)."""

    def test_healthy_final_composite_state(self) -> None:
        # Healthy final: SUCCEEDED + SUCCESS + DELIVERED + COMMITTED
        validate_composite_state(
            intent_state=PaymentIntentState.SUCCEEDED,
            settlement_state=SettlementState.SUCCESS,
            fulfillment_state=FulfillmentState.DELIVERED,
            ledger_state=BudgetLedgerState.COMMITTED,
        )

    def test_orphaned_settlement_composite_state(self) -> None:
        # Upstream 500: RECOVERY_HOLD + SUCCESS + RECOVERY_REQUIRED + RESERVED
        validate_composite_state(
            intent_state=PaymentIntentState.RECOVERY_HOLD,
            settlement_state=SettlementState.SUCCESS,
            fulfillment_state=FulfillmentState.RECOVERY_REQUIRED,
            ledger_state=BudgetLedgerState.RESERVED,
        )

    def test_anti_overspend_composite_rejection(self) -> None:
        # RECOVERY_HOLD with RELEASED ledger is a fatal violation
        with pytest.raises(InvalidCompositeStateError, match="Anti-Overspend Violation"):
            validate_composite_state(
                intent_state=PaymentIntentState.RECOVERY_HOLD,
                settlement_state=SettlementState.SUCCESS,
                fulfillment_state=FulfillmentState.RECOVERY_REQUIRED,
                ledger_state=BudgetLedgerState.RELEASED,
            )

    def test_success_without_delivery_rejection(self) -> None:
        # Intent SUCCEEDED but fulfillment was not DELIVERED
        with pytest.raises(
            InvalidCompositeStateError,
            match="Intent cannot be SUCCEEDED if resource was not DELIVERED",
        ):
            validate_composite_state(
                intent_state=PaymentIntentState.SUCCEEDED,
                settlement_state=SettlementState.SUCCESS,
                fulfillment_state=FulfillmentState.RECOVERY_REQUIRED,
                ledger_state=BudgetLedgerState.COMMITTED,
            )


class TestBuyerStateCoordinator:
    """Tests for BuyerStateCoordinator atomic transitions and invariants."""

    @pytest.fixture
    def buyer_conn(self) -> sqlite3.Connection:
        conn = create_buyer_connection(":memory:")
        init_buyer_database(conn)
        # Create a test policy
        conn.execute(
            """
            INSERT INTO ap_policy (
                id, name, wallet_address, max_per_tx_base_units, rolling_budget_24h_base_units, created_at, updated_at
            ) VALUES ('pol_test', 'Test Policy', '0xpayer', 100000, 1000000, 1000, 1000);
            """
        )
        return conn

    def test_happy_path_intent_lifecycle(self, buyer_conn: sqlite3.Connection) -> None:
        clock_time = 1000
        coordinator = BuyerStateCoordinator(buyer_conn, clock=lambda: clock_time)

        # 1. Create intent
        intent = BuyerIntentRecord(
            intentId="int_1",
            policyId="pol_test",
            idempotencyKey="idem_1",
            payerAddress="0xpayer",
            payeeAddress="0xpayee",
            assetAddress="0xasset",
            networkId="eip155:84532",
            amountBaseUnits=50000,
            status=PaymentIntentState.CREATED,
            createdAt=1000,
            updatedAt=1000,
        )
        coordinator.create_intent(intent)
        fetched = coordinator.get_intent("int_1")
        assert fetched is not None
        assert fetched.status == PaymentIntentState.CREATED

        # 2. Reserve intent
        res_id = coordinator.reserve_intent("int_1", "pol_test", "0xpayer", 50000)
        assert res_id.startswith("res_")
        assert coordinator.get_intent("int_1").status == PaymentIntentState.PROCESSING
        assert coordinator.get_ledger_state("int_1") == BudgetLedgerState.RESERVED

        # 3. Dispatch on wire
        coordinator.record_dispatch("int_1", raw_signature="0xsig", nonce="0xnonce")
        assert coordinator.get_intent("int_1").status == PaymentIntentState.SIGNATURE_SENT
        assert coordinator.get_ledger_state("int_1") == BudgetLedgerState.RESERVED

        # 4. Complete on 200 OK
        coordinator.complete_intent("int_1")
        assert coordinator.get_intent("int_1").status == PaymentIntentState.SUCCEEDED
        assert coordinator.get_ledger_state("int_1") == BudgetLedgerState.COMMITTED

    def test_pre_flight_cancel_releases_quota(self, buyer_conn: sqlite3.Connection) -> None:
        coordinator = BuyerStateCoordinator(buyer_conn, clock=lambda: 1000)

        intent = BuyerIntentRecord(
            intentId="int_cancel",
            policyId="pol_test",
            idempotencyKey="idem_c",
            payerAddress="0xpayer",
            payeeAddress="0xpayee",
            assetAddress="0xasset",
            networkId="eip155:84532",
            amountBaseUnits=50000,
            status=PaymentIntentState.CREATED,
        )
        coordinator.create_intent(intent)
        coordinator.reserve_intent("int_cancel", "pol_test", "0xpayer", 50000)
        assert coordinator.get_ledger_state("int_cancel") == BudgetLedgerState.RESERVED

        # Cancel before signature dispatched
        coordinator.cancel_intent("int_cancel")
        assert coordinator.get_intent("int_cancel").status == PaymentIntentState.CANCELLED
        assert coordinator.get_ledger_state("int_cancel") == BudgetLedgerState.RELEASED

    def test_anti_overspend_post_dispatch_cancel_forbidden(self, buyer_conn: sqlite3.Connection) -> None:
        coordinator = BuyerStateCoordinator(buyer_conn, clock=lambda: 1000)

        intent = BuyerIntentRecord(
            intentId="int_locked",
            policyId="pol_test",
            idempotencyKey="idem_l",
            payerAddress="0xpayer",
            payeeAddress="0xpayee",
            assetAddress="0xasset",
            networkId="eip155:84532",
            amountBaseUnits=50000,
            status=PaymentIntentState.CREATED,
        )
        coordinator.create_intent(intent)
        coordinator.reserve_intent("int_locked", "pol_test", "0xpayer", 50000)
        coordinator.record_dispatch("int_locked", raw_signature="0xsig", nonce="0xnonce")

        # Attempting cancel AFTER dispatch must raise SignatureSentReleaseForbiddenError
        with pytest.raises(SignatureSentReleaseForbiddenError):
            coordinator.cancel_intent("int_locked")

        # Quota remains strictly RESERVED
        assert coordinator.get_ledger_state("int_locked") == BudgetLedgerState.RESERVED

    def test_upstream_500_recovery_hold_and_reconcile(self, buyer_conn: sqlite3.Connection) -> None:
        coordinator = BuyerStateCoordinator(buyer_conn, clock=lambda: 1000)

        intent = BuyerIntentRecord(
            intentId="int_hold",
            policyId="pol_test",
            idempotencyKey="idem_h",
            payerAddress="0xpayer",
            payeeAddress="0xpayee",
            assetAddress="0xasset",
            networkId="eip155:84532",
            amountBaseUnits=50000,
            status=PaymentIntentState.CREATED,
        )
        coordinator.create_intent(intent)
        coordinator.reserve_intent("int_hold", "pol_test", "0xpayer", 50000)
        coordinator.record_dispatch("int_hold", raw_signature="0xsig", nonce="0xnonce")

        # Upstream returns 500 -> transition to RECOVERY_HOLD
        coordinator.hold_intent("int_hold", error_message="Upstream 500 Internal Error")
        assert coordinator.get_intent("int_hold").status == PaymentIntentState.RECOVERY_HOLD
        assert coordinator.get_ledger_state("int_hold") == BudgetLedgerState.RESERVED

        # Ordinary cancel is still forbidden!
        with pytest.raises(SignatureSentReleaseForbiddenError):
            coordinator.cancel_intent("int_hold")

        # Reconciler can release only after on-chain unspent proof
        coordinator.reconcile_release("int_hold", reason="Nonce verified unspent after validBefore + 60s")
        assert coordinator.get_intent("int_hold").status == PaymentIntentState.REJECTED
        assert coordinator.get_ledger_state("int_hold") == BudgetLedgerState.RELEASED


class TestGatewayStateCoordinator:
    """Tests for GatewayStateCoordinator atomic transitions and non-rollback invariant."""

    @pytest.fixture
    def gateway_conn(self) -> sqlite3.Connection:
        conn = create_gateway_connection(":memory:")
        init_gateway_database(conn)
        return conn

    def test_gateway_happy_path(self, gateway_conn: sqlite3.Connection) -> None:
        coordinator = GatewayStateCoordinator(gateway_conn, clock=lambda: 1000)

        settlement = GatewaySettlementRecord(
            settlementId="stl_gw_1",
            idempotencyKey="idem_gw_1",
            payerAddress="0xpayer",
            payeeAddress="0xpayee",
            assetAddress="0xasset",
            networkId="eip155:84532",
            amountBaseUnits=50000,
            nonce="0xnonce1",
            validAfter=900,
            validBefore=2000,
            rawSignature="0xsig",
            status=SettlementState.SUBMITTED,
        )

        ful_id = coordinator.register_inbound_payment(settlement, request_hash="req_hash_1")
        assert ful_id.startswith("ful_")

        # Start fulfillment
        coordinator.start_fulfillment(ful_id)

        # Complete fulfillment
        coordinator.complete_fulfillment(
            ful_id,
            upstream_status_code=200,
            response_body_blob=b'{"result": "delivered"}',
            response_headers_json='{"Content-Type": "application/json"}',
        )

        # Confirm settlement
        coordinator.transition_settlement("stl_gw_1", SettlementState.SUCCESS, tx_hash="0xtx1")

        # Check idempotency: should be CACHE HIT!
        eval_res = coordinator.check_idempotency("0xpayer", "idem_gw_1")
        assert eval_res.is_cache_hit is True
        assert eval_res.cached_body_blob == b'{"result": "delivered"}'
        assert eval_res.cached_status_code == 200

    def test_anti_rollback_invariant_on_upstream_500(self, gateway_conn: sqlite3.Connection) -> None:
        """
        Crucial Safety Invariant:
        When upstream returns 500, fulfillment enters RECOVERY_REQUIRED,
        but settlement remains SUCCESS (never rolled back)!
        Subsequent request with same idempotency key is evaluated as FREE REPLAY ELIGIBLE.
        """
        coordinator = GatewayStateCoordinator(gateway_conn, clock=lambda: 1000)

        settlement = GatewaySettlementRecord(
            settlementId="stl_gw_500",
            idempotencyKey="idem_gw_500",
            payerAddress="0xpayer",
            payeeAddress="0xpayee",
            assetAddress="0xasset",
            networkId="eip155:84532",
            amountBaseUnits=50000,
            nonce="0xnonce500",
            validAfter=900,
            validBefore=2000,
            rawSignature="0xsig",
            status=SettlementState.SUCCESS,  # Settlement was confirmed by Facilitator
        )

        ful_id = coordinator.register_inbound_payment(settlement, request_hash="req_hash_500")
        coordinator.start_fulfillment(ful_id)

        # Upstream fails with 500
        coordinator.fail_fulfillment(ful_id, upstream_status_code=500, error_message="LLM timeout")

        # Verify Settlement status is STILL SUCCESS (NOT rolled back!)
        cur = gateway_conn.cursor()
        cur.execute("SELECT status FROM ap_gateway_settlement WHERE settlement_id = 'stl_gw_500';")
        assert cur.fetchone()[0] == SettlementState.SUCCESS.value

        # Verify Fulfillment status is RECOVERY_REQUIRED
        cur.execute(
            "SELECT status, retry_count FROM ap_gateway_fulfillment WHERE fulfillment_id = ?;",
            (ful_id,),
        )
        row = cur.fetchone()
        assert row[0] == FulfillmentState.RECOVERY_REQUIRED.value
        assert row[1] == 1  # retry count incremented

        # Check Idempotency: Must be FREE REPLAY ELIGIBLE!
        eval_res = coordinator.check_idempotency("0xpayer", "idem_gw_500")
        assert eval_res.is_free_replay is True
        assert eval_res.settlement_id == "stl_gw_500"
        assert eval_res.fulfillment_id == ful_id
