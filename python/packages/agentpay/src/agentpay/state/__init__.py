"""
AgentPay Buyer State Machine and Coordination package.
"""

from __future__ import annotations

from agentpay.core.state import (
    BudgetLedgerState,
    BuyerIntentRecord,
    PaymentIntentState,
)
from agentpay.state.buyer_coordinator import BuyerStateCoordinator
from agentpay.state.errors import (
    IdempotencyLockConflictError,
    IllegalStateTransitionError,
    InvalidCompositeStateError,
    OrphanedSettlementRollbackForbiddenError,
    SignatureSentReleaseForbiddenError,
    StateCoordinationError,
)
from agentpay.state.guards import (
    LEGAL_INTENT_TRANSITIONS,
    LEGAL_LEDGER_TRANSITIONS,
    validate_composite_state,
    validate_intent_transition,
    validate_ledger_transition,
)
from agentpay.state.reconciliation import (
    BuyerReconciler,
    BuyerReconciliationOutcome,
    BuyerReconciliationReport,
    FacilitatorSettlementProbe,
    ISettlementProbe,
    MockSettlementProbe,
    NonceProbeResult,
    NonceStatus,
    SettlementProbeResult,
)

__all__ = [
    "LEGAL_INTENT_TRANSITIONS",
    "LEGAL_LEDGER_TRANSITIONS",
    "BudgetLedgerState",
    "BuyerIntentRecord",
    "BuyerReconciler",
    "BuyerReconciliationOutcome",
    "BuyerReconciliationReport",
    "BuyerStateCoordinator",
    "FacilitatorSettlementProbe",
    "ISettlementProbe",
    "IdempotencyLockConflictError",
    "IllegalStateTransitionError",
    "InvalidCompositeStateError",
    "MockSettlementProbe",
    "NonceProbeResult",
    "NonceStatus",
    "OrphanedSettlementRollbackForbiddenError",
    "PaymentIntentState",
    "SettlementProbeResult",
    "SignatureSentReleaseForbiddenError",
    "StateCoordinationError",
    "validate_composite_state",
    "validate_intent_transition",
    "validate_ledger_transition",
]
