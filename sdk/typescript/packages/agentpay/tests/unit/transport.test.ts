import { describe, expect, it } from 'vitest';
import { AgentPayClient } from '../../src/client/client.js';
import { LocalSigner } from '../../src/client/signer.js';
import {
  AgentPayTransport,
  ALLOW_UNMETERED_HEADER,
  computeRequestBinding,
  createAgentPayFetch,
} from '../../src/client/transport.js';
import {
  decodeAuthorization,
  encodeChallenge,
  HEADER_PAYMENT_REQUIRED,
  HEADER_PAYMENT_SIGNATURE,
} from '../../src/core/codecs.js';
import { PaymentChallenge } from '../../src/core/models.js';
import { createBuyerConnection, initBuyerDatabase } from '../../src/policy/db.js';
import { PolicyErrorCode, PolicyViolationError } from '../../src/policy/errors.js';
import { LedgerEntryState, PolicyConfig, type PolicyConfigInput } from '../../src/policy/models.js';
import { SQLitePolicyEngine } from '../../src/policy/sqlite-engine.js';

const TEST_PRIVATE_KEY = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';

function setupEnvironment(overrides?: Partial<PolicyConfigInput>, currentTime = 1729000100) {
  const signer = new LocalSigner(TEST_PRIVATE_KEY);
  const db = createBuyerConnection(':memory:');
  initBuyerDatabase(db);

  const config = new PolicyConfig({
    id: 'pol_test',
    name: 'Test Transport Policy',
    walletAddress: signer.address,
    maxPerTxBaseUnits: 100_000n, // 0.1 USDC cap
    rollingBudget24hBaseUnits: 1_000_000n, // 1.0 USDC budget
    whitelistPayees: [
      '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
      '0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc',
    ],
    whitelistAssets: ['0x036cbd53842c5426634e7929541ec2318f3dcf7e'],
    whitelistNetworks: ['eip155:84532'],
    whitelistDomains: ['api.example.com'],
    ...overrides,
  });

  const clock = () => currentTime;
  const policy = new SQLitePolicyEngine(db, config, clock);

  return { signer, db, config, clock, policy };
}

function createStandardChallenge(overrides?: Partial<PaymentChallenge>): PaymentChallenge {
  return new PaymentChallenge({
    version: '2.0',
    challengeId: 'ch_transport_01',
    network: 'eip155:84532',
    asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
    assetSymbol: 'USDC',
    decimals: 6,
    amount: '50000', // 0.05 USDC
    payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
    scheme: 'EIP-3009',
    validAfter: 1729000000,
    validBefore: 1729000300,
    facilitatorUrl: 'https://facilitator.example.com',
    resourceUri: '/v1/market-data',
    ...overrides,
  });
}

describe('AgentPayTransport & AgentPayClient', () => {
  it('passes through unprotected endpoints without payment', async () => {
    const { signer, policy, clock } = setupEnvironment();
    const challenge = createStandardChallenge();

    let requestCount = 0;
    const mockFetch: typeof globalThis.fetch = async (input, init) => {
      requestCount++;
      return new Response(JSON.stringify({ message: 'free' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    };

    const client = new AgentPayClient({
      signer,
      policy,
      clock,
      baseFetch: mockFetch,
    });

    const res = await client.get('https://api.example.com/public');
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data).toEqual({ message: 'free' });
    expect(requestCount).toBe(1);
  });

  it('automatically catches 402, signs challenge, replays, and commits budget in SQLite', async () => {
    const { signer, policy, clock, db } = setupEnvironment();
    const challenge = createStandardChallenge();
    const paymentsReceived: any[] = [];

    const mockFetch: typeof globalThis.fetch = async (input, init) => {
      const headers = new Headers(init?.headers);
      const sigHeader = headers.get(HEADER_PAYMENT_SIGNATURE);

      if (!sigHeader) {
        return new Response(JSON.stringify({ error: 'Payment Required' }), {
          status: 402,
          headers: {
            [HEADER_PAYMENT_REQUIRED]: encodeChallenge(challenge),
            'Content-Type': 'application/json',
          },
        });
      }

      const auth = decodeAuthorization(sigHeader);
      const isValid = await LocalSigner.verifyAuthorization(auth, challenge);
      if (!isValid) {
        return new Response(JSON.stringify({ error: 'Invalid signature' }), { status: 401 });
      }

      paymentsReceived.push({ payer: auth.payer, amount: challenge.amount });
      return new Response(JSON.stringify({ data: 'Protected Data', receipt: auth.challengeId }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    };

    const client = new AgentPayClient({
      signer,
      policy,
      clock,
      baseFetch: mockFetch,
    });

    const res = await client.get('https://api.example.com/v1/market-data');
    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.data).toBe('Protected Data');
    expect(paymentsReceived).toHaveLength(1);
    expect(paymentsReceived[0].payer).toBe(signer.address);

    // Verify budget committed in SQLite ledger
    const row = db
      .prepare('SELECT state, amount_base_units FROM ap_budget_ledger WHERE intent_id = ?')
      .get(challenge.challengeId) as any;
    expect(row).toBeDefined();
    expect(row.state).toBe(LedgerEntryState.COMMITTED);
    expect(BigInt(row.amount_base_units)).toBe(50_000n);
  });

  it('handles policy rejection: returns 402 when raiseOnPolicyRejection=false, throws when true', async () => {
    const { signer, policy, clock } = setupEnvironment();
    // Exceeds maxPerTxBaseUnits of 100,000
    const expensiveChallenge = createStandardChallenge({
      challengeId: 'ch_expensive',
      amount: '500000',
    });

    const mockFetch: typeof globalThis.fetch = async () => {
      return new Response(JSON.stringify({ error: 'Payment Required' }), {
        status: 402,
        headers: {
          [HEADER_PAYMENT_REQUIRED]: encodeChallenge(expensiveChallenge),
        },
      });
    };

    // Mode A: raiseOnPolicyRejection: false (default)
    const clientA = new AgentPayClient({
      signer,
      policy,
      clock,
      baseFetch: mockFetch,
      raiseOnPolicyRejection: false,
    });
    const res = await clientA.get('https://api.example.com/v1/market-data');
    expect(res.status).toBe(402);

    // Mode B: raiseOnPolicyRejection: true
    const clientB = new AgentPayClient({
      signer,
      policy,
      clock,
      baseFetch: mockFetch,
      raiseOnPolicyRejection: true,
    });
    await expect(clientB.get('https://api.example.com/v1/market-data')).rejects.toThrow(PolicyViolationError);
  });

  it('financial redline #6: upstream 500 after wire dispatch MUST NOT auto-release', async () => {
    const { signer, policy, clock, db } = setupEnvironment();
    const challenge = createStandardChallenge();

    const mockFetch: typeof globalThis.fetch = async (input, init) => {
      const headers = new Headers(init?.headers);
      const sigHeader = headers.get(HEADER_PAYMENT_SIGNATURE);

      if (!sigHeader) {
        return new Response(JSON.stringify({ error: 'Payment Required' }), {
          status: 402,
          headers: {
            [HEADER_PAYMENT_REQUIRED]: encodeChallenge(challenge),
          },
        });
      }

      // Upstream server crashes on retry with 500 Internal Server Error
      return new Response(JSON.stringify({ error: 'Upstream gateway timeout' }), {
        status: 500,
      });
    };

    const client = new AgentPayClient({
      signer,
      policy,
      clock,
      baseFetch: mockFetch,
    });

    const res = await client.get('https://api.example.com/v1/market-data');
    expect(res.status).toBe(500);

    // CRITICAL REDLINE: Reservation must REMAIN in RESERVED state, NEVER RELEASED!
    const row = db
      .prepare('SELECT state FROM ap_budget_ledger WHERE intent_id = ?')
      .get(challenge.challengeId) as any;
    expect(row).toBeDefined();
    expect(row.state).toBe(LedgerEntryState.RESERVED);
  });

  it('network failure after wire dispatch MUST NOT auto-release quota', async () => {
    const { signer, policy, clock, db } = setupEnvironment();
    const challenge = createStandardChallenge();

    const mockFetch: typeof globalThis.fetch = async (input, init) => {
      const headers = new Headers(init?.headers);
      const sigHeader = headers.get(HEADER_PAYMENT_SIGNATURE);

      if (!sigHeader) {
        return new Response(JSON.stringify({ error: 'Payment Required' }), {
          status: 402,
          headers: {
            [HEADER_PAYMENT_REQUIRED]: encodeChallenge(challenge),
          },
        });
      }

      // Socket closed abruptly during replay
      throw new Error('ECONNRESET: socket hang up');
    };

    const client = new AgentPayClient({
      signer,
      policy,
      clock,
      baseFetch: mockFetch,
    });

    await expect(client.get('https://api.example.com/v1/market-data')).rejects.toThrow('ECONNRESET');

    // CRITICAL REDLINE: Reservation remains RESERVED
    const row = db
      .prepare('SELECT state FROM ap_budget_ledger WHERE intent_id = ?')
      .get(challenge.challengeId) as any;
    expect(row).toBeDefined();
    expect(row.state).toBe(LedgerEntryState.RESERVED);
  });

  it('pre-dispatch signing failure immediately releases quota in SQLite ledger', async () => {
    const { policy, clock, db } = setupEnvironment();
    const challenge = createStandardChallenge();

    class FailingSigner extends LocalSigner {
      override async signPaymentAuthorization(): Promise<any> {
        throw new Error('KMS HSM connection timeout');
      }
    }

    const failingSigner = new FailingSigner(TEST_PRIVATE_KEY);

    const mockFetch: typeof globalThis.fetch = async () => {
      return new Response(JSON.stringify({ error: 'Payment Required' }), {
        status: 402,
        headers: {
          [HEADER_PAYMENT_REQUIRED]: encodeChallenge(challenge),
        },
      });
    };

    const client = new AgentPayClient({
      signer: failingSigner,
      policy,
      clock,
      baseFetch: mockFetch,
    });

    await expect(client.get('https://api.example.com/v1/market-data')).rejects.toThrow('KMS HSM connection timeout');

    // PRE-DISPATCH: Reservation safely released back to quota
    const row = db
      .prepare('SELECT state FROM ap_budget_ledger WHERE intent_id = ?')
      .get(challenge.challengeId) as any;
    expect(row).toBeDefined();
    expect(row.state).toBe(LedgerEntryState.RELEASED);
  });

  it('passes through 402 when autoRetryOn402 is false', async () => {
    const { signer, policy, clock } = setupEnvironment();
    const challenge = createStandardChallenge();

    const mockFetch: typeof globalThis.fetch = async () => {
      return new Response(JSON.stringify({ error: 'Payment Required' }), {
        status: 402,
        headers: {
          [HEADER_PAYMENT_REQUIRED]: encodeChallenge(challenge),
        },
      });
    };

    const client = new AgentPayClient({
      signer,
      policy,
      clock,
      baseFetch: mockFetch,
      autoRetryOn402: false,
    });

    const res = await client.get('https://api.example.com/v1/market-data');
    expect(res.status).toBe(402);
  });

  it('passes through 402 when challenge is already expired', async () => {
    const { signer, policy } = setupEnvironment(undefined, 1729000100);
    // validBefore in past relative to clock (1729000100)
    const expiredChallenge = createStandardChallenge({
      challengeId: 'ch_expired',
      validAfter: 1728000000,
      validBefore: 1728000050,
    });

    const mockFetch: typeof globalThis.fetch = async () => {
      return new Response(JSON.stringify({ error: 'Payment Required' }), {
        status: 402,
        headers: {
          [HEADER_PAYMENT_REQUIRED]: encodeChallenge(expiredChallenge),
        },
      });
    };

    const client = new AgentPayClient({
      signer,
      policy,
      clock: () => 1729000100,
      baseFetch: mockFetch,
    });

    const res = await client.get('https://api.example.com/v1/market-data');
    expect(res.status).toBe(402);
  });

  describe('Credential Mode', () => {
    it('blocks unmetered credential request by default (fail-closed)', async () => {
      const { signer, policy, clock } = setupEnvironment();

      const client = new AgentPayClient({
        signer,
        policy,
        clock,
        unmeteredCredentialPolicy: 'block',
      });

      await expect(
        client.get('https://api.example.com/v1/data', {
          headers: { 'x-agent-key': 'apk_123456' },
        }),
      ).rejects.toThrow(PolicyViolationError);
    });

    it('blocks Bearer token matching credential prefix by default', async () => {
      const { signer, policy, clock } = setupEnvironment();

      const client = new AgentPayClient({
        signer,
        policy,
        clock,
        unmeteredCredentialPolicy: 'block',
      });

      await expect(
        client.get('https://api.example.com/v1/data', {
          headers: { Authorization: 'Bearer apk_secret_token' },
        }),
      ).rejects.toThrow(PolicyViolationError);
    });

    it('allows unmetered request when explicitly marked with opt-out header', async () => {
      const { signer, policy, clock } = setupEnvironment();

      const mockFetch: typeof globalThis.fetch = async () => {
        return new Response(JSON.stringify({ status: 'ok' }), { status: 200 });
      };

      const client = new AgentPayClient({
        signer,
        policy,
        clock,
        baseFetch: mockFetch,
        unmeteredCredentialPolicy: 'block',
      });

      const res = await client.get('https://api.example.com/v1/data', {
        headers: {
          'x-agent-key': 'apk_123456',
          [ALLOW_UNMETERED_HEADER]: 'true',
        },
      });
      expect(res.status).toBe(200);
    });

    it('allows unmetered request when policy mode is allow', async () => {
      const { signer, policy, clock } = setupEnvironment();

      const mockFetch: typeof globalThis.fetch = async () => {
        return new Response(JSON.stringify({ status: 'ok' }), { status: 200 });
      };

      const client = new AgentPayClient({
        signer,
        policy,
        clock,
        baseFetch: mockFetch,
        unmeteredCredentialPolicy: 'allow',
      });

      const res = await client.get('https://api.example.com/v1/data', {
        headers: { 'x-agent-key': 'apk_123456' },
      });
      expect(res.status).toBe(200);
    });

    it('meters credential request with estimateCost and settles with actualCost on 200', async () => {
      const { signer, policy, clock, db } = setupEnvironment();

      const mockFetch: typeof globalThis.fetch = async () => {
        return new Response(JSON.stringify({ data: 'ok' }), {
          status: 200,
          headers: { 'x-token-cost': '30000' },
        });
      };

      const client = new AgentPayClient({
        signer,
        policy,
        clock,
        baseFetch: mockFetch,
        unmeteredCredentialPolicy: 'meter',
        estimateCost: () => 50_000n,
        actualCost: (res) => {
          const cost = res.headers.get('x-token-cost');
          return cost ? BigInt(cost) : null;
        },
      });

      const res = await client.get('https://api.example.com/v1/data', {
        headers: { 'x-agent-key': 'apk_123456' },
      });
      expect(res.status).toBe(200);

      const rows = db.prepare("SELECT state, amount_base_units FROM ap_budget_ledger WHERE intent_id LIKE 'cred_%'").all() as any[];
      expect(rows).toHaveLength(1);
      expect(rows[0].state).toBe(LedgerEntryState.COMMITTED);
      expect(BigInt(rows[0].amount_base_units)).toBe(30_000n);
    });
  });

  describe('AgentPay Wallet Scheme (agentpay-wallet)', () => {
    it('successfully processes agentpay-wallet challenge with requestBinding', async () => {
      const { signer, policy, clock, db } = setupEnvironment({
        whitelistAssets: ['0x036cbd53842c5426634e7929541ec2318f3dcf7e', 'USD'],
        whitelistNetworks: ['eip155:84532', 'agentpay:internal-ledger'],
      });

      const url = 'https://api.example.com/v1/generate';
      const body = JSON.stringify({ prompt: 'Hello agent' });
      const bodyBytes = new TextEncoder().encode(body);
      const expectedBinding = computeRequestBinding('POST', url, bodyBytes);

      const walletChallenge = new PaymentChallenge({
        version: '2.0',
        challengeId: 'ch_wallet_01',
        network: 'agentpay:internal-ledger',
        asset: 'USD',
        amount: '25000',
        payee: 'agentpay:route:openai-gpt4',
        scheme: 'agentpay-wallet',
        validAfter: 1729000000,
        validBefore: 1729000300,
        resourceUri: '/v1/generate',
        extensions: {
          requestBinding: expectedBinding,
          routeId: 'route_openai_gpt4',
        },
      });

      const mockFetch: typeof globalThis.fetch = async (input, init) => {
        const headers = new Headers(init?.headers);
        const sig = headers.get(HEADER_PAYMENT_SIGNATURE);
        if (!sig) {
          return new Response(JSON.stringify({ error: 'Payment Required' }), {
            status: 402,
            headers: {
              [HEADER_PAYMENT_REQUIRED]: encodeChallenge(walletChallenge),
            },
          });
        }

        return new Response(JSON.stringify({ completion: 'World' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      };

      const client = new AgentPayClient({
        signer,
        policy,
        clock,
        baseFetch: mockFetch,
        raiseOnPolicyRejection: true,
      });

      const res = await client.post(url, { prompt: 'Hello agent' });
      expect(res.status).toBe(200);
      const data = await res.json();
      expect(data.completion).toBe('World');

      const row = db
        .prepare('SELECT state FROM ap_budget_ledger WHERE intent_id = ?')
        .get(walletChallenge.challengeId) as any;
      expect(row).toBeDefined();
      expect(row.state).toBe(LedgerEntryState.COMMITTED);
    });
  });

  describe('AgentPayClient convenience methods', () => {
    it('supports get, post, put, delete, and asFetch', async () => {
      const { signer, policy, clock } = setupEnvironment();
      const calledMethods: string[] = [];

      const mockFetch: typeof globalThis.fetch = async (input, init) => {
        calledMethods.push(init?.method ?? 'GET');
        return new Response(JSON.stringify({ ok: true }), { status: 200 });
      };

      const client = new AgentPayClient({
        signer,
        policy,
        clock,
        baseUrl: 'https://api.example.com',
        baseFetch: mockFetch,
      });

      await client.get('/v1/test');
      await client.post('/v1/test', { key: 'val' });
      await client.put('/v1/test', { key: 'updated' });
      await client.delete('/v1/test');

      const customFetch = client.asFetch();
      await customFetch('https://api.example.com/v1/fetch');

      let receivedQuery = '';
      const queryFetch: typeof globalThis.fetch = async (input) => {
        receivedQuery = new URL(input.toString()).search;
        return new Response(JSON.stringify({ ok: true }), { status: 200 });
      };
      const queryClient = new AgentPayClient({
        signer,
        policy,
        clock,
        baseFetch: queryFetch,
      });
      await queryClient.get('https://api.example.com/v1/search', {
        params: { q: 'agent', count: 10, active: true },
      });
      expect(receivedQuery).toBe('?q=agent&count=10&active=true');

      expect(calledMethods).toEqual(['GET', 'POST', 'PUT', 'DELETE', 'GET']);
    });
  });
});
