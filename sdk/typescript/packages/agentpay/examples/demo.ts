import { AgentPayClient, LocalSigner } from '../dist/index.js';
import { PolicyConfig, SQLitePolicyEngine, WhitelistMode } from '../dist/policy/index.js';

async function main() {
  console.log('>>> [1/3] 初始化 AgentPay 客户端与策略防火墙...');

  // 从环境变量 AGENT_WALLET_KEY 获取买方 EVM 私钥（支持 0x 前缀或纯 64 位 Hex）
  const privateKey = process.env.AGENT_WALLET_KEY ?? '0xYOUR_EVM_PRIVATE_KEY';
  if (privateKey === '0xYOUR_EVM_PRIVATE_KEY') {
    console.error('❌ 请先配置环境变量 AGENT_WALLET_KEY，例如:');
    console.error('   AGENT_WALLET_KEY="0x..." npm run demo\n');
    process.exit(1);
  }
  const signer = new LocalSigner(privateKey);

  const policy = SQLitePolicyEngine.createInMemory(
    new PolicyConfig({
      id: 'agent_firewall',
      name: 'Agent Firewall',
      walletAddress: signer.address,
      whitelistMode: WhitelistMode.SIMPLE,
      maxPerTxBaseUnits: 2_500_000n, // 单次最高预算 2.50 USDC
      rollingBudget24hBaseUnits: 5_000_000n, // 24h 滚动预算 5.00 USDC
    })
  );

  const client = new AgentPayClient({
    signer,
    policy,
  });

  console.log('>>> [2/3] 发送请求: https://gate.openagentpay.dev/v1/free/defi/dex-volume ...');
  const start = performance.now();

  try {
    const response = await client.get(
      'https://gate.openagentpay.dev/v1/free/defi/dex-volume',
      {
        params: {
          chain: 'ethereum',
          excludeTotalDataChart: true,
          excludeTotalDataChartBreakdown: true,
        },
      }
    );

    const elapsed = (performance.now() - start) / 1000;
    console.log(`>>> [3/3] 请求完成! 耗时: ${elapsed.toFixed(3)}s`);
    console.log('HTTP Status:', response.status);

    const json = await response.json();
    console.log('HTTP JSON Response:', json);
  } catch (err) {
    console.error('Request failed:', err);
  }
}

main().catch(console.error);
