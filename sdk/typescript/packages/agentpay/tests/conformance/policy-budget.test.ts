import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  SQLitePolicyEngine,
  loadPolicyConfig,
} from '../../src/index.js';

const VECTORS_DIR = resolve(__dirname, '../../../../../../specs/test-vectors');

interface PolicyBudgetVectorSuite {
  version: string;
  suite: string;
  description: string;
  policy: Record<string, any>;
  testCases: Array<{
    id: string;
    description: string;
    clock?: number;
    initialState?: Record<string, any>;
    policyOverrides?: Record<string, any>;
    actions: Array<{
      step: number;
      action: 'reserve' | 'commit' | 'release';
      intentId: string;
      amount?: string;
      payee?: string;
      asset?: string;
      network?: string;
      domain?: string;
      expectedResult: 'APPROVED' | 'REJECTED' | 'COMMITTED' | 'RELEASED';
      expectedError?: string;
      expectedAvailableAfter?: string;
    }>;
  }>;
}

describe('Policy Budget Conformance Vectors (specs/test-vectors/policy-budget.json)', () => {
  const vectorRaw = readFileSync(resolve(VECTORS_DIR, 'policy-budget.json'), 'utf-8');
  const suite = JSON.parse(vectorRaw) as PolicyBudgetVectorSuite;

  it('contains all 15 conformance vector test cases', () => {
    expect(suite.testCases.length).toBe(15);
  });

  for (const tc of suite.testCases) {
    it(`executes test case [${tc.id}]: ${tc.description}`, () => {
      let currentClock = tc.clock ?? 1729000000;
      const clock = () => currentClock;

      const cfgDict: Record<string, any> = { ...suite.policy };
      if (tc.policyOverrides) {
        Object.assign(cfgDict, tc.policyOverrides);
      }
      if (tc.initialState) {
        Object.assign(cfgDict, tc.initialState);
      }

      const config = loadPolicyConfig(cfgDict);
      const engine = SQLitePolicyEngine.createInMemory(config, clock);

      for (const act of tc.actions) {
        if (act.action === 'reserve') {
          const decision = engine.reserve({
            intentId: act.intentId,
            amount: act.amount ?? '0',
            payee: act.payee ?? '',
            asset: act.asset ?? '',
            network: act.network ?? '',
            domain: act.domain,
          });

          if (act.expectedResult === 'APPROVED') {
            expect(decision.approved, `[${tc.id} step ${act.step}] expected approved`).toBe(true);
            if (act.expectedAvailableAfter !== undefined) {
              expect(
                decision.availableAfter,
                `[${tc.id} step ${act.step}] expected availableAfter ${act.expectedAvailableAfter}`,
              ).toBe(BigInt(act.expectedAvailableAfter));
            }
          } else if (act.expectedResult === 'REJECTED') {
            expect(decision.approved, `[${tc.id} step ${act.step}] expected rejected`).toBe(false);
            expect(
              decision.errorCode,
              `[${tc.id} step ${act.step}] expected error code ${act.expectedError}`,
            ).toBe(act.expectedError);
          }
        } else if (act.action === 'commit') {
          engine.commit(act.intentId);
          if (act.expectedAvailableAfter !== undefined) {
            const avail = engine.getAvailableRollingBudget();
            expect(
              avail,
              `[${tc.id} step ${act.step}] expected available budget ${act.expectedAvailableAfter}`,
            ).toBe(BigInt(act.expectedAvailableAfter));
          }
        } else if (act.action === 'release') {
          engine.release(act.intentId);
          if (act.expectedAvailableAfter !== undefined) {
            const avail = engine.getAvailableRollingBudget();
            expect(
              avail,
              `[${tc.id} step ${act.step}] expected available budget ${act.expectedAvailableAfter}`,
            ).toBe(BigInt(act.expectedAvailableAfter));
          }
        }
      }
    });
  }
});
