"""
Unit and Integration tests for T4.3: Reconciliation Engine and Recovery.

Tests:
  - BuyerReconciler:
      * Commits quota on used nonce (settled on-chain)
      * Retains RESERVED if unspent nonce is still within validBefore + safe_buffer window
      * Releases RESERVED to RELEASED only when unspent nonce is beyond validBefore + safe_buffer
      * Fails closed on UNKNOWN / probe network failure
      * Transitions timed-out SIGNATURE_SENT to RECOVERY_HOLD
  - GatewayReconciler:
      * Confirms settlement and runs compensating upstream delivery (2xx)
      * Preserves CONFIRMED settlement when upstream compensation fails (Anti-Rollback)
      * Moves to UNKNOWN on probe network split, recovers on network heal
      * Waits for validBefore window before marking settlement terminal FAILED
  - FacilitatorSettlementProbe:
      * Integration with MockFacilitatorService via FacilitatorClient
  - End-to-End Idempotent Free Replay:
      * Upstream 500 hold -> free retry -> 200 OK delivery -> ledger committed
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
    PaymentIntentState,
    SettlementState,
)
from agentpay.policy.db import create_buyer_connection, init_buyer_database
from agentpay.state.buyer_coordinator import BuyerStateCoordinator
from agentpay.state.reconciliation import (
    BuyerReconciler,
    BuyerReconciliationOutcome,
    FacilitatorSettlementProbe,
    MockSettlementProbe,
    NonceStatus,
)
from agentpay_gateway.facilitator.client import FacilitatorClient
from agentpay_gateway.facilitator.service import MockFacilitatorService
from agentpay_gateway.state.coordinator import GatewayStateCoordinator
from agentpay_gateway.state.db import create_gateway_connection, init_gateway_database
from agentpay_gateway.state.reconciliation import (
    GatewayReconciler,
    GatewayReconciliationOutcome,
)


@pytest.fixture
def buyer_db() -> sqlite3.Connection:
    conn = create_buyer_connection(":memory:")
    init_buyer_database(conn)
    conn.execute(
        """
        INSERT INTO ap_policy (
            id, name, wallet_address, max_per_tx_base_units, rolling_budget_24h_base_units, created_at, updated_at
        ) VALUES ('pol_default', 'Default Policy', '0x1111111111111111111111111111111111111111', 1000000, 10000000, 1000, 1000);
        """
    )
    return conn


@pytest.fixture
def gateway_db() -> sqlite3.Connection:
    conn = create_gateway_connection(":memory:")
    init_gateway_database(conn)
    return conn


class TestBuyerReconciliation:
    """Tests for BuyerReconciler auditing and budget quota recovery."""

    def test_reconcile_used_nonce_commits_ledger(self, buyer_db: sqlite3.Connection) -> None:
        """If nonce was confirmed used on-chain, finalize intent and commit quota."""
        simulated_time = 1_000
        coordinator = BuyerStateCoordinator(buyer_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        reconciler = BuyerReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
            safe_buffer_seconds=60,
        )

        intent_id = "cin_test_used_1"
        record = BuyerIntentRecord(
            intentId=intent_id,
            policyId="pol_default",
            idempotencyKey="idem_used_1",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=50_000,
            status=PaymentIntentState.CREATED,
            nonce="0xaaa111",
            validAfter=0,
            validBefore=1_200,
            createdAt=simulated_time,
            updatedAt=simulated_time,
        )
        coordinator.create_intent(record)
        coordinator.reserve_intent(
            intent_id,
            "pol_default",
            record.payer_address,
            50_000,
        )
        coordinator.record_dispatch(intent_id)
        # Upstream failed, enter RECOVERY_HOLD
        coordinator.hold_intent(intent_id)

        assert coordinator.get_ledger_state(intent_id) == BudgetLedgerState.RESERVED

        # Probe confirms nonce was used on-chain
        probe.set_nonce(
            record.network_id,
            record.asset_address,
            record.payer_address,
            record.nonce,
            NonceStatus.USED,
            tx_hash="0xdeadbeef123",
        )

        report = reconciler.reconcile_intent(intent_id)

        assert report.outcome == BuyerReconciliationOutcome.COMMITTED
        assert report.new_intent_status == PaymentIntentState.SUCCEEDED
        assert report.ledger_state == BudgetLedgerState.COMMITTED
        assert coordinator.get_ledger_state(intent_id) == BudgetLedgerState.COMMITTED

    def test_reconcile_unspent_nonce_before_expiry_preserves_reserved(self, buyer_db: sqlite3.Connection) -> None:
        """
        CLAUDE.md Invariant:
        Even if nonce is currently UNSPENT, do NOT release quota if validBefore + buffer is not reached!
        """
        simulated_time = 1_000
        coordinator = BuyerStateCoordinator(buyer_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        reconciler = BuyerReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
            safe_buffer_seconds=60,
        )

        intent_id = "cin_test_unspent_premature"
        valid_before = 1_100  # Expires at 1100 + 60 = 1160
        record = BuyerIntentRecord(
            intentId=intent_id,
            policyId="pol_default",
            idempotencyKey="idem_premature",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=20_000,
            status=PaymentIntentState.CREATED,
            nonce="0xaaa222",
            validAfter=0,
            validBefore=valid_before,
            createdAt=simulated_time,
            updatedAt=simulated_time,
        )
        coordinator.create_intent(record)
        coordinator.reserve_intent(intent_id, "pol_default", record.payer_address, 20_000)
        coordinator.record_dispatch(intent_id)
        coordinator.hold_intent(intent_id)

        # Nonce is unspent
        probe.set_nonce(
            record.network_id,
            record.asset_address,
            record.payer_address,
            record.nonce,
            NonceStatus.UNUSED,
        )

        # Attempt reconcile at t=1050 (still < 1160)
        simulated_time = 1_050
        report = reconciler.reconcile_intent(intent_id)

        assert report.outcome == BuyerReconciliationOutcome.HELD
        assert report.new_intent_status == PaymentIntentState.RECOVERY_HOLD
        assert report.ledger_state == BudgetLedgerState.RESERVED
        assert coordinator.get_ledger_state(intent_id) == BudgetLedgerState.RESERVED

    def test_reconcile_unspent_nonce_after_expiry_releases_quota(self, buyer_db: sqlite3.Connection) -> None:
        """
        Once now > validBefore + 60s AND nonce is UNUSED, safely release the quota.
        """
        simulated_time = 1_000
        coordinator = BuyerStateCoordinator(buyer_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        reconciler = BuyerReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
            safe_buffer_seconds=60,
        )

        intent_id = "cin_test_unspent_safe"
        valid_before = 1_100  # Expires at 1100 + 60 = 1160
        record = BuyerIntentRecord(
            intentId=intent_id,
            policyId="pol_default",
            idempotencyKey="idem_safe",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=20_000,
            status=PaymentIntentState.CREATED,
            nonce="0xaaa333",
            validAfter=0,
            validBefore=valid_before,
            createdAt=simulated_time,
            updatedAt=simulated_time,
        )
        coordinator.create_intent(record)
        coordinator.reserve_intent(intent_id, "pol_default", record.payer_address, 20_000)
        coordinator.record_dispatch(intent_id)
        coordinator.hold_intent(intent_id)

        probe.set_nonce(
            record.network_id,
            record.asset_address,
            record.payer_address,
            record.nonce,
            NonceStatus.UNUSED,
        )

        # Advance clock to 1161 (beyond cutoff)
        simulated_time = 1_161
        report = reconciler.reconcile_intent(intent_id)

        assert report.outcome == BuyerReconciliationOutcome.RELEASED
        assert report.new_intent_status == PaymentIntentState.REJECTED
        assert report.ledger_state == BudgetLedgerState.RELEASED
        assert coordinator.get_ledger_state(intent_id) == BudgetLedgerState.RELEASED

    def test_reconcile_unknown_probe_fails_closed(self, buyer_db: sqlite3.Connection) -> None:
        """If probe returns UNKNOWN or network splits, fail-closed preserve RESERVED."""
        simulated_time = 2_000
        coordinator = BuyerStateCoordinator(buyer_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        probe.raise_on_probe = True  # Network down
        reconciler = BuyerReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
            safe_buffer_seconds=60,
        )

        intent_id = "cin_test_unknown_probe"
        record = BuyerIntentRecord(
            intentId=intent_id,
            policyId="pol_default",
            idempotencyKey="idem_fail_closed",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=30_000,
            status=PaymentIntentState.CREATED,
            nonce="0xaaa444",
            validAfter=0,
            validBefore=1_000,
            createdAt=1_000,
            updatedAt=1_000,
        )
        coordinator.create_intent(record)
        coordinator.reserve_intent(intent_id, "pol_default", record.payer_address, 30_000)
        coordinator.record_dispatch(intent_id)
        coordinator.hold_intent(intent_id)

        report = reconciler.reconcile_intent(intent_id)

        assert report.outcome == BuyerReconciliationOutcome.SKIPPED_UNKNOWN
        assert report.new_intent_status == PaymentIntentState.RECOVERY_HOLD
        assert coordinator.get_ledger_state(intent_id) == BudgetLedgerState.RESERVED

    def test_reconcile_timed_out_signature_sent_transitions_to_recovery_hold(
        self, buyer_db: sqlite3.Connection
    ) -> None:
        """Stuck in-flight dispatch past timeout is placed in RECOVERY_HOLD."""
        simulated_time = 1_000
        coordinator = BuyerStateCoordinator(buyer_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        reconciler = BuyerReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
            in_flight_timeout_seconds=30,
            safe_buffer_seconds=60,
        )

        intent_id = "cin_test_stuck_sent"
        record = BuyerIntentRecord(
            intentId=intent_id,
            policyId="pol_default",
            idempotencyKey="idem_stuck",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=10_000,
            status=PaymentIntentState.CREATED,
            nonce="0xaaa555",
            validAfter=0,
            validBefore=2_000,
            createdAt=simulated_time,
            updatedAt=simulated_time,
        )
        coordinator.create_intent(record)
        coordinator.reserve_intent(intent_id, "pol_default", record.payer_address, 10_000)
        coordinator.record_dispatch(intent_id)  # status -> SIGNATURE_SENT

        # Advance time by 35s (> 30s timeout)
        simulated_time = 1_035
        report = reconciler.reconcile_intent(intent_id)

        # Status moved to RECOVERY_HOLD, kept held because 1035 <= 2060
        assert report.previous_intent_status == PaymentIntentState.RECOVERY_HOLD
        assert report.new_intent_status == PaymentIntentState.RECOVERY_HOLD
        assert coordinator.get_ledger_state(intent_id) == BudgetLedgerState.RESERVED

    def test_reconcile_all_held_batch(self, buyer_db: sqlite3.Connection) -> None:
        """Batch reconciliation processes multiple held intents."""
        simulated_time = 2_000
        coordinator = BuyerStateCoordinator(buyer_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        reconciler = BuyerReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
            safe_buffer_seconds=60,
        )

        # Intent 1: settled on-chain -> commits
        rec1 = BuyerIntentRecord(
            intentId="cin_batch_1",
            policyId="pol_default",
            idempotencyKey="idem_batch_1",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=10_000,
            status=PaymentIntentState.CREATED,
            nonce="0xbatch_1",
            validBefore=1_500,
        )
        coordinator.create_intent(rec1)
        coordinator.reserve_intent("cin_batch_1", "pol_default", rec1.payer_address, 10_000)
        coordinator.record_dispatch("cin_batch_1")
        coordinator.hold_intent("cin_batch_1")
        probe.set_nonce(
            rec1.network_id,
            rec1.asset_address,
            rec1.payer_address,
            rec1.nonce,
            NonceStatus.USED,
        )

        # Intent 2: expired unspent -> releases
        rec2 = BuyerIntentRecord(
            intentId="cin_batch_2",
            policyId="pol_default",
            idempotencyKey="idem_batch_2",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=20_000,
            status=PaymentIntentState.CREATED,
            nonce="0xbatch_2",
            validBefore=1_500,  # 1500 + 60 < 2000
        )
        coordinator.create_intent(rec2)
        coordinator.reserve_intent("cin_batch_2", "pol_default", rec2.payer_address, 20_000)
        coordinator.record_dispatch("cin_batch_2")
        coordinator.hold_intent("cin_batch_2")
        probe.set_nonce(
            rec2.network_id,
            rec2.asset_address,
            rec2.payer_address,
            rec2.nonce,
            NonceStatus.UNUSED,
        )

        reports = reconciler.reconcile_all_held()
        assert len(reports) == 2
        outcomes = {r.intent_id: r.outcome for r in reports}
        assert outcomes["cin_batch_1"] == BuyerReconciliationOutcome.COMMITTED
        assert outcomes["cin_batch_2"] == BuyerReconciliationOutcome.RELEASED


class TestGatewayReconciliation:
    """Tests for GatewayReconciler auditing and upstream compensating fulfillment."""

    def test_reconcile_confirmed_settlement_upstream_compensate_success(self, gateway_db: sqlite3.Connection) -> None:
        """When settlement is confirmed and upstream compensation succeeds (2xx), mark DELIVERED."""
        simulated_time = 1_000
        coordinator = GatewayStateCoordinator(gateway_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()

        # Compensating upstream executor returns 200 OK
        def mock_executor(
            fulfillment: GatewayFulfillmentRecord,
        ) -> tuple[int, bytes, dict[str, str] | None]:
            return 200, b'{"result":"success"}', {"Content-Type": "application/json"}

        reconciler = GatewayReconciler(
            coordinator,
            probe,
            upstream_executor=mock_executor,
            clock=lambda: simulated_time,
        )

        settlement_id = "stl_test_comp_success"
        settlement = GatewaySettlementRecord(
            settlementId=settlement_id,
            idempotencyKey="idem_comp_1",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=50_000,
            nonce="0xbbb111",
            validAfter=0,
            validBefore=2_000,
            rawSignature="0x" + "aa" * 65,
            status=SettlementState.SUBMITTED,
        )
        ful_id = coordinator.register_inbound_payment(settlement)
        # Upstream failed previously
        coordinator.start_fulfillment(ful_id)
        coordinator.fail_fulfillment(ful_id, upstream_status_code=500)

        # Facilitator probe reports CONFIRMED
        probe.set_settlement(
            settlement_id,
            SettlementState.CONFIRMED,
            tx_hash="0x9999999999",
        )

        report = reconciler.reconcile_settlement(settlement_id)

        assert report.outcome == GatewayReconciliationOutcome.SETTLED_AND_DELIVERED
        assert report.new_settlement_status == SettlementState.CONFIRMED
        assert report.fulfillment_status == FulfillmentState.DELIVERED
        assert report.tx_hash == "0x9999999999"

        # Verify idempotency cache contains cached response
        idem_eval = coordinator.check_idempotency(settlement.payer_address, settlement.idempotency_key)
        assert idem_eval.is_cache_hit is True
        assert idem_eval.cached_body_blob == b'{"result":"success"}'

    def test_reconcile_confirmed_settlement_upstream_compensate_failure(self, gateway_db: sqlite3.Connection) -> None:
        """
        ANTI-ROLLBACK INVARIANT:
        When upstream compensation fails (500), settlement remains CONFIRMED.
        Fulfillment remains RECOVERY_REQUIRED and eligible for free client replay.
        """
        simulated_time = 1_000
        coordinator = GatewayStateCoordinator(gateway_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()

        # Compensating upstream returns 500 error
        def mock_failing_executor(
            fulfillment: GatewayFulfillmentRecord,
        ) -> tuple[int, bytes, dict[str, str] | None]:
            return 500, b'{"error":"downstream error"}', None

        reconciler = GatewayReconciler(
            coordinator,
            probe,
            upstream_executor=mock_failing_executor,
            clock=lambda: simulated_time,
        )

        settlement_id = "stl_test_comp_fail"
        settlement = GatewaySettlementRecord(
            settlementId=settlement_id,
            idempotencyKey="idem_comp_fail",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=50_000,
            nonce="0xbbb222",
            validAfter=0,
            validBefore=2_000,
            rawSignature="0x" + "aa" * 65,
            status=SettlementState.SUBMITTED,
        )
        ful_id = coordinator.register_inbound_payment(settlement)
        coordinator.start_fulfillment(ful_id)
        coordinator.fail_fulfillment(ful_id, upstream_status_code=500)

        probe.set_settlement(
            settlement_id,
            SettlementState.CONFIRMED,
            tx_hash="0x88888888",
        )

        report = reconciler.reconcile_settlement(settlement_id)

        assert report.outcome == GatewayReconciliationOutcome.SETTLED_FULFILLMENT_RECOVERY_REQUIRED
        assert report.new_settlement_status == SettlementState.CONFIRMED
        assert report.fulfillment_status == FulfillmentState.RECOVERY_REQUIRED

        # Assert free replay is preserved
        idem_eval = coordinator.check_idempotency(settlement.payer_address, settlement.idempotency_key)
        assert idem_eval.is_free_replay is True

    def test_reconcile_probe_unknown_network_split_and_heal(self, gateway_db: sqlite3.Connection) -> None:
        """When probe encounters network split, settlement moves to UNKNOWN, then recovers."""
        simulated_time = 1_000
        coordinator = GatewayStateCoordinator(gateway_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        reconciler = GatewayReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
        )

        settlement_id = "stl_test_split"
        settlement = GatewaySettlementRecord(
            settlementId=settlement_id,
            idempotencyKey="idem_split",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=10_000,
            nonce="0xbbb333",
            validAfter=0,
            validBefore=2_000,
            rawSignature="0x" + "aa" * 65,
            status=SettlementState.SUBMITTED,
        )
        coordinator.register_inbound_payment(settlement)

        # 1. Network split
        probe.raise_on_probe = True
        rep1 = reconciler.reconcile_settlement(settlement_id)
        assert rep1.outcome == GatewayReconciliationOutcome.SKIPPED_UNKNOWN
        assert rep1.new_settlement_status == SettlementState.UNKNOWN

        # 2. Network heals
        probe.raise_on_probe = False
        probe.set_settlement(
            settlement_id,
            SettlementState.CONFIRMED,
            tx_hash="0xhealed",
        )
        rep2 = reconciler.reconcile_settlement(settlement_id)
        assert rep2.outcome == GatewayReconciliationOutcome.SETTLED_PENDING_CLIENT_RETRY
        assert rep2.new_settlement_status == SettlementState.CONFIRMED
        assert rep2.tx_hash == "0xhealed"

    def test_reconcile_failed_settlement_before_vs_after_expiry(self, gateway_db: sqlite3.Connection) -> None:
        """Settlement marked FAILED only after validBefore + 60s."""
        simulated_time = 1_000
        coordinator = GatewayStateCoordinator(gateway_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        reconciler = GatewayReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
            safe_buffer_seconds=60,
        )

        settlement_id = "stl_test_failed_timing"
        valid_before = 1_100  # Cutoff = 1160
        settlement = GatewaySettlementRecord(
            settlementId=settlement_id,
            idempotencyKey="idem_fail_timing",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=10_000,
            nonce="0xbbb444",
            validAfter=0,
            validBefore=valid_before,
            rawSignature="0x" + "aa" * 65,
            status=SettlementState.SUBMITTED,
        )
        coordinator.register_inbound_payment(settlement)

        probe.set_settlement(
            settlement_id,
            SettlementState.FAILED,
            error="Simulation revert",
        )

        # At t=1050 (<= 1160), move to UNKNOWN waiting expiration
        simulated_time = 1_050
        rep_early = reconciler.reconcile_settlement(settlement_id)
        assert rep_early.outcome == GatewayReconciliationOutcome.UNKNOWN_WAITING_EXPIRATION
        assert rep_early.new_settlement_status == SettlementState.UNKNOWN

        # At t=1161 (> 1160), safely mark FAILED
        simulated_time = 1_161
        rep_expired = reconciler.reconcile_settlement(settlement_id)
        assert rep_expired.outcome == GatewayReconciliationOutcome.FAILED_EXPIRED
        assert rep_expired.new_settlement_status == SettlementState.FAILED

    def test_reconcile_all_unresolved_batch(self, gateway_db: sqlite3.Connection) -> None:
        """Batch reconciliation processes multiple unresolved gateway settlements."""
        simulated_time = 1_000
        coordinator = GatewayStateCoordinator(gateway_db, clock=lambda: simulated_time)
        probe = MockSettlementProbe()
        reconciler = GatewayReconciler(
            coordinator,
            probe,
            clock=lambda: simulated_time,
        )

        # Settlement 1: SUBMITTED -> CONFIRMED
        s1 = GatewaySettlementRecord(
            settlementId="stl_batch_1",
            idempotencyKey="idem_batch_1",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=10_000,
            nonce="0xgateway_batch_1",
            validAfter=0,
            validBefore=2_000,
            rawSignature="0x" + "aa" * 65,
            status=SettlementState.SUBMITTED,
        )
        coordinator.register_inbound_payment(s1)
        probe.set_settlement("stl_batch_1", SettlementState.CONFIRMED, tx_hash="0xhash_b1")

        # Settlement 2: PENDING -> CONFIRMED
        s2 = GatewaySettlementRecord(
            settlementId="stl_batch_2",
            idempotencyKey="idem_batch_2",
            payerAddress="0x1111111111111111111111111111111111111111",
            payeeAddress="0x2222222222222222222222222222222222222222",
            assetAddress="0x036cb53842c5426634e7929541ec2318f3dcf7e8",
            networkId="eip155:84532",
            amountBaseUnits=20_000,
            nonce="0xgateway_batch_2",
            validAfter=0,
            validBefore=2_000,
            rawSignature="0x" + "aa" * 65,
            status=SettlementState.SUBMITTED,
        )
        coordinator.register_inbound_payment(s2)
        coordinator.transition_settlement("stl_batch_2", SettlementState.PENDING)
        probe.set_settlement("stl_batch_2", SettlementState.CONFIRMED, tx_hash="0xhash_b2")

        reports = reconciler.reconcile_all_unresolved()
        assert len(reports) == 2
        statuses = {r.settlement_id: r.new_settlement_status for r in reports}
        assert statuses["stl_batch_1"] == SettlementState.CONFIRMED
        assert statuses["stl_batch_2"] == SettlementState.CONFIRMED


class TestFacilitatorProbeIntegration:
    """Integration tests connecting FacilitatorSettlementProbe to MockFacilitatorService."""

    def test_facilitator_settlement_probe_real_queries(self) -> None:
        service = MockFacilitatorService()
        transport = service.create_sync_transport()
        client = FacilitatorClient(
            base_url="http://mock-facilitator",
            transport=transport,
        )
        probe = FacilitatorSettlementProbe(client)

        network = "eip155:84532"
        asset = "0x036cb53842c5426634e7929541ec2318f3dcf7e8"
        payer = "0x1111111111111111111111111111111111111111"
        nonce = "0x12345678"

        # Initially nonce is unused
        res_nonce = probe.probe_nonce(network, asset, payer, nonce)
        assert res_nonce.status == NonceStatus.UNUSED

        # Unknown settlement
        res_stl = probe.probe_settlement("stl_unknown")
        assert res_stl.status == SettlementState.FAILED


class TestEndToEndFreeReplayCycle:
    """
    End-to-End integration test covering the complete cycle:
      Upstream 500 -> RECOVERY_HOLD & Free Replay -> Upstream 200 -> Final COMMIT.
    """

    def test_full_free_replay_flow(self, buyer_db: sqlite3.Connection, gateway_db: sqlite3.Connection) -> None:
        clock_time = 1_000
        b_coord = BuyerStateCoordinator(buyer_db, clock=lambda: clock_time)
        g_coord = GatewayStateCoordinator(gateway_db, clock=lambda: clock_time)

        intent_id = "cin_e2e_replay"
        idempotency_key = "idem_e2e_key_1"
        payer = "0x1111111111111111111111111111111111111111"
        payee = "0x2222222222222222222222222222222222222222"
        asset = "0x036cb53842c5426634e7929541ec2318f3dcf7e8"
        network = "eip155:84532"
        amount = 100_000
        nonce = "0xeeff0011"

        # 1. Buyer creates intent and reserves budget
        b_record = BuyerIntentRecord(
            intentId=intent_id,
            policyId="pol_default",
            idempotencyKey=idempotency_key,
            payerAddress=payer,
            payeeAddress=payee,
            assetAddress=asset,
            networkId=network,
            amountBaseUnits=amount,
            nonce=nonce,
            validBefore=clock_time + 300,
        )
        b_coord.create_intent(b_record)
        b_coord.reserve_intent(intent_id, "pol_default", payer, amount)
        b_coord.record_dispatch(intent_id)

        assert b_coord.get_ledger_state(intent_id) == BudgetLedgerState.RESERVED

        # 2. Gateway registers payment
        settlement_id = "stl_e2e_replay"
        g_record = GatewaySettlementRecord(
            settlementId=settlement_id,
            idempotencyKey=idempotency_key,
            payerAddress=payer,
            payeeAddress=payee,
            assetAddress=asset,
            networkId=network,
            amountBaseUnits=amount,
            nonce=nonce,
            validAfter=0,
            validBefore=clock_time + 300,
            rawSignature="0x" + "bb" * 65,
            status=SettlementState.SUBMITTED,
        )
        ful_id = g_coord.register_inbound_payment(g_record)

        # 3. Gateway settles on-chain, but upstream API returns 500 Internal Server Error
        g_coord.transition_settlement(settlement_id, SettlementState.SUCCESS, tx_hash="0xhash123")
        g_coord.start_fulfillment(ful_id)
        g_coord.fail_fulfillment(ful_id, upstream_status_code=500)

        # 4. Buyer receives 500 error; places intent in RECOVERY_HOLD
        b_coord.hold_intent(intent_id)
        # Quota MUST remain locked in RESERVED!
        assert b_coord.get_ledger_state(intent_id) == BudgetLedgerState.RESERVED

        # 5. Buyer retries with SAME idempotency key
        eval_replay = g_coord.check_idempotency(payer, idempotency_key)
        assert eval_replay.is_fresh is False
        assert eval_replay.is_free_replay is True
        assert eval_replay.settlement_id == settlement_id

        # Zero new charge! Gateway directly forwards to upstream, which now returns 200 OK
        g_coord.start_fulfillment(ful_id)
        g_coord.complete_fulfillment(
            ful_id,
            upstream_status_code=200,
            response_body_blob=b'{"status":"ok","tokens":42}',
            response_headers_json='{"Content-Type":"application/json"}',
        )

        # 6. Buyer receives 200 OK and completes intent
        b_coord.complete_intent(intent_id)

        # Final assertions:
        final_intent = b_coord.get_intent(intent_id)
        assert final_intent is not None
        assert final_intent.status == PaymentIntentState.SUCCEEDED
        assert b_coord.get_ledger_state(intent_id) == BudgetLedgerState.COMMITTED

        final_settlement = g_coord.get_settlement(settlement_id)
        assert final_settlement is not None
        assert final_settlement.status == SettlementState.SUCCESS

        final_fulfillment = g_coord.get_fulfillment(ful_id)
        assert final_fulfillment is not None
        assert final_fulfillment.status == FulfillmentState.DELIVERED
