import { describe, expect, it } from 'vitest';
import { AgentPayClient } from '../../src/client/client.js';
import { LocalSigner } from '../../src/client/signer.js';
import { encodeChallenge, HEADER_PAYMENT_REQUIRED, HEADER_PAYMENT_SIGNATURE } from '../../src/core/codecs.js';
import { PaymentChallenge } from '../../src/core/models.js';
import { BudgetLedgerState, BuyerIntentRecord, PaymentIntentState } from '../../src/core/state.js';
import { createBuyerConnection, initBuyerDatabase } from '../../src/policy/db.js';
import { PolicyConfig } from '../../src/policy/models.js';
import { SQLitePolicyEngine } from '../../src/policy/sqlite-engine.js';
import { BuyerStateCoordinator } from '../../src/state/buyer-coordinator.js';
import {
  BuyerReconciliationOutcome,
  BuyerReconciler,
  MockSettlementProbe,
  NonceStatus,
} from '../../src/state/reconciliation.js';
import { SignatureSentReleaseForbiddenError } from '../../src/state/errors.js';

const TEST_KEY = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';

describe('End-to-End Fault Injection & Crash Recovery', () => {
  it('Fault Scenario 1: Upstream 500 crash after wire dispatch preserves RESERVED ledger state', async () => {
    const signer = new LocalSigner(TEST_KEY);
    const db = createBuyerConnection(':memory:');
    initBuyerDatabase(db);

    const config = new PolicyConfig({
      id: 'pol_fault',
      name: 'Fault Injection Policy',
      walletAddress: signer.address,
      maxPerTxBaseUnits: 100_000n,
      rollingBudget24hBaseUnits: 1_000_000n,
      whitelistPayees: ['0x70997970c51812dc3a010c7d01b50e0d17dc79c8'],
      whitelistAssets: ['0x036cbd53842c5426634e7929541ec2318f3dcf7e'],
      whitelistNetworks: ['eip155:84532'],
      whitelistDomains: ['api.example.com'],
    });

    const clock = () => 1729000100;
    const policy = new SQLitePolicyEngine(db, config, clock);

    const challenge = new PaymentChallenge({
      version: '2.0',
      challengeId: 'ch_fault_500',
      network: 'eip155:84532',
      asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
      amount: '50000',
      payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
      scheme: 'EIP-3009',
      validAfter: 1729000000,
      validBefore: 1729000300,
      facilitatorUrl: 'https://facilitator.example.com',
      resourceUri: '/v1/critical-action',
    });

    const mockFetch: typeof globalThis.fetch = async (input, init) => {
      const headers = new Headers(init?.headers);
      if (!headers.get(HEADER_PAYMENT_SIGNATURE)) {
        return new Response(JSON.stringify({ error: 'Payment Required' }), {
          status: 402,
          headers: {
            [HEADER_PAYMENT_REQUIRED]: encodeChallenge(challenge),
          },
        });
      }

      // Upstream crashes immediately after receiving payment signature
      return new Response(JSON.stringify({ error: 'Database transaction deadlock' }), {
        status: 500,
      });
    };

    const client = new AgentPayClient({
      signer,
      policy,
      clock,
      baseFetch: mockFetch,
    });

    const res = await client.get('https://api.example.com/v1/critical-action');
    expect(res.status).toBe(500);

    // Verify Financial Redline: Reservation MUST NOT be released automatically
    const ledgerRow = db
      .prepare('SELECT state FROM ap_budget_ledger WHERE intent_id = ?')
      .get(challenge.challengeId) as any;
    expect(ledgerRow).toBeDefined();
    expect(ledgerRow.state).toBe(BudgetLedgerState.RESERVED);
  });

  it('Fault Scenario 2: Guard strictly forbids manual cancellation once SIGNATURE_SENT', () => {
    const db = createBuyerConnection(':memory:');
    initBuyerDatabase(db);
    db.exec(`
      INSERT INTO ap_policy (
        id, name, wallet_address, max_per_tx_base_units, rolling_budget_24h_base_units, created_at, updated_at
      ) VALUES ('pol_default', 'Default Policy', '0x1111111111111111111111111111111111111111', 1000000, 10000000, 1000, 1000);
    `);
    const coordinator = new BuyerStateCoordinator(db, { clock: () => 1729000100 });

    const record = new BuyerIntentRecord({
      intentId: 'intent_sig_sent',
      policyId: 'pol_default',
      idempotencyKey: 'idem_sig_1',
      payerAddress: '0x1111111111111111111111111111111111111111',
      payeeAddress: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
      assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
      networkId: 'eip155:84532',
      amountBaseUnits: 50_000n,
      status: PaymentIntentState.CREATED,
      validAfter: 1729000000,
      validBefore: 1729000300,
      createdAt: 1729000100,
      updatedAt: 1729000100,
    });
    coordinator.createIntent(record);
    coordinator.reserveIntent('intent_sig_sent', 'pol_default', record.payerAddress, 50_000n);
    coordinator.recordDispatch('intent_sig_sent');

    // Attempting to cancel directly throws SignatureSentReleaseForbiddenError
    expect(() => coordinator.cancelIntent('intent_sig_sent', 'manual abort')).toThrow(
      SignatureSentReleaseForbiddenError,
    );
  });

  it('Fault Scenario 3: Cold restart and reconciliation safe recovery', async () => {
    const db = createBuyerConnection(':memory:');
    initBuyerDatabase(db);
    db.exec(`
      INSERT INTO ap_policy (
        id, name, wallet_address, max_per_tx_base_units, rolling_budget_24h_base_units, created_at, updated_at
      ) VALUES ('pol_default', 'Default Policy', '0x1111111111111111111111111111111111111111', 1000000, 10000000, 1000, 1000);
    `);

    let currentTime = 1729000100;
    const clock = () => currentTime;
    const coordinator = new BuyerStateCoordinator(db, { clock });

    // Intent 1: settled on-chain
    const record1 = new BuyerIntentRecord({
      intentId: 'intent_settled',
      policyId: 'pol_default',
      idempotencyKey: 'idem_settled',
      payerAddress: '0x1111111111111111111111111111111111111111',
      payeeAddress: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
      assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
      networkId: 'eip155:84532',
      amountBaseUnits: 25_000n,
      status: PaymentIntentState.CREATED,
      nonce: '0xnonce_settled',
      validAfter: 1729000000,
      validBefore: 1729000200,
      createdAt: currentTime,
      updatedAt: currentTime,
    });
    coordinator.createIntent(record1);
    coordinator.reserveIntent('intent_settled', 'pol_default', record1.payerAddress, 25_000n);
    coordinator.recordDispatch('intent_settled');
    coordinator.holdIntent('intent_settled');

    // Intent 2: never settled on-chain
    const record2 = new BuyerIntentRecord({
      intentId: 'intent_unspent',
      policyId: 'pol_default',
      idempotencyKey: 'idem_unspent',
      payerAddress: '0x1111111111111111111111111111111111111111',
      payeeAddress: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
      assetAddress: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
      networkId: 'eip155:84532',
      amountBaseUnits: 35_000n,
      status: PaymentIntentState.CREATED,
      nonce: '0xnonce_unspent',
      validAfter: 1729000000,
      validBefore: 1729000200,
      createdAt: currentTime,
      updatedAt: currentTime,
    });
    coordinator.createIntent(record2);
    coordinator.reserveIntent('intent_unspent', 'pol_default', record2.payerAddress, 35_000n);
    coordinator.recordDispatch('intent_unspent');
    coordinator.holdIntent('intent_unspent');

    // Time advances past validBefore + safeBufferSeconds (1729000200 + 60 = 1729000260)
    currentTime = 1729000400;

    const probe = new MockSettlementProbe();
    probe.setNonce(record1.networkId, record1.assetAddress, record1.payerAddress, record1.nonce!, NonceStatus.USED, {
      txHash: '0xabc123',
    });
    probe.setNonce(record2.networkId, record2.assetAddress, record2.payerAddress, record2.nonce!, NonceStatus.UNUSED);

    const reconciler = new BuyerReconciler(coordinator, probe, { clock, safeBufferSeconds: 60 });
    const report1 = await reconciler.reconcileIntent('intent_settled');
    const report2 = await reconciler.reconcileIntent('intent_unspent');

    expect(report1.outcome).toBe(BuyerReconciliationOutcome.COMMITTED);
    expect(report1.newIntentStatus).toBe(PaymentIntentState.SUCCEEDED);
    expect(coordinator.getLedgerState('intent_settled')).toBe(BudgetLedgerState.COMMITTED);

    expect(report2.outcome).toBe(BuyerReconciliationOutcome.RELEASED);
    expect(report2.newIntentStatus).toBe(PaymentIntentState.REJECTED);
    expect(coordinator.getLedgerState('intent_unspent')).toBe(BudgetLedgerState.RELEASED);
  });
});
