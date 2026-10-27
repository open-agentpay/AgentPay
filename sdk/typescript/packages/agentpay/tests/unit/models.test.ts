import { describe, expect, it } from 'vitest';
import {
  PaymentAuthorization,
  PaymentChallenge,
  SignatureData,
  WalletAuthorization,
  X402ErrorCode,
  X402ValidationError,
} from '../../src/core/index.js';

function makeValidChallengeDict(): Record<string, unknown> {
  return {
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
  };
}

function makeValidAuthDict(): Record<string, unknown> {
  return {
    version: '2.0',
    challengeId: 'ch_01j7xyz890abcdef12345678',
    payer: '0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266',
    scheme: 'EIP-3009',
    nonce: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    validAfter: 1729000000,
    validBefore: 1729000300,
    signature: {
      v: 28,
      r: '0xd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83',
      s: '0x530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e0',
      serialized:
        'd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e01c',
    },
  };
}

describe('PaymentChallenge Model', () => {
  it('creates valid challenge and normalizes EVM addresses to lowercase', () => {
    const ch = new PaymentChallenge(makeValidChallengeDict() as any);
    expect(ch.version).toBe('2.0');
    expect(ch.challengeId).toBe('ch_01j7xyz890abcdef12345678');
    expect(ch.network).toBe('eip155:84532');
    expect(ch.asset).toBe('0x036cbd53842c5426634e7929541ec2318f3dcf7e');
    expect(ch.payee).toBe('0x70997970c51812dc3a010c7d01b50e0d17dc79c8');
    expect(ch.amount).toBe('50000');
    expect(ch.amountBaseUnits).toBe(50000n);
    expect(ch.decimals).toBe(6);
    expect(ch.validAfter).toBe(1729000000);
    expect(ch.validBefore).toBe(1729000300);
    expect(ch.facilitatorUrl).toBe('https://facilitator.example.com');
    expect(ch.resourceUri).toBe('/v1/market-data');
  });

  it('enforces immutability via Object.freeze', () => {
    const ch = new PaymentChallenge(makeValidChallengeDict() as any);
    expect(Object.isFrozen(ch)).toBe(true);
    expect(() => {
      (ch as any).amount = '100000';
    }).toThrow();
  });

  it('checks challenge expiration accurately', () => {
    const ch = new PaymentChallenge(makeValidChallengeDict() as any);
    expect(ch.isExpired(1729000200)).toBe(false);
    expect(ch.isExpired(1729000300)).toBe(false);
    expect(ch.isExpired(1729000301)).toBe(true);
  });

  it('converts challenge to PaymentRequest', () => {
    const ch = new PaymentChallenge(makeValidChallengeDict() as any);
    const req = ch.toPaymentRequest({ domain: 'example.com', ttlSeconds: 300 });
    expect(req.intentId).toBe(ch.challengeId);
    expect(req.amount).toBe(50000n);
    expect(req.payee).toBe(ch.payee);
    expect(req.asset).toBe(ch.asset);
    expect(req.network).toBe(ch.network);
    expect(req.domain).toBe('example.com');
    expect(req.ttlSeconds).toBe(300);
    expect(req.isAgentpayWallet).toBe(false);
  });

  it('rejects unsupported protocol version', () => {
    const data = { ...makeValidChallengeDict(), version: '1.0' };
    try {
      new PaymentChallenge(data as any);
      expect.fail('Expected X402ValidationError');
    } catch (err) {
      expect(err).toBeInstanceOf(X402ValidationError);
      expect((err as X402ValidationError).code).toBe(X402ErrorCode.UNSUPPORTED_VERSION);
    }
  });

  it('rejects empty challengeId', () => {
    const data = { ...makeValidChallengeDict(), challengeId: '   ' };
    try {
      new PaymentChallenge(data as any);
      expect.fail('Expected X402ValidationError');
    } catch (err) {
      expect(err).toBeInstanceOf(X402ValidationError);
      expect((err as X402ValidationError).code).toBe(X402ErrorCode.MISSING_REQUIRED_FIELD);
    }
  });

  it('rejects invalid CAIP-2 network', () => {
    const data = { ...makeValidChallengeDict(), network: 'base-testnet' };
    try {
      new PaymentChallenge(data as any);
      expect.fail('Expected X402ValidationError');
    } catch (err) {
      expect(err).toBeInstanceOf(X402ValidationError);
      expect((err as X402ValidationError).code).toBe(X402ErrorCode.NETWORK_NOT_WHITELISTED);
    }
  });

  it('rejects invalid EVM address', () => {
    const data = { ...makeValidChallengeDict(), asset: '0xnotanaddress' };
    try {
      new PaymentChallenge(data as any);
      expect.fail('Expected X402ValidationError');
    } catch (err) {
      expect(err).toBeInstanceOf(X402ValidationError);
      expect((err as X402ValidationError).code).toBe(X402ErrorCode.INVALID_ADDRESS);
    }
  });

  it('rejects zero, negative, float, or boolean amounts', () => {
    for (const badAmount of ['0', -100, 50.5, true, '9223372036854775808']) {
      const data = { ...makeValidChallengeDict(), amount: badAmount };
      try {
        new PaymentChallenge(data as any);
        expect.fail(`Expected X402ValidationError for amount=${String(badAmount)}`);
      } catch (err) {
        expect(err).toBeInstanceOf(X402ValidationError);
        expect((err as X402ValidationError).code).toBe(X402ErrorCode.INVALID_AMOUNT_FORMAT);
      }
    }
  });

  it('rejects inverted or equal time windows', () => {
    for (const [after, before] of [
      [1729000300, 1729000000],
      [1729000000, 1729000000],
    ] as const) {
      const data = { ...makeValidChallengeDict(), validAfter: after, validBefore: before };
      try {
        new PaymentChallenge(data as any);
        expect.fail('Expected X402ValidationError');
      } catch (err) {
        expect(err).toBeInstanceOf(X402ValidationError);
        expect((err as X402ValidationError).code).toBe(X402ErrorCode.INVALID_TIME_WINDOW);
      }
    }
  });

  it('supports agentpay-wallet scheme with CNY, USD, EUR currencies', () => {
    for (const currency of ['CNY', 'USD', 'EUR']) {
      const challenge = new PaymentChallenge({
        version: '2.0',
        scheme: 'agentpay-wallet',
        challengeId: '0x' + 'ab'.repeat(32),
        network: 'agentpay:internal-ledger',
        asset: currency,
        assetSymbol: currency,
        decimals: 6,
        amount: '350000',
        payee: 'agentpay:route:rt_cny_demo',
        validAfter: 1700000000,
        validBefore: 1700000300,
        resourceUri: '/api/v1/chat',
      });
      expect(challenge.asset).toBe(currency);
      const req = challenge.toPaymentRequest({ domain: 'api.example.com' });
      expect(req.asset).toBe(currency);
      expect(req.isAgentpayWallet).toBe(true);
    }
  });
});

describe('PaymentAuthorization & WalletAuthorization Models', () => {
  it('creates valid authorization and normalizes payer and nonce', () => {
    const auth = new PaymentAuthorization(makeValidAuthDict() as any);
    expect(auth.version).toBe('2.0');
    expect(auth.challengeId).toBe('ch_01j7xyz890abcdef12345678');
    expect(auth.payer).toBe('0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266');
    expect(auth.scheme).toBe('EIP-3009');
    expect(auth.nonce).toBe('0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef');
    expect(auth.signature).toBeInstanceOf(SignatureData);
    expect((auth.signature as SignatureData).v).toBe(28);
  });

  it('rejects invalid payer or nonce', () => {
    try {
      new PaymentAuthorization({ ...makeValidAuthDict(), payer: '0xinvalid' } as any);
      expect.fail('Expected INVALID_ADDRESS');
    } catch (err) {
      expect((err as X402ValidationError).code).toBe(X402ErrorCode.INVALID_ADDRESS);
    }

    try {
      new PaymentAuthorization({ ...makeValidAuthDict(), nonce: '0x1234' } as any);
      expect.fail('Expected INVALID_NONCE');
    } catch (err) {
      expect((err as X402ValidationError).code).toBe(X402ErrorCode.INVALID_NONCE);
    }
  });

  it('validates WalletAuthorization fields and requestBinding', () => {
    const walletAuth = new WalletAuthorization({
      challengeId: 'ch_wallet_01',
      nonce: '0x' + '11'.repeat(32),
      validAfter: 1700000000,
      validBefore: 1700000300,
      amount: '50000',
      resourceUri: '/v1/chat',
      requestBinding: 'sha256:' + 'a'.repeat(64),
    });
    expect(walletAuth.scheme).toBe('agentpay-wallet');
    expect(walletAuth.version).toBe('2.0');
  });
});

describe('SignatureData Model', () => {
  it('auto-populates serialized and compact hex strings', () => {
    const sig = new SignatureData({
      v: 28,
      r: '0xd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83',
      s: '0x530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e0',
    });
    expect(sig.serialized).toBe(
      'd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e01c',
    );
    expect(sig.compact).toBe('0x' + sig.serialized);
  });

  it('rejects invalid v value', () => {
    try {
      new SignatureData({
        v: 10,
        r: '0xd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83',
        s: '0x530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e0',
      });
      expect.fail('Expected INVALID_SIGNATURE');
    } catch (err) {
      expect((err as X402ValidationError).code).toBe(X402ErrorCode.INVALID_SIGNATURE);
    }
  });
});
