import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  PaymentAuthorization,
  PaymentChallenge,
  X402ErrorCode,
  X402ValidationError,
  decodeAuthorization,
  decodeChallenge,
  encodeAuthorization,
  encodeChallenge,
} from '../../src/core/index.js';

const VECTORS_DIR = resolve(__dirname, '../../../../../../specs/test-vectors');

describe('x402-v2-challenge.json Conformance Vectors', () => {
  const rawJson = readFileSync(resolve(VECTORS_DIR, 'x402-v2-challenge.json'), 'utf-8');
  const suite = JSON.parse(rawJson) as {
    version: string;
    validChallenges: Array<{
      id: string;
      description: string;
      payload: Record<string, unknown>;
      expectedHeader: string;
    }>;
    invalidChallenges: Array<{
      id: string;
      description: string;
      payload: Record<string, unknown>;
      expectedError: string;
    }>;
  };

  it('validates all validChallenges and roundtrip codecs', () => {
    for (const item of suite.validChallenges) {
      if (item.expectedHeader === 'PAYMENT-REQUIRED') {
        const challenge = PaymentChallenge.modelValidate(item.payload);
        expect(challenge.version).toBe('2.0');
        expect(challenge.challengeId).toBe(item.payload.challengeId);
        expect(challenge.amountBaseUnits).toBe(BigInt(item.payload.amount as string));

        const encoded = encodeChallenge(challenge);
        const decoded = decodeChallenge(encoded);
        expect(decoded.toJSON()).toEqual(challenge.toJSON());
      } else if (item.expectedHeader === 'PAYMENT-SIGNATURE') {
        const auth = PaymentAuthorization.modelValidate(item.payload);
        expect(auth.version).toBe('2.0');
        expect(auth.challengeId).toBe(item.payload.challengeId);

        const encoded = encodeAuthorization(auth);
        const decoded = decodeAuthorization(encoded);
        expect(decoded.toJSON()).toEqual(auth.toJSON());
      }
    }
  });

  it('rejects all invalidChallenges with the exact expectedError code', () => {
    for (const item of suite.invalidChallenges) {
      let caughtError: unknown;
      try {
        PaymentChallenge.modelValidate(item.payload);
      } catch (err) {
        caughtError = err;
      }
      expect(caughtError).toBeInstanceOf(X402ValidationError);
      expect((caughtError as X402ValidationError).code).toBe(item.expectedError as X402ErrorCode);
    }
  });
});
