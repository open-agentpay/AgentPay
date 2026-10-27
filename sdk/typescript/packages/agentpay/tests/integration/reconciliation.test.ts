import { beforeEach, describe, expect, it } from 'vitest';
import {
  BudgetLedgerState,
  BuyerIntentRecord,
  BuyerReconciler,
  BuyerReconciliationOutcome,
  BuyerStateCoordinator,
  MockSettlementProbe,
  NonceStatus,
  PaymentIntentState,
  SignatureSentReleaseForbiddenError,
  createBuyerConnection,
  initBuyerDatabase,
} from '../../src/index.js';

describe('Buyer State Coordinator & Reconciliation Integration Tests', () => {
  let db: ReturnType<typeof createBuyerConnection>;
  let simulatedTime = 1000;
  const clock = () => simulatedTime;

  beforeEach(() => {
    simulatedTime = 1000;
    db = createBuyerConnection(':memory:');
    initBuyerDatabase(db);
    db.exec(`
      INSERT INTO ap_policy (
        id, name, wallet_address, max_per_tx_base_units, rolling_budget_24h_base_units, created_at, updated_at
      ) VALUES ('pol_default', 'Default Policy', '0x1111111111111111111111111111111111111111', 1000000, 10000000, 1000, 1000);
    `);
  });

  describe('BuyerStateCoordinator Lifecycle & Anti-Overspend Guards', () => {
    it('executes normal happy path (create -> reserve -> dispatch -> complete)', () => {
      const coordinator = new BuyerStateCoordinator(db, { clock });
      const intentId = 'cin_happy_1';

      const record = new BuyerIntentRecord({
        intentId,
        policyId: 'pol_default',
        idempotencyKey: 'idem_happy_1',
        payerAddress: '0x1111111111111111111111111111111111111111',
        payeeAddress: '0x2222222222222222222222222222222222222222',
        assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        networkId: 'eip155:84532',
        amountBaseUnits: 50_000n,
        status: PaymentIntentState.CREATED,
        nonce: '0xaaa111',
        validAfter: 0,
        validBefore: 1200,
        createdAt: simulatedTime,
        updatedAt: simulatedTime,
      });

      coordinator.createIntent(record);
      expect(coordinator.getIntent(intentId)?.status).toBe(PaymentIntentState.CREATED);

      const reservationId = coordinator.reserveIntent(intentId, 'pol_default', record.payerAddress, 50_000n);
      expect(reservationId).toMatch(/^res_/);
      expect(coordinator.getIntent(intentId)?.status).toBe(PaymentIntentState.PROCESSING);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.RESERVED);

      coordinator.recordDispatch(intentId, '0xsig123', '0xaaa111');
      expect(coordinator.getIntent(intentId)?.status).toBe(PaymentIntentState.SIGNATURE_SENT);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.RESERVED);

      coordinator.completeIntent(intentId);
      expect(coordinator.getIntent(intentId)?.status).toBe(PaymentIntentState.SUCCEEDED);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.COMMITTED);
    });

    it('allows cancel before wire dispatch and releases quota', () => {
      const coordinator = new BuyerStateCoordinator(db, { clock });
      const intentId = 'cin_cancel_1';

      const record = new BuyerIntentRecord({
        intentId,
        policyId: 'pol_default',
        idempotencyKey: 'idem_cancel_1',
        payerAddress: '0x1111111111111111111111111111111111111111',
        payeeAddress: '0x2222222222222222222222222222222222222222',
        assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        networkId: 'eip155:84532',
        amountBaseUnits: 50_000n,
        status: PaymentIntentState.CREATED,
        createdAt: simulatedTime,
        updatedAt: simulatedTime,
      });

      coordinator.createIntent(record);
      coordinator.reserveIntent(intentId, 'pol_default', record.payerAddress, 50_000n);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.RESERVED);

      // Cancel before dispatch (e.g. signing error)
      coordinator.cancelIntent(intentId, 'User aborted');
      expect(coordinator.getIntent(intentId)?.status).toBe(PaymentIntentState.CANCELLED);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.RELEASED);
    });

    it('forbids cancelIntent once SIGNATURE_SENT or in RECOVERY_HOLD', () => {
      const coordinator = new BuyerStateCoordinator(db, { clock });
      const intentId = 'cin_forbidden_cancel';

      const record = new BuyerIntentRecord({
        intentId,
        policyId: 'pol_default',
        idempotencyKey: 'idem_forbidden_1',
        payerAddress: '0x1111111111111111111111111111111111111111',
        payeeAddress: '0x2222222222222222222222222222222222222222',
        assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        networkId: 'eip155:84532',
        amountBaseUnits: 50_000n,
        status: PaymentIntentState.CREATED,
        nonce: '0xnonce_fb',
        createdAt: simulatedTime,
        updatedAt: simulatedTime,
      });

      coordinator.createIntent(record);
      coordinator.reserveIntent(intentId, 'pol_default', record.payerAddress, 50_000n);
      coordinator.recordDispatch(intentId);

      // Attempt cancel after dispatch must throw
      expect(() => coordinator.cancelIntent(intentId)).toThrow(SignatureSentReleaseForbiddenError);

      coordinator.holdIntent(intentId);
      // Attempt cancel in RECOVERY_HOLD must throw
      expect(() => coordinator.cancelIntent(intentId)).toThrow(SignatureSentReleaseForbiddenError);
    });
  });

  describe('BuyerReconciler Nonce Auditing & Quota Finalization', () => {
    it('commits quota when probe confirms nonce was USED on-chain', async () => {
      const coordinator = new BuyerStateCoordinator(db, { clock });
      const probe = new MockSettlementProbe();
      const reconciler = new BuyerReconciler(coordinator, probe, { clock, safeBufferSeconds: 60 });

      const intentId = 'reconcile_used';
      const record = new BuyerIntentRecord({
        intentId,
        policyId: 'pol_default',
        idempotencyKey: 'idem_used',
        payerAddress: '0x1111111111111111111111111111111111111111',
        payeeAddress: '0x2222222222222222222222222222222222222222',
        assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        networkId: 'eip155:84532',
        amountBaseUnits: 50_000n,
        status: PaymentIntentState.CREATED,
        nonce: '0xnonce_used_1',
        validAfter: 0,
        validBefore: 1200,
        createdAt: simulatedTime,
        updatedAt: simulatedTime,
      });

      coordinator.createIntent(record);
      coordinator.reserveIntent(intentId, 'pol_default', record.payerAddress, 50_000n);
      coordinator.recordDispatch(intentId);
      coordinator.holdIntent(intentId);

      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.RESERVED);

      // Probe reports USED
      probe.setNonce(record.networkId, record.assetAddress, record.payerAddress, record.nonce!, NonceStatus.USED, {
        txHash: '0xhash123',
      });

      const report = await reconciler.reconcileIntent(intentId);
      expect(report.outcome).toBe(BuyerReconciliationOutcome.COMMITTED);
      expect(report.newIntentStatus).toBe(PaymentIntentState.SUCCEEDED);
      expect(report.ledgerState).toBe(BudgetLedgerState.COMMITTED);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.COMMITTED);
    });

    it('preserves quota in RESERVED if nonce is UNUSED before validBefore + safeBuffer', async () => {
      const coordinator = new BuyerStateCoordinator(db, { clock });
      const probe = new MockSettlementProbe();
      const reconciler = new BuyerReconciler(coordinator, probe, { clock, safeBufferSeconds: 60 });

      const intentId = 'reconcile_unspent_premature';
      const validBefore = 1100; // safeExpiry = 1100 + 60 = 1160
      const record = new BuyerIntentRecord({
        intentId,
        policyId: 'pol_default',
        idempotencyKey: 'idem_premature',
        payerAddress: '0x1111111111111111111111111111111111111111',
        payeeAddress: '0x2222222222222222222222222222222222222222',
        assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        networkId: 'eip155:84532',
        amountBaseUnits: 20_000n,
        status: PaymentIntentState.CREATED,
        nonce: '0xnonce_premature',
        validAfter: 0,
        validBefore,
        createdAt: simulatedTime,
        updatedAt: simulatedTime,
      });

      coordinator.createIntent(record);
      coordinator.reserveIntent(intentId, 'pol_default', record.payerAddress, 20_000n);
      coordinator.recordDispatch(intentId);
      coordinator.holdIntent(intentId);

      probe.setNonce(record.networkId, record.assetAddress, record.payerAddress, record.nonce!, NonceStatus.UNUSED);

      // Advance to 1050 (still < 1160)
      simulatedTime = 1050;
      const report = await reconciler.reconcileIntent(intentId);

      expect(report.outcome).toBe(BuyerReconciliationOutcome.HELD);
      expect(report.newIntentStatus).toBe(PaymentIntentState.RECOVERY_HOLD);
      expect(report.ledgerState).toBe(BudgetLedgerState.RESERVED);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.RESERVED);
    });

    it('releases quota once now > validBefore + safeBuffer and nonce is UNUSED', async () => {
      const coordinator = new BuyerStateCoordinator(db, { clock });
      const probe = new MockSettlementProbe();
      const reconciler = new BuyerReconciler(coordinator, probe, { clock, safeBufferSeconds: 60 });

      const intentId = 'reconcile_unspent_safe';
      const validBefore = 1100; // safeExpiry = 1160
      const record = new BuyerIntentRecord({
        intentId,
        policyId: 'pol_default',
        idempotencyKey: 'idem_safe',
        payerAddress: '0x1111111111111111111111111111111111111111',
        payeeAddress: '0x2222222222222222222222222222222222222222',
        assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        networkId: 'eip155:84532',
        amountBaseUnits: 20_000n,
        status: PaymentIntentState.CREATED,
        nonce: '0xnonce_safe',
        validAfter: 0,
        validBefore,
        createdAt: simulatedTime,
        updatedAt: simulatedTime,
      });

      coordinator.createIntent(record);
      coordinator.reserveIntent(intentId, 'pol_default', record.payerAddress, 20_000n);
      coordinator.recordDispatch(intentId);
      coordinator.holdIntent(intentId);

      probe.setNonce(record.networkId, record.assetAddress, record.payerAddress, record.nonce!, NonceStatus.UNUSED);

      // Advance clock beyond 1160 (e.g. 1165)
      simulatedTime = 1165;
      const report = await reconciler.reconcileIntent(intentId);

      expect(report.outcome).toBe(BuyerReconciliationOutcome.RELEASED);
      expect(report.newIntentStatus).toBe(PaymentIntentState.REJECTED);
      expect(report.ledgerState).toBe(BudgetLedgerState.RELEASED);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.RELEASED);
    });

    it('fails closed when probe status is UNKNOWN', async () => {
      const coordinator = new BuyerStateCoordinator(db, { clock });
      const probe = new MockSettlementProbe();
      probe.raiseOnProbe = true; // Simulates network split
      const reconciler = new BuyerReconciler(coordinator, probe, { clock });

      const intentId = 'reconcile_unknown';
      const record = new BuyerIntentRecord({
        intentId,
        policyId: 'pol_default',
        idempotencyKey: 'idem_unknown',
        payerAddress: '0x1111111111111111111111111111111111111111',
        payeeAddress: '0x2222222222222222222222222222222222222222',
        assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        networkId: 'eip155:84532',
        amountBaseUnits: 20_000n,
        status: PaymentIntentState.CREATED,
        nonce: '0xnonce_unknown',
        validAfter: 0,
        validBefore: 1100,
        createdAt: simulatedTime,
        updatedAt: simulatedTime,
      });

      coordinator.createIntent(record);
      coordinator.reserveIntent(intentId, 'pol_default', record.payerAddress, 20_000n);
      coordinator.recordDispatch(intentId);
      coordinator.holdIntent(intentId);

      simulatedTime = 2000;
      const report = await reconciler.reconcileIntent(intentId);

      expect(report.outcome).toBe(BuyerReconciliationOutcome.SKIPPED_UNKNOWN);
      expect(report.ledgerState).toBe(BudgetLedgerState.RESERVED);
      expect(coordinator.getLedgerState(intentId)).toBe(BudgetLedgerState.RESERVED);
    });

    it('transitions timed-out SIGNATURE_SENT to RECOVERY_HOLD during reconciliation scan', async () => {
      const coordinator = new BuyerStateCoordinator(db, { clock });
      const probe = new MockSettlementProbe();
      const reconciler = new BuyerReconciler(coordinator, probe, { clock, inFlightTimeoutSeconds: 30 });

      const intentId = 'reconcile_timeout_scan';
      const record = new BuyerIntentRecord({
        intentId,
        policyId: 'pol_default',
        idempotencyKey: 'idem_scan',
        payerAddress: '0x1111111111111111111111111111111111111111',
        payeeAddress: '0x2222222222222222222222222222222222222222',
        assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        networkId: 'eip155:84532',
        amountBaseUnits: 20_000n,
        status: PaymentIntentState.CREATED,
        nonce: '0xnonce_scan',
        validAfter: 0,
        validBefore: 1100,
        createdAt: simulatedTime,
        updatedAt: simulatedTime,
      });

      coordinator.createIntent(record);
      coordinator.reserveIntent(intentId, 'pol_default', record.payerAddress, 20_000n);
      coordinator.recordDispatch(intentId);

      // Advance clock past inFlightTimeout (30s)
      simulatedTime += 40;

      const reports = await reconciler.reconcileAllHeld();
      expect(reports.length).toBe(1);
      expect(reports[0]?.previousIntentStatus).toBe(PaymentIntentState.RECOVERY_HOLD);
    });
  });
});
