"""
Gateway Reconciliation Engine for Seller Gateway.

Reconciles in-flight or ambiguous settlements (SUBMITTED, PENDING, UNKNOWN) and
ensures fulfillment recovery without double-spending.
"""

from __future__ import annotations

import enum
from collections.abc import Callable

from agentpay.core.state import (
    FulfillmentState,
    GatewayFulfillmentRecord,
    SettlementState,
)
from agentpay.state.reconciliation import (
    ISettlementProbe,
)
from pydantic import BaseModel, ConfigDict, Field

from agentpay_gateway.state.coordinator import GatewayStateCoordinator


class GatewayReconciliationOutcome(str, enum.Enum):
    """Outcome of reconciling a gateway settlement and upstream fulfillment."""

    SETTLED_AND_DELIVERED = "SETTLED_AND_DELIVERED"
    SETTLED_PENDING_CLIENT_RETRY = "SETTLED_PENDING_CLIENT_RETRY"
    SETTLED_FULFILLMENT_RECOVERY_REQUIRED = "SETTLED_FULFILLMENT_RECOVERY_REQUIRED"
    ALREADY_DELIVERED = "ALREADY_DELIVERED"
    UNKNOWN_WAITING_EXPIRATION = "UNKNOWN_WAITING_EXPIRATION"
    FAILED_EXPIRED = "FAILED_EXPIRED"
    SKIPPED_UNKNOWN = "SKIPPED_UNKNOWN"


class GatewayReconciliationReport(BaseModel):
    """Audit report for an individual gateway settlement reconciliation attempt."""

    model_config = ConfigDict(frozen=True)

    settlement_id: str = Field(..., alias="settlementId")
    previous_settlement_status: SettlementState = Field(..., alias="previousSettlementStatus")
    new_settlement_status: SettlementState = Field(..., alias="newSettlementStatus")
    fulfillment_status: FulfillmentState = Field(..., alias="fulfillmentStatus")
    outcome: GatewayReconciliationOutcome
    tx_hash: str | None = Field(default=None, alias="txHash")
    reason: str


# Type for compensating upstream execution callback:
# (fulfillment_record) -> (http_status_code, response_body_bytes, response_headers_dict)
UpstreamExecutor = Callable[
    [GatewayFulfillmentRecord],
    tuple[int, bytes, dict[str, str] | None],
]


class GatewayReconciler:
    """
    Reconciliation worker for Seller Gateway.

    Reconciles in-flight or ambiguous settlements (SUBMITTED, PENDING, UNKNOWN).
    If settlement is confirmed on-chain/Facilitator:
      1. Transitions settlement to CONFIRMED.
      2. If fulfillment is RECOVERY_REQUIRED and an upstream_executor is supplied,
         attempts compensating delivery to complete fulfillment.
      3. If delivery still fails, leaves fulfillment as RECOVERY_REQUIRED for free retry.
    """

    def __init__(
        self,
        coordinator: GatewayStateCoordinator,
        probe: ISettlementProbe,
        *,
        upstream_executor: UpstreamExecutor | None = None,
        clock: Callable[[], int] | None = None,
        safe_buffer_seconds: int = 60,
    ) -> None:
        self.coordinator = coordinator
        self.probe = probe
        self.upstream_executor = upstream_executor
        self._clock = clock or coordinator.get_time
        self.safe_buffer_seconds = safe_buffer_seconds

    def get_time(self) -> int:
        return self._clock()

    def reconcile_settlement(self, settlement_id: str) -> GatewayReconciliationReport:
        """
        Reconcile a single settlement by ID.

        Returns:
            GatewayReconciliationReport with final settlement and fulfillment status.
        """
        now = self.get_time()
        settlement = self.coordinator.get_settlement(settlement_id)
        if not settlement:
            raise ValueError(f"Gateway settlement '{settlement_id}' not found.")

        prev_settlement_status = settlement.status
        fulfillment = self.coordinator.get_fulfillment_by_settlement(settlement_id)
        ful_status = fulfillment.status if fulfillment else FulfillmentState.NOT_STARTED

        # If settlement is already terminal CONFIRMED / SUCCESS:
        if prev_settlement_status in (
            SettlementState.CONFIRMED,
            SettlementState.SUCCESS,
        ):
            if ful_status == FulfillmentState.DELIVERED:
                return GatewayReconciliationReport(
                    settlementId=settlement.settlement_id,
                    previousSettlementStatus=prev_settlement_status,
                    newSettlementStatus=prev_settlement_status,
                    fulfillmentStatus=ful_status,
                    outcome=GatewayReconciliationOutcome.ALREADY_DELIVERED,
                    txHash=settlement.tx_hash,
                    reason="Settlement confirmed and resource already delivered.",
                )
            if ful_status in (
                FulfillmentState.RECOVERY_REQUIRED,
                FulfillmentState.NOT_DELIVERED,
                FulfillmentState.FAILED,
            ):
                return self._attempt_compensating_fulfillment_or_ready(settlement, fulfillment, prev_settlement_status)

        # Probe Facilitator for settlement status
        probe_result = self.probe.probe_settlement(settlement_id)

        if probe_result.status in (SettlementState.CONFIRMED, SettlementState.SUCCESS):
            tx_hash = probe_result.tx_hash or settlement.tx_hash
            if prev_settlement_status != SettlementState.CONFIRMED:
                self.coordinator.transition_settlement(
                    settlement.settlement_id,
                    SettlementState.CONFIRMED,
                    tx_hash=tx_hash,
                )
            return self._attempt_compensating_fulfillment_or_ready(
                settlement, fulfillment, prev_settlement_status, tx_hash=tx_hash
            )

        if probe_result.status == SettlementState.FAILED:
            expiry_threshold = settlement.valid_before + self.safe_buffer_seconds
            if now > expiry_threshold:
                self.coordinator.transition_settlement(
                    settlement.settlement_id,
                    SettlementState.FAILED,
                    error_message=f"Reconciler verified settlement failed on facilitator; expired at {settlement.valid_before}",
                )
                return GatewayReconciliationReport(
                    settlementId=settlement.settlement_id,
                    previousSettlementStatus=prev_settlement_status,
                    newSettlementStatus=SettlementState.FAILED,
                    fulfillmentStatus=ful_status,
                    outcome=GatewayReconciliationOutcome.FAILED_EXPIRED,
                    txHash=None,
                    reason=f"Settlement failed on facilitator; expired at {settlement.valid_before}; marked FAILED",
                )

            if prev_settlement_status != SettlementState.UNKNOWN:
                self.coordinator.transition_settlement(
                    settlement.settlement_id,
                    SettlementState.UNKNOWN,
                    error_message="Facilitator failed but validBefore window still open; awaiting expiration",
                )
            return GatewayReconciliationReport(
                settlementId=settlement.settlement_id,
                previousSettlementStatus=prev_settlement_status,
                newSettlementStatus=SettlementState.UNKNOWN,
                fulfillmentStatus=ful_status,
                outcome=GatewayReconciliationOutcome.UNKNOWN_WAITING_EXPIRATION,
                txHash=None,
                reason=f"Settlement failed on probe but validBefore ({settlement.valid_before}) not yet expired; awaiting safe window",
            )

        # UNKNOWN probe result: Fail-closed
        if prev_settlement_status != SettlementState.UNKNOWN:
            self.coordinator.transition_settlement(
                settlement.settlement_id,
                SettlementState.UNKNOWN,
                error_message=f"Probe result was UNKNOWN ({probe_result.error})",
            )
        return GatewayReconciliationReport(
            settlementId=settlement.settlement_id,
            previousSettlementStatus=prev_settlement_status,
            newSettlementStatus=SettlementState.UNKNOWN,
            fulfillmentStatus=ful_status,
            outcome=GatewayReconciliationOutcome.SKIPPED_UNKNOWN,
            txHash=settlement.tx_hash,
            reason=f"Probe result was UNKNOWN ({probe_result.error}); skipping mutation to prevent double-spend",
        )

    def _attempt_compensating_fulfillment_or_ready(
        self,
        settlement: any,
        fulfillment: GatewayFulfillmentRecord | None,
        prev_settlement_status: SettlementState,
        tx_hash: str | None = None,
    ) -> GatewayReconciliationReport:
        """Attempt compensating delivery if executor provided, else leave as RECOVERY_REQUIRED."""
        resolved_tx_hash = tx_hash or settlement.tx_hash
        if not fulfillment:
            return GatewayReconciliationReport(
                settlementId=settlement.settlement_id,
                previousSettlementStatus=prev_settlement_status,
                newSettlementStatus=SettlementState.CONFIRMED,
                fulfillmentStatus=FulfillmentState.NOT_STARTED,
                outcome=GatewayReconciliationOutcome.SETTLED_PENDING_CLIENT_RETRY,
                txHash=resolved_tx_hash,
                reason="Settlement confirmed; no fulfillment record existed; ready for client replay",
            )

        ful_id = fulfillment.fulfillment_id

        if self.upstream_executor:
            try:
                status_code, body_bytes, headers_dict = self.upstream_executor(fulfillment)
                if 200 <= status_code < 300:
                    import json

                    headers_json = json.dumps(headers_dict) if headers_dict else None
                    self.coordinator.complete_fulfillment(
                        ful_id,
                        upstream_status_code=status_code,
                        response_body_blob=body_bytes,
                        response_headers_json=headers_json,
                    )
                    return GatewayReconciliationReport(
                        settlementId=settlement.settlement_id,
                        previousSettlementStatus=prev_settlement_status,
                        newSettlementStatus=SettlementState.CONFIRMED,
                        fulfillmentStatus=FulfillmentState.DELIVERED,
                        outcome=GatewayReconciliationOutcome.SETTLED_AND_DELIVERED,
                        txHash=resolved_tx_hash,
                        reason="Compensating upstream execution succeeded with 2xx",
                    )
                else:
                    self.coordinator.fail_fulfillment(
                        ful_id,
                        upstream_status_code=status_code,
                        error_message=f"Compensating upstream attempt failed: HTTP {status_code}",
                    )
                    return GatewayReconciliationReport(
                        settlementId=settlement.settlement_id,
                        previousSettlementStatus=prev_settlement_status,
                        newSettlementStatus=SettlementState.CONFIRMED,
                        fulfillmentStatus=FulfillmentState.RECOVERY_REQUIRED,
                        outcome=GatewayReconciliationOutcome.SETTLED_FULFILLMENT_RECOVERY_REQUIRED,
                        txHash=resolved_tx_hash,
                        reason=f"Compensating upstream execution failed (HTTP {status_code}); remains RECOVERY_REQUIRED",
                    )
            except Exception as exc:  # noqa: BLE001
                self.coordinator.fail_fulfillment(
                    ful_id,
                    upstream_status_code=500,
                    error_message=str(exc),
                )
                return GatewayReconciliationReport(
                    settlementId=settlement.settlement_id,
                    previousSettlementStatus=prev_settlement_status,
                    newSettlementStatus=SettlementState.CONFIRMED,
                    fulfillmentStatus=FulfillmentState.RECOVERY_REQUIRED,
                    outcome=GatewayReconciliationOutcome.SETTLED_FULFILLMENT_RECOVERY_REQUIRED,
                    txHash=resolved_tx_hash,
                    reason=f"Compensating upstream execution raised error: {exc}",
                )

        # No upstream executor provided: keep ready for client retry
        return GatewayReconciliationReport(
            settlementId=settlement.settlement_id,
            previousSettlementStatus=prev_settlement_status,
            newSettlementStatus=SettlementState.CONFIRMED,
            fulfillmentStatus=fulfillment.status,
            outcome=GatewayReconciliationOutcome.SETTLED_PENDING_CLIENT_RETRY,
            txHash=resolved_tx_hash,
            reason="Settlement confirmed; no upstream executor configured; ready for free client replay",
        )

    def reconcile_all_unresolved(self) -> list[GatewayReconciliationReport]:
        """Scan and reconcile all settlements in SUBMITTED, PENDING, or UNKNOWN."""
        unresolved = self.coordinator.list_settlements(
            [
                SettlementState.SUBMITTED,
                SettlementState.PENDING,
                SettlementState.UNKNOWN,
            ]
        )
        reports: list[GatewayReconciliationReport] = []
        for s in unresolved:
            rep = self.reconcile_settlement(s.settlement_id)
            reports.append(rep)
        return reports
