import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  LocalSigner,
  PaymentAuthorization,
  PaymentChallenge,
  ReservationToken,
  SignatureData,
} from '../../src/index.js';

const VECTORS_DIR = resolve(__dirname, '../../../../../../specs/test-vectors');

interface Eip712VectorSuite {
  version: string;
  suite: string;
  description: string;
  domain: {
    name: string;
    version: string;
    chainId: number;
    verifyingContract: string;
  };
  types: Record<string, any>;
  testWallet: {
    privateKey: string;
    address: string;
  };
  testCases: Array<{
    id: string;
    description: string;
    message?: {
      from: string;
      to: string;
      value: string;
      validAfter: string;
      validBefore: string;
      nonce: string;
    };
    expectedSignature?: {
      v: number;
      r: string;
      s: string;
      serialized: string;
      recoveredAddress: string;
    };
    tamperedMessage?: {
      from: string;
      to: string;
      value: string;
      validAfter: string;
      validBefore: string;
      nonce: string;
    };
    signatureFromTC03?: string;
    replayDomain?: {
      name: string;
      version: string;
      chainId: number;
      verifyingContract: string;
    };
    signatureFromTC01?: string;
    expectedBehavior?: string;
  }>;
}

describe('EIP-712 / EIP-3009 Authorization Conformance Vectors', () => {
  const vectorRaw = readFileSync(resolve(VECTORS_DIR, 'eip712-authorization.json'), 'utf-8');
  const suite = JSON.parse(vectorRaw) as Eip712VectorSuite;

  const signer = new LocalSigner(suite.testWallet.privateKey);

  it('matches test wallet address in vector suite', () => {
    expect(signer.address).toBe(suite.testWallet.address.toLowerCase());
  });

  for (const tc of suite.testCases) {
    if (tc.expectedSignature && tc.message) {
      it(`passes positive conformance case: ${tc.id}`, async () => {
        const msg = tc.message!;
        const expectedSig = tc.expectedSignature!;

        const challenge = new PaymentChallenge({
          version: '2.0',
          challengeId: `ch_${tc.id}`,
          network: `eip155:${suite.domain.chainId}`,
          asset: suite.domain.verifyingContract,
          assetSymbol: 'USDC',
          decimals: 6,
          amount: msg.value,
          payee: msg.to,
          scheme: 'EIP-3009',
          validAfter: Number.parseInt(msg.validAfter, 10),
          validBefore: Number.parseInt(msg.validBefore, 10),
          facilitatorUrl: 'https://facilitator.example.com',
        });

        const reservation = new ReservationToken({
          reservationId: `res_${tc.id}`,
          intentId: challenge.challengeId,
          policyId: 'pol_default',
          amount: BigInt(msg.value),
          expiresAt: Number.parseInt(msg.validBefore, 10) + 60,
          createdAt: Number.parseInt(msg.validAfter, 10),
        });

        const auth = await signer.signPaymentAuthorization(challenge, reservation, {
          nonce: msg.nonce,
          tokenName: suite.domain.name,
          tokenVersion: suite.domain.version,
        });

        expect(auth.payer).toBe(signer.address);
        expect(auth.nonce.toLowerCase()).toBe(msg.nonce.toLowerCase());
        expect(auth.validAfter).toBe(Number.parseInt(msg.validAfter, 10));
        expect(auth.validBefore).toBe(Number.parseInt(msg.validBefore, 10));

        const sig = auth.signature as SignatureData;
        expect(sig.v).toBe(expectedSig.v);
        expect(BigInt(sig.r)).toBe(BigInt(expectedSig.r));
        expect(BigInt(sig.s)).toBe(BigInt(expectedSig.s));
        expect(sig.serialized.toLowerCase()).toBe(expectedSig.serialized.toLowerCase());

        // Verify with static verifyAuthorization
        const isValid = await LocalSigner.verifyAuthorization(auth, challenge, {
          tokenName: suite.domain.name,
          tokenVersion: suite.domain.version,
        });
        expect(isValid).toBe(true);
      });
    }

    if (tc.id === 'TC_EIP712_NEGATIVE_01_TAMPERED_AMOUNT') {
      it('fails verification on tampered amount (TC_EIP712_NEGATIVE_01)', async () => {
        const tampered = tc.tamperedMessage!;
        const tamperedChallenge = new PaymentChallenge({
          version: '2.0',
          challengeId: 'ch_tampered',
          network: `eip155:${suite.domain.chainId}`,
          asset: suite.domain.verifyingContract,
          assetSymbol: 'USDC',
          decimals: 6,
          amount: tampered.value,
          payee: tampered.to,
          scheme: 'EIP-3009',
          validAfter: Number.parseInt(tampered.validAfter, 10),
          validBefore: Number.parseInt(tampered.validBefore, 10),
          facilitatorUrl: 'https://facilitator.example.com',
        });

        const auth = new PaymentAuthorization({
          version: '2.0',
          challengeId: 'ch_tampered',
          payer: suite.testWallet.address,
          scheme: 'EIP-3009',
          nonce: tampered.nonce,
          validAfter: Number.parseInt(tampered.validAfter, 10),
          validBefore: Number.parseInt(tampered.validBefore, 10),
          signature: tc.signatureFromTC03!,
        });

        const isValid = await LocalSigner.verifyAuthorization(auth, tamperedChallenge, {
          tokenName: suite.domain.name,
          tokenVersion: suite.domain.version,
        });
        expect(isValid).toBe(false);
      });
    }

    if (tc.id === 'TC_EIP712_NEGATIVE_02_CROSS_CHAIN_REPLAY') {
      it('fails verification on cross-chain replay (TC_EIP712_NEGATIVE_02)', async () => {
        const replay = tc.replayDomain!;
        // Original TC01 message values on replayed chain
        const replayedChallenge = new PaymentChallenge({
          version: '2.0',
          challengeId: 'ch_replay',
          network: `eip155:${replay.chainId}`,
          asset: replay.verifyingContract,
          assetSymbol: 'USDC',
          decimals: 6,
          amount: '100000',
          payee: '0x70997970C51812dc3A010C7d01b50e0d17dc79C8',
          scheme: 'EIP-3009',
          validAfter: 1729000000,
          validBefore: 1729000300,
          facilitatorUrl: 'https://facilitator.example.com',
        });

        const auth = new PaymentAuthorization({
          version: '2.0',
          challengeId: 'ch_replay',
          payer: suite.testWallet.address,
          scheme: 'EIP-3009',
          nonce: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
          validAfter: 1729000000,
          validBefore: 1729000300,
          signature: tc.signatureFromTC01!,
        });

        const isValid = await LocalSigner.verifyAuthorization(auth, replayedChallenge, {
          tokenName: replay.name,
          tokenVersion: replay.version,
        });
        expect(isValid).toBe(false);
      });
    }
  }
});
