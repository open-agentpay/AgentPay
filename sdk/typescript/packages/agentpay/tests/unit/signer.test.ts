import { describe, expect, it } from 'vitest';
import {
  InvalidPrivateKeyError,
  LocalSigner,
  PaymentChallenge,
  ReservationMismatchError,
  ReservationToken,
  UnsupportedNetworkError,
  generateNonce,
  parseCaip2ChainId,
} from '../../src/index.js';

describe('LocalSigner Unit Tests', () => {
  const sampleChallenge = new PaymentChallenge({
    version: '2.0',
    challengeId: 'ch_01j7xyz890abcdef12345678',
    network: 'eip155:84532',
    asset: '0x036CbD53842c5426634e7929541eC2318f3dCF7e',
    assetSymbol: 'USDC',
    decimals: 6,
    amount: '50000',
    payee: '0x70997970C51812dc3A010C7d01b50e0d17dc79C8',
    scheme: 'EIP-3009',
    validAfter: 1729000000,
    validBefore: 1729000300,
    facilitatorUrl: 'https://facilitator.example.com',
    resourceUri: '/v1/market-data',
  });

  const sampleReservation = new ReservationToken({
    reservationId: 'res_01j7xyz123',
    intentId: 'ch_01j7xyz890abcdef12345678',
    policyId: 'pol_default',
    amount: 50000n,
    expiresAt: 1729000360,
    createdAt: 1729000000,
  });

  describe('Basics & Key Management', () => {
    it('initializes from hex string with or without 0x prefix', () => {
      const privKeyWith0x = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';
      const signer1 = new LocalSigner(privKeyWith0x);
      expect(signer1.address).toBe('0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266');

      const privKeyWithout0x = 'ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';
      const signer2 = new LocalSigner(privKeyWithout0x);
      expect(signer2.address).toBe('0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266');
    });

    it('initializes from Uint8Array bytes', () => {
      const bytes = Buffer.from('ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80', 'hex');
      const signer = new LocalSigner(new Uint8Array(bytes));
      expect(signer.address).toBe('0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266');
    });

    it('generates random signer with valid EVM address', () => {
      const signer1 = LocalSigner.generate();
      const signer2 = LocalSigner.generate();
      expect(signer1.address).not.toBe(signer2.address);
      expect(signer1.address).toMatch(/^0x[0-9a-f]{40}$/);
      expect(signer2.address).toMatch(/^0x[0-9a-f]{40}$/);
    });

    it('rejects invalid private keys', () => {
      expect(() => new LocalSigner('0xinvalidhex')).toThrow(InvalidPrivateKeyError);
      expect(() => new LocalSigner('short')).toThrow(InvalidPrivateKeyError);
      expect(() => new LocalSigner(new Uint8Array(16))).toThrow(InvalidPrivateKeyError);
      expect(() => new LocalSigner(123 as unknown as string)).toThrow(InvalidPrivateKeyError);
    });

    it('strictly redacts private key in toString, inspect, and toJSON', () => {
      const privKey = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';
      const signer = new LocalSigner(privKey);

      const str = signer.toString();
      const inspectStr = (signer as any)[Symbol.for('nodejs.util.inspect.custom')]();
      const json = JSON.stringify(signer);

      expect(str).not.toContain('ac0974');
      expect(inspectStr).not.toContain('ac0974');
      expect(json).not.toContain('ac0974');

      expect(str).toContain(signer.address);
      expect(inspectStr).toContain(signer.address);
      expect(json).toContain(signer.address);
    });

    it('generates 32-byte hex nonce with 0x prefix', () => {
      const n1 = generateNonce();
      const n2 = generateNonce();
      expect(n1).not.toBe(n2);
      expect(n1).toMatch(/^0x[0-9a-fA-F]{64}$/);
      expect(n2).toMatch(/^0x[0-9a-fA-F]{64}$/);
    });

    it('parses valid CAIP-2 eip155 chain IDs and rejects non-eip155', () => {
      expect(parseCaip2ChainId('eip155:1')).toBe(1);
      expect(parseCaip2ChainId('eip155:84532')).toBe(84532);
      expect(parseCaip2ChainId('eip155:421614')).toBe(421614);

      expect(() => parseCaip2ChainId('solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp')).toThrow(UnsupportedNetworkError);
      expect(() => parseCaip2ChainId('cosmos:cosmoshub-4')).toThrow(UnsupportedNetworkError);
    });
  });

  describe('Reservation Guardrails', () => {
    it('refuses to sign when reservation intentId does not match challengeId', async () => {
      const signer = LocalSigner.generate();
      const badReservation = new ReservationToken({
        reservationId: sampleReservation.reservationId,
        intentId: 'mismatched_intent_id',
        policyId: sampleReservation.policyId,
        amount: sampleReservation.amount,
        expiresAt: sampleReservation.expiresAt,
        createdAt: sampleReservation.createdAt,
      });

      await expect(signer.signPaymentAuthorization(sampleChallenge, badReservation)).rejects.toThrow(
        ReservationMismatchError,
      );
    });

    it('refuses to sign when reservation amount does not match challenge amount', async () => {
      const signer = LocalSigner.generate();
      const badReservation = new ReservationToken({
        reservationId: sampleReservation.reservationId,
        intentId: sampleReservation.intentId,
        policyId: sampleReservation.policyId,
        amount: sampleReservation.amount + 1000n,
        expiresAt: sampleReservation.expiresAt,
        createdAt: sampleReservation.createdAt,
      });

      await expect(signer.signPaymentAuthorization(sampleChallenge, badReservation)).rejects.toThrow(
        ReservationMismatchError,
      );
    });

    it('refuses to sign when reservation expiresAt is before challenge validBefore', async () => {
      const signer = LocalSigner.generate();
      const badReservation = new ReservationToken({
        reservationId: sampleReservation.reservationId,
        intentId: sampleReservation.intentId,
        policyId: sampleReservation.policyId,
        amount: sampleReservation.amount,
        expiresAt: sampleChallenge.validBefore - 10,
        createdAt: sampleReservation.createdAt,
      });

      await expect(signer.signPaymentAuthorization(sampleChallenge, badReservation)).rejects.toThrow(
        ReservationMismatchError,
      );
    });

    it('refuses to sign on unsupported network', async () => {
      const signer = LocalSigner.generate();
      const nonEvmChallenge = new PaymentChallenge({
        version: '2.0',
        challengeId: sampleReservation.intentId,
        network: 'solana:mainnet',
        asset: '0x036CbD53842c5426634e7929541eC2318f3dCF7e',
        assetSymbol: 'USDC',
        decimals: 6,
        amount: sampleReservation.amount.toString(),
        payee: '0x70997970C51812dc3A010C7d01b50e0d17dc79C8',
        scheme: 'EIP-3009',
        validAfter: 1729000000,
        validBefore: 1729000300,
        facilitatorUrl: 'https://facilitator.example.com',
      });

      await expect(signer.signPaymentAuthorization(nonEvmChallenge, sampleReservation)).rejects.toThrow(
        UnsupportedNetworkError,
      );
    });
  });

  describe('Custom Token Name Mapping', () => {
    it('uses custom token name map when resolving token name', () => {
      const customAsset = '0x1111111111111111111111111111111111111111';
      const signer = new LocalSigner('0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80', {
        tokenNameMap: {
          [customAsset]: 'My Custom Stablecoin',
        },
      });

      const challenge = new PaymentChallenge({
        version: '2.0',
        challengeId: 'ch_custom',
        network: 'eip155:84532',
        asset: customAsset,
        amount: '1000',
        payee: '0x70997970C51812dc3A010C7d01b50e0d17dc79C8',
        scheme: 'EIP-3009',
        validAfter: 1729000000,
        validBefore: 1729000300,
        facilitatorUrl: 'https://facilitator.example.com',
      });

      expect(signer.resolveTokenName(challenge)).toBe('My Custom Stablecoin');
    });
  });
});
