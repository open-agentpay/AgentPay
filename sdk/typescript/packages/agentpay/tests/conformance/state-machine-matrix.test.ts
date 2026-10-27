import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  BudgetLedgerState,
  FulfillmentState,
  IllegalStateTransitionError,
  InvalidCompositeStateError,
  OrphanedSettlementRollbackForbiddenError,
  PaymentIntentState,
  SettlementState,
  SignatureSentReleaseForbiddenError,
  validateCompositeState,
  validateFulfillmentTransition,
  validateIntentTransition,
  validateLedgerTransition,
  validateSettlementTransition,
} from '../../src/index.js';

const SPEC_PATH = resolve(__dirname, '../../../../../../specs/conformance/state-machine.json');

describe('State Machine Matrix & Invariants Conformance (specs/conformance/state-machine.json)', () => {
  const specRaw = readFileSync(SPEC_PATH, 'utf-8');
  const spec = JSON.parse(specRaw) as {
    version: string;
    supported_payment_methods: string[];
    entities: Record<string, {
      entity_type: string;
      states: Record<string, { is_terminal: boolean }>;
      transitions: Array<{ from: string; event: string; to: string }>;
    }>;
    coordination: {
      composite_views: Array<{
        settlement: string[];
        fulfillment: string[];
        display: string;
      }>;
    };
  };

  it('validates state-machine.json specification integrity', () => {
    expect(spec.version).toBe('2.1.0');
    expect(spec.supported_payment_methods).toEqual(['onchain', 'wallet']);
    expect(spec.entities.payment_intent).toBeDefined();
    expect(spec.entities.settlement).toBeDefined();
    expect(spec.entities.fulfillment).toBeDefined();
  });

  describe('Transition Guards and Error Handling', () => {
    it('allows legal intent transitions', () => {
      expect(() => validateIntentTransition(PaymentIntentState.CREATED, PaymentIntentState.PROCESSING)).not.toThrow();
      expect(() => validateIntentTransition(PaymentIntentState.PROCESSING, PaymentIntentState.SIGNATURE_SENT)).not.toThrow();
      expect(() => validateIntentTransition(PaymentIntentState.SIGNATURE_SENT, PaymentIntentState.SUCCEEDED)).not.toThrow();
      expect(() => validateIntentTransition(PaymentIntentState.SIGNATURE_SENT, PaymentIntentState.RECOVERY_HOLD)).not.toThrow();
      expect(() => validateIntentTransition(PaymentIntentState.RECOVERY_HOLD, PaymentIntentState.SUCCEEDED)).not.toThrow();
    });

    it('rejects illegal intent transitions', () => {
      // Terminal state mutating
      expect(() => validateIntentTransition(PaymentIntentState.SUCCEEDED, PaymentIntentState.PROCESSING)).toThrow(
        IllegalStateTransitionError,
      );
      // Illegal jump
      expect(() => validateIntentTransition(PaymentIntentState.CREATED, PaymentIntentState.SUCCEEDED)).toThrow(
        IllegalStateTransitionError,
      );
    });

    it('enforces Anti-Overspend Guard: SIGNATURE_SENT & RECOVERY_HOLD cannot be CANCELLED', () => {
      expect(() =>
        validateIntentTransition(PaymentIntentState.SIGNATURE_SENT, PaymentIntentState.CANCELLED, {
          intentId: 'int_123',
        }),
      ).toThrow(SignatureSentReleaseForbiddenError);

      expect(() =>
        validateIntentTransition(PaymentIntentState.RECOVERY_HOLD, PaymentIntentState.CANCELLED, {
          intentId: 'int_123',
        }),
      ).toThrow(SignatureSentReleaseForbiddenError);
    });

    it('enforces Anti-Rollback Guard: active settlement cannot rollback to NOT_STARTED / NOT_SUBMITTED', () => {
      expect(() =>
        validateSettlementTransition(SettlementState.SUBMITTED, SettlementState.NOT_STARTED, {
          settlementId: 'stl_1',
        }),
      ).toThrow(OrphanedSettlementRollbackForbiddenError);

      expect(() =>
        validateSettlementTransition(SettlementState.SUCCESS, SettlementState.NOT_STARTED, {
          settlementId: 'stl_2',
        }),
      ).toThrow(OrphanedSettlementRollbackForbiddenError);

      expect(() =>
        validateSettlementTransition(SettlementState.CONFIRMED, SettlementState.NOT_SUBMITTED, {
          settlementId: 'stl_3',
        }),
      ).toThrow(OrphanedSettlementRollbackForbiddenError);
    });

    it('validates legal and illegal fulfillment transitions', () => {
      expect(() => validateFulfillmentTransition(FulfillmentState.NOT_DELIVERED, FulfillmentState.DELIVERING)).not.toThrow();
      expect(() => validateFulfillmentTransition(FulfillmentState.DELIVERING, FulfillmentState.DELIVERED)).not.toThrow();
      expect(() => validateFulfillmentTransition(FulfillmentState.DELIVERING, FulfillmentState.RECOVERY_REQUIRED)).not.toThrow();
      expect(() => validateFulfillmentTransition(FulfillmentState.RECOVERY_REQUIRED, FulfillmentState.DELIVERING)).not.toThrow();

      // Terminal state mutation
      expect(() => validateFulfillmentTransition(FulfillmentState.DELIVERED, FulfillmentState.PROCESSING)).toThrow(
        IllegalStateTransitionError,
      );
    });

    it('validates ledger transitions', () => {
      expect(() => validateLedgerTransition(BudgetLedgerState.RESERVED, BudgetLedgerState.COMMITTED)).not.toThrow();
      expect(() => validateLedgerTransition(BudgetLedgerState.RESERVED, BudgetLedgerState.RELEASED)).not.toThrow();

      // Terminal states mutation
      expect(() => validateLedgerTransition(BudgetLedgerState.COMMITTED, BudgetLedgerState.RELEASED)).toThrow(
        IllegalStateTransitionError,
      );
      expect(() => validateLedgerTransition(BudgetLedgerState.RELEASED, BudgetLedgerState.COMMITTED)).toThrow(
        IllegalStateTransitionError,
      );
    });
  });

  describe('Composite State Validation', () => {
    it('rejects Anti-Overspend violation (SIGNATURE_SENT / RECOVERY_HOLD with RELEASED ledger)', () => {
      expect(() =>
        validateCompositeState(
          PaymentIntentState.SIGNATURE_SENT,
          SettlementState.SUBMITTED,
          FulfillmentState.PROCESSING,
          BudgetLedgerState.RELEASED,
        ),
      ).toThrow(InvalidCompositeStateError);

      expect(() =>
        validateCompositeState(
          PaymentIntentState.RECOVERY_HOLD,
          SettlementState.UNKNOWN,
          FulfillmentState.RECOVERY_REQUIRED,
          BudgetLedgerState.RELEASED,
        ),
      ).toThrow(InvalidCompositeStateError);
    });

    it('rejects SUCCEEDED intent when fulfillment was not DELIVERED', () => {
      expect(() =>
        validateCompositeState(
          PaymentIntentState.SUCCEEDED,
          SettlementState.SUCCESS,
          FulfillmentState.DELIVERING,
          BudgetLedgerState.COMMITTED,
        ),
      ).toThrow(InvalidCompositeStateError);
    });

    it('rejects SUCCEEDED intent when ledger is not COMMITTED', () => {
      expect(() =>
        validateCompositeState(
          PaymentIntentState.SUCCEEDED,
          SettlementState.SUCCESS,
          FulfillmentState.DELIVERED,
          BudgetLedgerState.RESERVED,
        ),
      ).toThrow(InvalidCompositeStateError);
    });

    it('rejects DELIVERED resource when settlement was never submitted or succeeded', () => {
      expect(() =>
        validateCompositeState(
          PaymentIntentState.PROCESSING,
          SettlementState.NOT_STARTED,
          FulfillmentState.DELIVERED,
          BudgetLedgerState.RESERVED,
        ),
      ).toThrow(InvalidCompositeStateError);
    });

    it('allows valid composite states', () => {
      expect(() =>
        validateCompositeState(
          PaymentIntentState.SUCCEEDED,
          SettlementState.SUCCESS,
          FulfillmentState.DELIVERED,
          BudgetLedgerState.COMMITTED,
        ),
      ).not.toThrow();

      expect(() =>
        validateCompositeState(
          PaymentIntentState.RECOVERY_HOLD,
          SettlementState.UNKNOWN,
          FulfillmentState.RECOVERY_REQUIRED,
          BudgetLedgerState.RESERVED,
        ),
      ).not.toThrow();
    });
  });
});
