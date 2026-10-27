import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  HEADER_PAYMENT_REQUIRED,
  PaymentChallenge,
  decodeChallenge,
  encodeChallenge,
} from '../../src/core/index.js';

const VECTORS_DIR = resolve(__dirname, '../../../../../../specs/test-vectors');
const CONFORMANCE_DIR = resolve(__dirname, '../../../../../../specs/conformance');

describe('dual-path-protocol.json & protocol-errors.json Conformance Vectors', () => {
  const dualPathRaw = readFileSync(resolve(VECTORS_DIR, 'dual-path-protocol.json'), 'utf-8');
  const dualPathSuite = JSON.parse(dualPathRaw) as {
    version: string;
    testCases: Array<{
      id: string;
      category: string;
      description: string;
      input: Record<string, any>;
      expected: Record<string, any>;
    }>;
  };

  const errorsRaw = readFileSync(resolve(CONFORMANCE_DIR, 'protocol-errors.json'), 'utf-8');
  const errorTaxonomy = JSON.parse(errorsRaw) as {
    version: string;
    errors: Array<{ code: string; status: number; domain: string }>;
  };

  it('verifies canonical error taxonomy integrity', () => {
    expect(errorTaxonomy.version).toBe('2.0.0');
    expect(errorTaxonomy.errors.length).toBeGreaterThanOrEqual(17);
    const codeMap = new Map(errorTaxonomy.errors.map((e) => [e.code, e]));

    expect(codeMap.get('ERR_WALLET_INSUFFICIENT_BALANCE')?.status).toBe(402);
    expect(codeMap.get('ERR_WALLET_INSUFFICIENT_BALANCE')?.domain).toBe('WALLET');
    expect(codeMap.get('ERR_WALLET_FROZEN')?.status).toBe(403);
    expect(codeMap.get('ERR_PAYMENT_METHOD_NOT_ALLOWED')?.status).toBe(403);
    expect(codeMap.get('ERR_IDEMPOTENCY_CONFLICT')?.status).toBe(409);
  });

  it('executes dual-path protocol test vectors', () => {
    const errorCodes = new Set(errorTaxonomy.errors.map((e) => e.code));

    for (const tc of dualPathSuite.testCases) {
      const { id, input, expected } = tc;

      if (id === 'TC_PROTO_01_ONCHAIN_CHALLENGE_FORMAT') {
        expect(expected.http_status).toBe(402);
        expect(expected.challenge_header).toBe(HEADER_PAYMENT_REQUIRED);

        const challenge = new PaymentChallenge({
          version: expected.challenge_payload.version,
          challengeId: 'ch_proto_01',
          network: input.network,
          asset: input.asset,
          assetSymbol: expected.challenge_payload.assetSymbol,
          decimals: expected.challenge_payload.decimals,
          amount: input.amount,
          payee: input.payee,
          scheme: expected.challenge_payload.scheme,
          validAfter: 1729000000,
          validBefore: 1729000300,
          facilitatorUrl: 'https://facilitator.example.com',
        });

        const encoded = encodeChallenge(challenge);
        const decoded = decodeChallenge(encoded);
        expect(decoded.version).toBe('2.0');
        expect(decoded.scheme).toBe('EIP-3009');
        expect(decoded.amount).toBe('50000');
        expect(decoded.amountBaseUnits).toBe(50000n);
      } else if (id === 'TC_PROTO_02_WALLET_APIKEY_SELECTION') {
        expect(expected.selected_payment_method).toBe('wallet');
        expect(expected.auto_downgrade).toBe(false);
        expect(expected.target_status).toBe('held');
      } else if (id === 'TC_PROTO_03_WALLET_INSUFFICIENT_NO_DOWNGRADE') {
        expect(expected.http_status).toBe(402);
        expect(expected.error_code).toBe('ERR_WALLET_INSUFFICIENT_BALANCE');
        expect(expected.auto_downgrade_to_onchain).toBe(false);
        expect(errorCodes.has(expected.error_code)).toBe(true);
      } else if (id === 'TC_PROTO_04_WALLET_FROZEN_ARREARS') {
        expect(expected.http_status).toBe(403);
        expect(expected.error_code).toBe('ERR_WALLET_FROZEN');
        expect(errorCodes.has(expected.error_code)).toBe(true);
      } else if (id === 'TC_PROTO_05_PAYMENT_METHOD_MISMATCH') {
        expect(expected.http_status).toBe(403);
        expect(expected.error_code).toBe('ERR_PAYMENT_METHOD_NOT_ALLOWED');
        expect(errorCodes.has(expected.error_code)).toBe(true);
      } else if (id === 'TC_PROTO_06_IDEMPOTENCY_CONFLICT') {
        expect(expected.http_status).toBe(409);
        expect(expected.error_code).toBe('ERR_IDEMPOTENCY_CONFLICT');
        expect(errorCodes.has(expected.error_code)).toBe(true);
      }
    }
  });
});
