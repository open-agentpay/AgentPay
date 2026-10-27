import { describe, expect, it } from 'vitest';
import {
  BreakerState,
  PolicyConfig,
  PolicyErrorCode,
  PolicyViolationError,
  SQLitePolicyEngine,
} from '../../src/index.js';

describe('SQLitePolicyEngine Unit Tests', () => {
  const createBaseConfig = (overrides?: Partial<ConstructorParameters<typeof PolicyConfig>[0]>) => {
    return new PolicyConfig({
      id: 'pol_unit_test',
      name: 'Unit Test Policy',
      walletAddress: '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266',
      maxPerTxBaseUnits: 1_000_000n,
      rollingBudget24hBaseUnits: 5_000_000n,
      lifetimeBudgetBaseUnits: 20_000_000n,
      whitelistPayees: ['0x70997970c51812dc3a010c7d01b50e0d17dc79c8'],
      whitelistAssets: ['0x036cbd53842c5426634e7929541ec2318f3dcf7e'],
      whitelistNetworks: ['eip155:84532'],
      whitelistDomains: ['api.trusted.com'],
      circuitMaxRpm: 5,
      circuitConsecutiveFailures: 3,
      circuitCooldownSeconds: 60,
      ...overrides,
    });
  };

  describe('Circuit Breaker Mechanics', () => {
    it('trips to OPEN after consecutive failures threshold reached', () => {
      let clockTime = 1000;
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig(), () => clockTime);

      expect(engine.getBreakerState()).toBe(BreakerState.CLOSED);

      engine.recordFailure('timeout 1');
      expect(engine.getBreakerState()).toBe(BreakerState.CLOSED);

      engine.recordFailure('timeout 2');
      expect(engine.getBreakerState()).toBe(BreakerState.CLOSED);

      engine.recordFailure('timeout 3');
      expect(engine.getBreakerState()).toBe(BreakerState.OPEN);

      const decision = engine.reserve({
        intentId: 'intent_cb_1',
        amount: 100_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
      });
      expect(decision.approved).toBe(false);
      expect(decision.errorCode).toBe(PolicyErrorCode.CIRCUIT_BREAKER_OPEN);
    });

    it('transitions from OPEN to HALF_OPEN after cooldown elapsed', () => {
      let clockTime = 1000;
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig(), () => clockTime);

      // Trip breaker
      engine.recordFailure('f1');
      engine.recordFailure('f2');
      engine.recordFailure('f3');
      expect(engine.getBreakerState()).toBe(BreakerState.OPEN);

      // Advance clock past cooldown (cooldown = 60s)
      clockTime += 61;
      expect(engine.getBreakerState()).toBe(BreakerState.HALF_OPEN);
    });

    it('allows 1 probe in HALF_OPEN and rejects second probe', () => {
      let clockTime = 1000;
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig(), () => clockTime);

      engine.recordFailure('f1');
      engine.recordFailure('f2');
      engine.recordFailure('f3');
      expect(engine.getBreakerState()).toBe(BreakerState.OPEN);

      // Cooldown elapses
      clockTime += 61;
      expect(engine.getBreakerState()).toBe(BreakerState.HALF_OPEN);

      // First probe reservation: should be approved
      const probe1 = engine.reserve({
        intentId: 'intent_probe_1',
        amount: 50_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
        ttlSeconds: 300,
      });
      expect(probe1.approved).toBe(true);

      // Second probe while first is still in-flight: should be rejected
      const probe2 = engine.reserve({
        intentId: 'intent_probe_2',
        amount: 50_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
      });
      expect(probe2.approved).toBe(false);
      expect(probe2.errorCode).toBe(PolicyErrorCode.CIRCUIT_BREAKER_OPEN);

      // Successful probe commit transitions HALF_OPEN to CLOSED
      engine.commit('intent_probe_1');
      expect(engine.getBreakerState()).toBe(BreakerState.CLOSED);
    });

    it('trips immediately to OPEN if probe fails in HALF_OPEN', () => {
      let clockTime = 1000;
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig(), () => clockTime);

      engine.recordFailure('f1');
      engine.recordFailure('f2');
      engine.recordFailure('f3');
      clockTime += 61;
      expect(engine.getBreakerState()).toBe(BreakerState.HALF_OPEN);

      // Failure during half-open
      engine.recordFailure('probe failed');
      expect(engine.getBreakerState()).toBe(BreakerState.OPEN);
    });

    it('trips breaker when 60s RPM threshold is exceeded', () => {
      let clockTime = 1000;
      // circuitMaxRpm is 5
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig({ circuitMaxRpm: 3 }), () => clockTime);

      for (let i = 1; i <= 3; i++) {
        const res = engine.reserve({
          intentId: `rpm_${i}`,
          amount: 10_000n,
          payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
          asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
          network: 'eip155:84532',
        });
        expect(res.approved).toBe(true);
      }

      // 4th request within same minute exceeds RPM (>= 3 in last 60s)
      const res4 = engine.reserve({
        intentId: 'rpm_4',
        amount: 10_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
      });
      expect(res4.approved).toBe(false);
      expect(res4.errorCode).toBe(PolicyErrorCode.CIRCUIT_BREAKER_OPEN);
      expect(engine.getBreakerState()).toBe(BreakerState.OPEN);
    });

    it('resets breaker via resetBreaker', () => {
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig());
      engine.recordFailure('f1');
      engine.recordFailure('f2');
      engine.recordFailure('f3');
      expect(engine.getBreakerState()).toBe(BreakerState.OPEN);

      engine.resetBreaker();
      expect(engine.getBreakerState()).toBe(BreakerState.CLOSED);
    });
  });

  describe('Credential-Mode Metered Reservations', () => {
    it('executes reserveMetered and settleMetered', () => {
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig());

      const res = engine.reserveMetered('metered_01', 200_000n, 'api.trusted.com');
      expect(res.approved).toBe(true);
      expect(res.reservation).toBeDefined();
      expect(res.availableAfter).toBe(4_800_000n);

      // Settle with actual amount higher or lower
      engine.settleMetered('metered_01', 150_000n);
      expect(engine.getAvailableRollingBudget()).toBe(4_850_000n);
    });

    it('rejects unwhitelisted domain in reserveMetered', () => {
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig());
      const res = engine.reserveMetered('metered_02', 200_000n, 'api.untrusted.com');
      expect(res.approved).toBe(false);
      expect(res.errorCode).toBe(PolicyErrorCode.DOMAIN_NOT_WHITELISTED);
    });

    it('rejects invalid amounts in reserveMetered', () => {
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig());
      const res = engine.reserveMetered('metered_03', -100n, 'api.trusted.com');
      expect(res.approved).toBe(false);
      expect(res.errorCode).toBe(PolicyErrorCode.INVALID_AMOUNT_FORMAT);
    });
  });

  describe('Commit and Release Guardrails', () => {
    it('rejects commit when reservation has expired', () => {
      let clockTime = 1000;
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig(), () => clockTime);

      const res = engine.reserve({
        intentId: 'exp_intent',
        amount: 50_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
        ttlSeconds: 60,
      });
      expect(res.approved).toBe(true);

      // Clock advances past expiresAt (1000 + 60 = 1060)
      clockTime = 1061;

      expect(() => engine.commit('exp_intent')).toThrow(PolicyViolationError);
    });

    it('rejects release when already COMMITTED', () => {
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig());
      const res = engine.reserve({
        intentId: 'committed_intent',
        amount: 50_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
      });
      expect(res.approved).toBe(true);

      engine.commit('committed_intent');

      expect(() => engine.release('committed_intent')).toThrow(PolicyViolationError);
    });

    it('treats duplicate commit as idempotent no-op', () => {
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig());
      engine.reserve({
        intentId: 'dup_commit',
        amount: 50_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
      });
      engine.commit('dup_commit');
      // Second commit should not throw
      expect(() => engine.commit('dup_commit')).not.toThrow();
    });

    it('treats duplicate release as idempotent no-op', () => {
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig());
      engine.reserve({
        intentId: 'dup_release',
        amount: 50_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
      });
      engine.release('dup_release');
      // Second release should not throw
      expect(() => engine.release('dup_release')).not.toThrow();
    });

    it('returns DUPLICATE_INTENT when reserving with already existing intentId', () => {
      const engine = SQLitePolicyEngine.createInMemory(createBaseConfig());
      const res1 = engine.reserve({
        intentId: 'unique_intent',
        amount: 50_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
      });
      expect(res1.approved).toBe(true);

      const res2 = engine.reserve({
        intentId: 'unique_intent',
        amount: 50_000n,
        payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
        asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
        network: 'eip155:84532',
      });
      expect(res2.approved).toBe(false);
      expect(res2.errorCode).toBe(PolicyErrorCode.DUPLICATE_INTENT);
    });
  });

  describe('Concurrency & Anti-Overspend Stress Test', () => {
    it('strictly guarantees no overspend under rapid parallel reservations', async () => {
      // 5 USDC total rolling budget, 0.5 USDC per tx -> max 10 transactions can succeed
      const budgetCap = 5_000_000n;
      const txAmount = 500_000n;
      const engine = SQLitePolicyEngine.createInMemory(
        createBaseConfig({
          maxPerTxBaseUnits: txAmount,
          rollingBudget24hBaseUnits: budgetCap,
          circuitMaxRpm: 100,
        }),
      );

      // Launch 25 concurrent reservations
      const attempts = Array.from({ length: 25 }, (_, i) => {
        return Promise.resolve().then(() => {
          return engine.reserve({
            intentId: `concurrent_${i}`,
            amount: txAmount,
            payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
            asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
            network: 'eip155:84532',
          });
        });
      });

      const results = await Promise.all(attempts);

      const approved = results.filter((r) => r.approved);
      const rejected = results.filter((r) => !r.approved);

      // Exactly 10 approved (10 * 500,000 = 5,000,000)
      expect(approved.length).toBe(10);
      expect(rejected.length).toBe(15);

      for (const rej of rejected) {
        expect(rej.errorCode).toBe(PolicyErrorCode.DAILY_BUDGET_EXCEEDED);
      }

      // Available budget should now be 0
      expect(engine.getAvailableRollingBudget()).toBe(0n);
    });
  });
});
