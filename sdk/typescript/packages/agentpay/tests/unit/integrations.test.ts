import { describe, expect, it } from 'vitest';
import { AgentPayClient } from '../../src/client/client.js';
import { LocalSigner } from '../../src/client/signer.js';
import { encodeChallenge, HEADER_PAYMENT_REQUIRED, HEADER_PAYMENT_SIGNATURE } from '../../src/core/codecs.js';
import { PaymentChallenge } from '../../src/core/models.js';
import {
  createPaidMcpFetch,
  createPaidMcpToolHandler,
  createPaidTool,
  paidVercelTool,
  ToolException,
} from '../../src/integrations/index.js';
import { createBuyerConnection, initBuyerDatabase } from '../../src/policy/db.js';
import { PolicyErrorCode, PolicyViolationError } from '../../src/policy/errors.js';
import { PolicyConfig } from '../../src/policy/models.js';
import { SQLitePolicyEngine } from '../../src/policy/sqlite-engine.js';

const TEST_PRIVATE_KEY = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';

function setupClient(maxPerTx = 100_000n) {
  const signer = new LocalSigner(TEST_PRIVATE_KEY);
  const db = createBuyerConnection(':memory:');
  initBuyerDatabase(db);

  const config = new PolicyConfig({
    id: 'pol_integ',
    name: 'Integrations Policy',
    walletAddress: signer.address,
    maxPerTxBaseUnits: maxPerTx,
    rollingBudget24hBaseUnits: 1_000_000n,
    whitelistPayees: ['0x70997970c51812dc3a010c7d01b50e0d17dc79c8'],
    whitelistAssets: ['0x036cbd53842c5426634e7929541ec2318f3dcf7e'],
    whitelistNetworks: ['eip155:84532'],
    whitelistDomains: ['api.example.com'],
  });

  const clock = () => 1729000100;
  const policy = new SQLitePolicyEngine(db, config, clock);

  const challenge = new PaymentChallenge({
    version: '2.0',
    challengeId: 'ch_integ_01',
    network: 'eip155:84532',
    asset: '0x036cbd53842c5426634e7929541ec2318f3dcf7e',
    amount: '50000',
    payee: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8',
    scheme: 'EIP-3009',
    validAfter: 1729000000,
    validBefore: 1729000300,
    facilitatorUrl: 'https://facilitator.example.com',
    resourceUri: '/v1/paid-op',
  });

  const mockFetch: typeof globalThis.fetch = async (input, init) => {
    const headers = new Headers(init?.headers);
    if (!headers.get(HEADER_PAYMENT_SIGNATURE)) {
      return new Response(JSON.stringify({ error: 'Payment Required' }), {
        status: 402,
        headers: {
          [HEADER_PAYMENT_REQUIRED]: encodeChallenge(challenge),
        },
      });
    }

    return new Response(JSON.stringify({ result: 'premium data computed' }), {
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

  return { client, policy, signer, challenge };
}

describe('LangChain Integration (createPaidTool)', () => {
  it('executes paid tool successfully with injected AgentPayClient', async () => {
    const { client } = setupClient();

    const paidTool = createPaidTool({
      name: 'fetch_premium_report',
      description: 'Fetches paid financial report',
      client,
      func: async (args: { symbol: string }, context) => {
        expect(context?.client).toBe(client);
        const res = await context!.client!.get('https://api.example.com/v1/paid-op');
        const json = await res.json();
        return `${args.symbol}: ${json.result}`;
      },
    });

    const output = await paidTool.invoke({ symbol: 'BTC' });
    expect(output).toBe('BTC: premium data computed');
  });

  it('converts PolicyViolationError into ToolException when handlePolicyViolation is true', async () => {
    // Single tx cap is 10,000, challenge is 50,000 -> policy rejection
    const { client } = setupClient(10_000n);

    const paidTool = createPaidTool({
      name: 'expensive_tool',
      description: 'Tool that exceeds budget',
      client,
      handlePolicyViolation: true,
      func: async () => {
        await client.get('https://api.example.com/v1/paid-op');
        return 'success';
      },
    });

    await expect(paidTool.invoke({})).rejects.toThrow(ToolException);
    await expect(paidTool.invoke({})).rejects.toThrow(/AgentPay Firewall Rejected: \[SINGLE_TX_CAP_EXCEEDED\]/);
  });

  it('rethrows PolicyViolationError when handlePolicyViolation is false', async () => {
    const { client } = setupClient(10_000n);

    const paidTool = createPaidTool({
      name: 'expensive_tool_raw',
      description: 'Tool that throws raw error',
      client,
      handlePolicyViolation: false,
      func: async () => {
        await client.get('https://api.example.com/v1/paid-op');
        return 'success';
      },
    });

    await expect(paidTool.invoke({})).rejects.toThrow(PolicyViolationError);
  });
});

describe('Vercel AI SDK Integration (paidVercelTool)', () => {
  it('executes vercel tool successfully', async () => {
    const { client } = setupClient();

    const tool = paidVercelTool({
      description: 'Paid query tool',
      parameters: { type: 'object' },
      client,
      execute: async (args: { query: string }, context) => {
        expect(context?.client).toBe(client);
        const res = await context!.client!.get('https://api.example.com/v1/paid-op');
        const json = await res.json();
        return { data: json.result, query: args.query };
      },
    });

    const result = await tool.execute({ query: 'hello' });
    expect(result).toEqual({ data: 'premium data computed', query: 'hello' });
  });

  it('returns structured error object on policy rejection when handlePolicyViolation is true', async () => {
    const { client } = setupClient(10_000n);

    const tool = paidVercelTool({
      description: 'Paid query tool with budget check',
      parameters: { type: 'object' },
      client,
      handlePolicyViolation: true,
      execute: async () => {
        await client.get('https://api.example.com/v1/paid-op');
        return 'ok';
      },
    });

    const result = (await tool.execute({})) as any;
    expect(result.error).toBe(true);
    expect(result.code).toBe(PolicyErrorCode.SINGLE_TX_CAP_EXCEEDED);
    expect(result.message).toContain('AgentPay Firewall Rejected: [SINGLE_TX_CAP_EXCEEDED]');
  });

  it('rethrows PolicyViolationError when handlePolicyViolation is false', async () => {
    const { client } = setupClient(10_000n);

    const tool = paidVercelTool({
      description: 'Paid query tool',
      parameters: { type: 'object' },
      client,
      handlePolicyViolation: false,
      execute: async () => {
        await client.get('https://api.example.com/v1/paid-op');
        return 'ok';
      },
    });

    await expect(tool.execute({})).rejects.toThrow(PolicyViolationError);
  });
});

describe('Model Context Protocol (MCP) Integration', () => {
  it('creates MCP tool handler that returns standard content', async () => {
    const { client } = setupClient();

    const handler = createPaidMcpToolHandler(client, async (args: { id: string }, context) => {
      const res = await context!.client.get('https://api.example.com/v1/paid-op');
      const json = await res.json();
      return `Item ${args.id}: ${json.result}`;
    });

    const result = await handler({ id: '42' });
    expect(result.isError).toBeUndefined();
    expect(result.content).toHaveLength(1);
    expect(result.content[0]!.type).toBe('text');
    expect(result.content[0]!.text).toBe('Item 42: premium data computed');
  });

  it('handles policy violation in MCP tool handler by returning isError: true', async () => {
    const { client } = setupClient(10_000n);

    const handler = createPaidMcpToolHandler(client, async () => {
      await client.get('https://api.example.com/v1/paid-op');
      return 'ok';
    });

    const result = await handler({});
    expect(result.isError).toBe(true);
    expect(result.content).toHaveLength(1);
    expect(result.content[0]!.text).toContain('AgentPay Firewall Rejected: [SINGLE_TX_CAP_EXCEEDED]');
  });

  it('createPaidMcpFetch returns drop-in fetch function from client', async () => {
    const { client } = setupClient();

    const mcpFetch = createPaidMcpFetch(client);
    const res = await mcpFetch('https://api.example.com/v1/paid-op');
    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.result).toBe('premium data computed');
  });
});
