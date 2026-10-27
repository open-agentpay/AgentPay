import { describe, expect, it } from 'vitest';
import {
  HEADER_PAYMENT_SIGNATURE,
  PaymentAuthorization,
  PaymentChallenge,
  X402DecodeError,
  X402ErrorCode,
  X402ValidationError,
  base64urlDecode,
  base64urlEncode,
  decodeAuthorization,
  decodeChallenge,
  encodeAuthorization,
  encodeChallenge,
  extractAuthorizationFromHeaders,
  extractChallengeFromHeaders,
  injectAuthorizationToHeaders,
} from '../../src/core/index.js';
import {
  ARBITRUM_SEPOLIA,
  ARBITRUM_SEPOLIA_CAIP2,
  ARBITRUM_SEPOLIA_CHAIN_ID,
  decodeAuthorizationStateResult,
  decodeBalanceOfResult,
  decodeTransferWithAuthorization,
  encodeAuthorizationState,
  encodeBalanceOf,
  encodeTransferWithAuthorization,
  getNetworkSpec,
} from '../../src/evm/index.js';

function makeSampleChallenge(): PaymentChallenge {
  return new PaymentChallenge({
    version: '2.0',
    challengeId: 'ch_test123',
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
}

function makeSampleAuth(): PaymentAuthorization {
  return new PaymentAuthorization({
    version: '2.0',
    challengeId: 'ch_test123',
    payer: '0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266',
    scheme: 'EIP-3009',
    nonce: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    validAfter: 1729000000,
    validBefore: 1729000300,
    signature: '0xd90ab6b3...serialized',
  });
}

describe('Base64URL & x402 Header Codecs', () => {
  it('encodes and decodes unpadded Base64URL accurately', () => {
    const raw = Buffer.from('{"hello": "world", "price": 100}', 'utf-8');
    const encoded = base64urlEncode(raw);
    expect(encoded.includes('=')).toBe(false);
    const decoded = base64urlDecode(encoded);
    expect(Buffer.from(decoded).equals(raw)).toBe(true);
  });

  it('roundtrips PaymentChallenge in Base64URL and raw JSON modes', () => {
    const ch = makeSampleChallenge();
    const b64 = encodeChallenge(ch, { base64Url: true });
    expect(b64.startsWith('{')).toBe(false);
    expect(decodeChallenge(b64).toJSON()).toEqual(ch.toJSON());

    const rawJson = encodeChallenge(ch, { base64Url: false });
    expect(rawJson.startsWith('{')).toBe(true);
    expect(decodeChallenge(rawJson).toJSON()).toEqual(ch.toJSON());
  });

  it('rejects malformed or empty headers with X402DecodeError(MALFORMED_HEADER)', () => {
    for (const bad of ['!!!not_base64_or_json!!!', '   ', '']) {
      try {
        decodeChallenge(bad);
        expect.fail('Expected X402DecodeError');
      } catch (err) {
        expect(err).toBeInstanceOf(X402DecodeError);
        expect((err as X402DecodeError).code).toBe(X402ErrorCode.MALFORMED_HEADER);
      }
    }
  });

  it('raises X402ValidationError(MISSING_REQUIRED_FIELD) on incomplete JSON header', () => {
    const incomplete = base64urlEncode(
      Buffer.from(
        JSON.stringify({
          version: '2.0',
          challengeId: 'ch_bad',
          network: 'eip155:84532',
        }),
        'utf-8',
      ),
    );
    try {
      decodeChallenge(incomplete);
      expect.fail('Expected X402ValidationError');
    } catch (err) {
      expect(err).toBeInstanceOf(X402ValidationError);
      expect((err as X402ValidationError).code).toBe(X402ErrorCode.MISSING_REQUIRED_FIELD);
    }
  });

  it('extracts and injects headers case-insensitively (both Record and Fetch Headers)', () => {
    const ch = makeSampleChallenge();
    const auth = makeSampleAuth();

    const recordHeaders: Record<string, string> = {
      'Content-Type': 'application/json',
      'payment-required': encodeChallenge(ch),
    };
    const extractedCh = extractChallengeFromHeaders(recordHeaders);
    expect(extractedCh?.toJSON()).toEqual(ch.toJSON());

    injectAuthorizationToHeaders(recordHeaders, auth);
    expect(recordHeaders[HEADER_PAYMENT_SIGNATURE]).toBeDefined();
    const extractedAuth = extractAuthorizationFromHeaders(recordHeaders);
    expect(extractedAuth?.toJSON()).toEqual(auth.toJSON());

    // Web Fetch Headers instance
    const fetchHeaders = new Headers({
      'payment-required': encodeChallenge(ch),
    });
    expect(extractChallengeFromHeaders(fetchHeaders)?.challengeId).toBe('ch_test123');
    injectAuthorizationToHeaders(fetchHeaders, auth);
    expect(extractAuthorizationFromHeaders(fetchHeaders)?.challengeId).toBe('ch_test123');
  });
});

describe('EVM Constants & EIP-3009 Calldata Codecs', () => {
  it('resolves Arbitrum Sepolia network spec by CAIP-2, chain ID, or alias', () => {
    expect(getNetworkSpec(ARBITRUM_SEPOLIA_CAIP2)).toBe(ARBITRUM_SEPOLIA);
    expect(getNetworkSpec(ARBITRUM_SEPOLIA_CHAIN_ID)).toBe(ARBITRUM_SEPOLIA);
    expect(getNetworkSpec('arbitrum-sepolia')).toBe(ARBITRUM_SEPOLIA);
    expect(getNetworkSpec('unknown-chain')).toBeUndefined();
  });

  it('encodes and decodes transferWithAuthorization calldata (292 bytes)', () => {
    const from = '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266';
    const to = '0x70997970c51812dc3a010c7d01b50e0d17dc79c8';
    const value = 50000n;
    const validAfter = 1729000000n;
    const validBefore = 1729000300n;
    const nonce = '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef';
    const v = 28;
    const r = '0xd90ab6b36f43fea017b168106d443b874dbeaeec24a4c416b7abc38afa9baa83';
    const s = '0x530e805bb1a29839651fac0cb1f3f1c05a7c659760488173ed1fd582819467e0';

    const calldata = encodeTransferWithAuthorization({
      from,
      to,
      value,
      validAfter,
      validBefore,
      nonce,
      v,
      r,
      s,
    });
    expect(calldata.byteLength).toBe(292);
    expect(Buffer.from(calldata.subarray(0, 4)).toString('hex')).toBe('e3ee160e');

    const decoded = decodeTransferWithAuthorization(calldata);
    expect(decoded.from.toLowerCase()).toBe(from);
    expect(decoded.to.toLowerCase()).toBe(to);
    expect(decoded.value).toBe(value);
    expect(decoded.validAfter).toBe(validAfter);
    expect(decoded.validBefore).toBe(validBefore);
    expect(decoded.nonce).toBe(nonce);
    expect(decoded.v).toBe(v);
    expect(decoded.r).toBe(r);
    expect(decoded.s).toBe(s);
  });

  it('encodes and decodes authorizationState and balanceOf', () => {
    const authStateCall = encodeAuthorizationState(
      '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266',
      '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    );
    expect(authStateCall.byteLength).toBe(68);
    expect(Buffer.from(authStateCall.subarray(0, 4)).toString('hex')).toBe('e94a0102');

    const trueWord = '0x' + '00'.repeat(31) + '01';
    const falseWord = '0x' + '00'.repeat(32);
    expect(decodeAuthorizationStateResult(trueWord)).toBe(true);
    expect(decodeAuthorizationStateResult(falseWord)).toBe(false);

    const balCall = encodeBalanceOf('0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266');
    expect(balCall.byteLength).toBe(36);
    const balResult = '0x' + (1500000n).toString(16).padStart(64, '0');
    expect(decodeBalanceOfResult(balResult)).toBe(1500000n);
  });
});
