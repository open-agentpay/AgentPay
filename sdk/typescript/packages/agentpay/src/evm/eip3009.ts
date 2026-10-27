/**
 * EIP-3009 ABI Encoding and Decoding Utilities for AgentPay.
 *
 * Implements calldata builders and result decoders for FiatTokenV2_2 / EIP-3009:
 * - transferWithAuthorization
 * - authorizationState
 * - balanceOf
 */

import { type PaymentAuthorization, SignatureData } from '../core/models.js';
import {
  AUTHORIZATION_STATE_SELECTOR,
  BALANCE_OF_SELECTOR,
  TRANSFER_WITH_AUTHORIZATION_SELECTOR,
} from './constants.js';

function hexToBytes32(val: string | Uint8Array): Uint8Array {
  if (val instanceof Uint8Array) {
    if (val.byteLength !== 32) {
      throw new Error(`Expected 32 bytes, got ${val.byteLength}`);
    }
    return new Uint8Array(val);
  }
  const clean = val.startsWith('0x') ? val.slice(2) : val;
  if (clean.length > 64 || !/^[0-9a-fA-F]*$/.test(clean)) {
    throw new Error(`Invalid bytes32 hex value: ${val}`);
  }
  return new Uint8Array(Buffer.from(clean.padStart(64, '0'), 'hex'));
}

function encodeAddressWord(address: string): Uint8Array {
  const clean = address.startsWith('0x') ? address.slice(2) : address;
  if (clean.length !== 40 || !/^[0-9a-fA-F]{40}$/.test(clean)) {
    throw new Error(`Invalid EVM address: ${address}`);
  }
  return new Uint8Array(Buffer.from(clean.toLowerCase().padStart(64, '0'), 'hex'));
}

function encodeUint256Word(value: bigint | number): Uint8Array {
  const big = typeof value === 'bigint' ? value : BigInt(value);
  if (big < 0n) {
    throw new Error(`uint256 cannot be negative: ${big.toString()}`);
  }
  const hex = big.toString(16);
  if (hex.length > 64) {
    throw new Error(`uint256 overflow: ${big.toString()}`);
  }
  return new Uint8Array(Buffer.from(hex.padStart(64, '0'), 'hex'));
}

function concatBytes(chunks: Uint8Array[]): Uint8Array {
  const total = chunks.reduce((acc, c) => acc + c.byteLength, 0);
  const out = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    out.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return out;
}

function toRawBytes(data: Uint8Array | string): Uint8Array {
  if (data instanceof Uint8Array) {
    return data;
  }
  const clean = data.startsWith('0x') ? data.slice(2) : data;
  return new Uint8Array(Buffer.from(clean, 'hex'));
}

export interface TransferWithAuthorizationParams {
  from: string;
  to: string;
  value: bigint | number;
  validAfter: bigint | number;
  validBefore: bigint | number;
  nonce: string | Uint8Array;
  v: number;
  r: string | Uint8Array;
  s: string | Uint8Array;
}

export interface DecodedTransferWithAuthorization {
  from: `0x${string}`;
  to: `0x${string}`;
  value: bigint;
  validAfter: bigint;
  validBefore: bigint;
  nonce: `0x${string}`;
  v: number;
  r: `0x${string}`;
  s: `0x${string}`;
}

/**
 * Encode calldata for ERC-3009 transferWithAuthorization.
 * Selector: 0xe3ee160e (4 bytes) + 9 * 32 bytes = 292 bytes total.
 */
export function encodeTransferWithAuthorization(
  params: TransferWithAuthorizationParams,
): Uint8Array {
  return concatBytes([
    TRANSFER_WITH_AUTHORIZATION_SELECTOR,
    encodeAddressWord(params.from),
    encodeAddressWord(params.to),
    encodeUint256Word(params.value),
    encodeUint256Word(params.validAfter),
    encodeUint256Word(params.validBefore),
    hexToBytes32(params.nonce),
    encodeUint256Word(params.v),
    hexToBytes32(params.r),
    hexToBytes32(params.s),
  ]);
}

export function encodeTransferFromAuthorization(
  authorization: PaymentAuthorization,
  toAddress: string,
  value: bigint | number,
): Uint8Array {
  if (!(authorization.signature instanceof SignatureData)) {
    throw new Error('PaymentAuthorization.signature must be a SignatureData instance');
  }
  return encodeTransferWithAuthorization({
    from: authorization.payer,
    to: toAddress,
    value,
    validAfter: authorization.validAfter,
    validBefore: authorization.validBefore,
    nonce: authorization.nonce,
    v: authorization.signature.v,
    r: authorization.signature.r,
    s: authorization.signature.s,
  });
}

export function decodeTransferWithAuthorization(
  calldata: Uint8Array | string,
): DecodedTransferWithAuthorization {
  const raw = toRawBytes(calldata);
  if (raw.byteLength < 292) {
    throw new Error(`Calldata too short: expected 292 bytes, got ${raw.byteLength}`);
  }
  const selector = Buffer.from(raw.subarray(0, 4)).toString('hex');
  const expectedSelector = Buffer.from(TRANSFER_WITH_AUTHORIZATION_SELECTOR).toString('hex');
  if (selector !== expectedSelector) {
    throw new Error(`Selector mismatch: expected ${expectedSelector}, got ${selector}`);
  }

  const readWordHex = (index: number): string => {
    const start = 4 + index * 32;
    return Buffer.from(raw.subarray(start, start + 32)).toString('hex');
  };

  return {
    from: `0x${readWordHex(0).slice(24).toLowerCase()}`,
    to: `0x${readWordHex(1).slice(24).toLowerCase()}`,
    value: BigInt(`0x${readWordHex(2)}`),
    validAfter: BigInt(`0x${readWordHex(3)}`),
    validBefore: BigInt(`0x${readWordHex(4)}`),
    nonce: `0x${readWordHex(5).toLowerCase()}`,
    v: Number(BigInt(`0x${readWordHex(6)}`)),
    r: `0x${readWordHex(7).toLowerCase()}`,
    s: `0x${readWordHex(8).toLowerCase()}`,
  };
}

export function encodeAuthorizationState(authorizer: string, nonce: string | Uint8Array): Uint8Array {
  return concatBytes([
    AUTHORIZATION_STATE_SELECTOR,
    encodeAddressWord(authorizer),
    hexToBytes32(nonce),
  ]);
}

export function decodeAuthorizationStateResult(data: Uint8Array | string): boolean {
  const raw = toRawBytes(data);
  if (raw.byteLength < 32) {
    return false;
  }
  const wordHex = Buffer.from(raw.subarray(0, 32)).toString('hex');
  return BigInt(`0x${wordHex}`) !== 0n;
}

export function encodeBalanceOf(account: string): Uint8Array {
  return concatBytes([BALANCE_OF_SELECTOR, encodeAddressWord(account)]);
}

export function decodeBalanceOfResult(data: Uint8Array | string): bigint {
  const raw = toRawBytes(data);
  if (raw.byteLength < 32) {
    return 0n;
  }
  const wordHex = Buffer.from(raw.subarray(0, 32)).toString('hex');
  return BigInt(`0x${wordHex}`);
}
