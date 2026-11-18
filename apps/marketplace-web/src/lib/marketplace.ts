import { SEED_MARKETPLACE_ROUTES } from '@/lib/seed-data';
import type { MarketplaceFilter, MarketplaceRouteItem } from '@/types/marketplace';

const INTERNAL_GATEWAY_URL =
  process.env.GATEWAY_INTERNAL_URL ||
  process.env.NEXT_PUBLIC_GATEWAY_API_URL ||
  'http://127.0.0.1:8080';

export const PUBLIC_GATEWAY_URL =
  process.env.NEXT_PUBLIC_GATEWAY_URL || 'https://gate.aipmedia.cn';

export const PUBLIC_SITE_URL =
  process.env.NEXT_PUBLIC_SITE_URL || 'https://aipmedia.cn';

export const PUBLIC_CONSOLE_URL =
  process.env.NEXT_PUBLIC_CONSOLE_URL || 'https://console.aipmedia.cn';

export const PUBLIC_PLAYGROUND_URL =
  process.env.NEXT_PUBLIC_PLAYGROUND_URL || `${PUBLIC_CONSOLE_URL}/playground`;

export const PUBLIC_DOCS_URL =
  process.env.NEXT_PUBLIC_DOCS_URL || '/docs/';

export const PUBLIC_GITHUB_URL =
  process.env.NEXT_PUBLIC_GITHUB_URL || 'https://github.com/agentfinops/AgentFinOps';

export const PUBLIC_TWITTER_URL =
  process.env.NEXT_PUBLIC_TWITTER_URL || 'https://x.com/OpenAgentPay';

export const PUBLIC_DISCORD_URL =
  process.env.NEXT_PUBLIC_DISCORD_URL || 'https://discord.gg/2SYyfx4UuQ';

/**
 * Formats 6-decimal USDC base units into a clean human-readable string (e.g. "0.001" or "0.05").
 */
export function formatUsdcPrice(baseUnits: number): string {
  const val = baseUnits / 1_000_000;
  if (val === 0) return '0.00';
  if (val < 0.001) return val.toFixed(6).replace(/0+$/, '').replace(/\.$/, '');
  return val.toFixed(4).replace(/0+$/, '').replace(/\.$/, '');
}

/**
 * Maps CAIP-2 network identifiers to human-readable network labels.
 */
export function formatNetworkLabel(network?: string, fallback?: string): string {
  if (fallback) return fallback;
  const clean = (network || '').toLowerCase();
  if (clean.includes('8453')) return 'Base USDC (x402)';
  if (clean.includes('42161')) return 'Arbitrum USDC (x402)';
  if (clean.includes('solana')) return 'Solana USDC (x402)';
  return network || 'Base / Arbitrum USDC';
}

/**
 * Formats CNY/Credits price into a clean human-readable string with 2 decimal places (分, e.g. "0.05" or "10.00").
 */
export function formatCreditsPrice(baseUnits: number): string {
  const val = baseUnits / 1_000_000;
  return val.toFixed(2);
}

/**
 * Checks whether the given route is an LLM / Generative AI route.
 */
export function isLlmRoute(route: MarketplaceRouteItem): boolean {
  if (!route) return false;
  return (
    route.route_type === 'llm' ||
    route.pricing_type === 'per_token' ||
    (route.category === 'AI & LLM' && (Boolean(route.model_name) || route.path_pattern.includes('/chat') || route.path_pattern.includes('/llm')))
  );
}

/**
 * Resolves the primary LLM model identifier for chat/completions.
 */
export function getLlmModelName(route: MarketplaceRouteItem): string {
  if (route.model_name && route.model_name.trim()) {
    return route.model_name.trim();
  }
  // Fallbacks inferred from slug or title
  const slug = (route.slug || '').toLowerCase();
  if (slug.includes('deepseek-r1') || slug.includes('reasoner')) return 'deepseek-reasoner';
  if (slug.includes('deepseek')) return 'deepseek-chat';
  if (slug.includes('qwen')) return 'qwen-plus';
  if (slug.includes('gpt-4o')) return 'gpt-4o';
  return route.slug || 'llm-chat';
}

/**
 * Formats token pricing base units into human-readable Fen per 1,000 tokens (分 / 1k Tokens).
 * Guaranteed strictly in "分" (no "微" units).
 * 1 CNY = 100 分 = 1,000,000 base_units => 10,000 base_units = 1 分.
 */
export function formatTokenPriceFen(baseUnits?: number): string {
  if (baseUnits === undefined || baseUnits === null) return '0.00';
  const fen = baseUnits / 10_000;
  if (fen > 0 && fen < 0.01) {
    return fen.toFixed(4);
  }
  return fen.toFixed(2);
}

/**
 * Maps cluster identifiers to human-readable cluster labels.
 */
export function formatClusterLabel(cluster?: string, fallback?: string, locale: string = 'en'): string {
  if (fallback) return fallback;
  if (!cluster) {
    return locale === 'en'
      ? 'Enterprise High-Availability Cluster (domestic-v1)'
      : '企业内部高可用集群 (domestic-v1)';
  }
  if (cluster === 'domestic-v1' || cluster.toLowerCase().includes('domestic')) {
    return locale === 'en'
      ? 'East China Dedicated Cluster (domestic-v1)'
      : '华东专线集群 (domestic-v1)';
  }
  return cluster;
}

/**
 * Generates the standardized Answer-First TL;DR summary for GEO optimization.
 */
export function buildAnswerFirstTldr(
  route: MarketplaceRouteItem,
  locale: string = 'en'
): string {
  const clusterLabel = formatClusterLabel(route.cluster, undefined, locale);

  if (isLlmRoute(route)) {
    const model = getLlmModelName(route);
    const promptFen = formatTokenPriceFen(route.prompt_token_price_base_units ?? 1000);
    const completionFen = formatTokenPriceFen(route.completion_token_price_base_units ?? 2000);

    if (locale === 'en') {
      const baseSummary =
        route.summary ||
        route.description_md.split('\n').find((line) => line.trim() && !line.trim().startsWith('#')) ||
        `${route.title} provides high-performance LLM inference and streaming dialog services for AI Agents.`;
      return `${baseSummary} Uses OpenAI-compatible API protocol (/v1/chat/completions, model: ${model}) with millisecond per-1k-token streaming billing (Prompt: ${promptFen} cents/1k Tokens, Completion: ${completionFen} cents/1k Tokens, ${clusterLabel}). Seamlessly works with official OpenAI SDK, LangChain, and LlamaIndex, backed by Policy Firewall budget protection.`;
    }

    const baseSummary =
      route.summary ||
      route.description_md.split('\n').find((line) => line.trim() && !line.trim().startsWith('#')) ||
      `${route.title} 提供面向 AI Agent 的高质量大模型推理与流式对话服务。`;

    return `${baseSummary} 采用 OpenAI 标准兼容接口协议（/v1/chat/completions，模型名：${model}），支持毫秒级千 Token 流式按需计费（输入：${promptFen} 分/1k Tokens，输出：${completionFen} 分/1k Tokens，${clusterLabel}）。无缝兼容主流官方 OpenAI SDK 与 LangChain、LlamaIndex 等智能体框架，配合策略防火墙实现自动断流预算保护与即用即结。`;
  }

  const priceStr = formatCreditsPrice(route.price_base_units);

  if (locale === 'en') {
    const defaultSummaryEn = `${route.title} provides standardized API services for AI Agents.`;
    const baseSummary =
      route.summary ||
      route.description_md.split('\n').find((line) => line.trim() && !line.trim().startsWith('#')) ||
      defaultSummaryEn;
    return `${baseSummary} Supports enterprise quota settlement and billing (${clusterLabel}) at ${priceStr} Credits/req, enabling zero-friction AgentPay SDK integration with real-time budget firewall enforcement.`;
  }

  const defaultSummaryZh = `${route.title} 提供面向 AI Agent 的标准化接口服务。`;
  const baseSummary =
    route.summary ||
    route.description_md.split('\n').find((line) => line.trim() && !line.trim().startsWith('#')) ||
    defaultSummaryZh;

  return `${baseSummary} 支持企业级配额核销与计费（${clusterLabel}），单次调用价格为 ${priceStr} Credits/req，支持通过 AgentFinOps SDK 免繁琐开户流程，配合策略引擎实现调用即结算。`;
}

/**
 * Generates official OpenAI Python SDK snippet for LLM routes.
 */
export function buildOpenAiPythonSdkSnippet(
  route: MarketplaceRouteItem,
  locale: string = 'en',
  activeApiKey?: string
): string {
  const effectiveKey = activeApiKey || 'apk_live_your_api_key';
  const modelName = getLlmModelName(route);

  if (locale === 'en') {
    return `import os
from openai import OpenAI

# 1. Connect to gateway, seamlessly compatible with OpenAI official Python SDK (pip install openai)
client = OpenAI(
    base_url="${PUBLIC_GATEWAY_URL}/v1",  # OpenAI-compatible gateway endpoint
    api_key=os.getenv("AGENT_API_KEY", "${effectiveKey}"),  # Platform API Key or claim a trial key
)

# 2. Stream completions with millisecond Token-level billing
response = client.chat.completions.create(
    model="${modelName}",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello! Please briefly introduce your core capabilities."},
    ],
    temperature=0.7,
    stream=True,  # Enable SSE streaming with real-time settlement
)

# 3. Consume streaming token chunks (SSE Chunks)
print(">>> AI Assistant:")
for chunk in response:
    delta = chunk.choices[0].delta.content or ""
    print(delta, end="", flush=True)
print()`;
  }

  return `import os
from openai import OpenAI

# 1. 统一接入 AgentFinOps 网关，无缝兼容 OpenAI 官方 Python SDK (pip install openai)
client = OpenAI(
    base_url="${PUBLIC_GATEWAY_URL}/v1",  # 网关 OpenAI 兼容入口
    api_key=os.getenv("AGENT_API_KEY", "${effectiveKey}"),  # 平台 API Key 或一键申领测试 Key
)

# 2. 发起流式推理，按实际产生 Token 毫秒级计费
response = client.chat.completions.create(
    model="${modelName}",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "你好，请简要介绍一下你具备的核心能力。"},
    ],
    temperature=0.7,
    stream=True,  # 启用 SSE 流式输出，实时计费核销
)

# 3. 消费流式响应块 (Token SSE Chunks)
print(">>> AI 回复:")
for chunk in response:
    delta = chunk.choices[0].delta.content or ""
    print(delta, end="", flush=True)
print()`;
}

/**
 * Generates official OpenAI Node.js / TypeScript SDK snippet for LLM routes.
 */
export function buildOpenAiNodeSdkSnippet(
  route: MarketplaceRouteItem,
  locale: string = 'en',
  activeApiKey?: string
): string {
  const effectiveKey = activeApiKey || 'apk_live_your_api_key';
  const modelName = getLlmModelName(route);

  if (locale === 'en') {
    return `import OpenAI from 'openai';

// 1. Initialize OpenAI client (npm install openai) pointing to gateway
const client = new OpenAI({
  baseURL: '${PUBLIC_GATEWAY_URL}/v1',
  apiKey: process.env.AGENT_API_KEY ?? '${effectiveKey}',
});

async function main() {
  // 2. Stream chat completions with real-time token settlement
  const stream = await client.chat.completions.create({
    model: '${modelName}',
    messages: [
      { role: 'system', content: 'You are a helpful assistant.' },
      { role: 'user', content: 'Hello! Please briefly introduce your core capabilities.' },
    ],
    temperature: 0.7,
    stream: true,
  });

  // 3. Output streaming tokens in real time
  process.stdout.write('>>> AI Assistant: ');
  for await (const chunk of stream) {
    const text = chunk.choices[0]?.delta?.content || '';
    process.stdout.write(text);
  }
  process.stdout.write('\\n');
}

main().catch(console.error);`;
  }

  return `import OpenAI from 'openai';

// 1. 初始化 OpenAI 官方客户端 (npm install openai)，指向 AgentFinOps 网关
const client = new OpenAI({
  baseURL: '${PUBLIC_GATEWAY_URL}/v1',
  apiKey: process.env.AGENT_API_KEY ?? '${effectiveKey}',
});

async function main() {
  // 2. 发起流式对话，网关实时按千 Token 核销扣款
  const stream = await client.chat.completions.create({
    model: '${modelName}',
    messages: [
      { role: 'system', content: 'You are a helpful assistant.' },
      { role: 'user', content: '你好，请简要介绍一下你具备的核心能力。' },
    ],
    temperature: 0.7,
    stream: true,
  });

  // 3. 实时输出流式 Token
  process.stdout.write('>>> AI 回复: ');
  for await (const chunk of stream) {
    const text = chunk.choices[0]?.delta?.content || '';
    process.stdout.write(text);
  }
  process.stdout.write('\\n');
}

main().catch(console.error);`;
}

/**
 * Generates cURL SSE streaming request for LLM routes.
 */
export function buildCurlLlmSnippet(
  route: MarketplaceRouteItem,
  locale: string = 'en',
  activeApiKey?: string
): string {
  const effectiveKey = activeApiKey || 'apk_live_your_api_key';
  const modelName = getLlmModelName(route);

  if (locale === 'en') {
    return `# 1. Terminal SSE streaming test (SSE data chunks)
curl -N -X POST "${PUBLIC_GATEWAY_URL}/v1/chat/completions" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${effectiveKey}" \\
  -d '{
    "model": "${modelName}",
    "messages": [
      {"role": "user", "content": "Hello! Please give a brief self-introduction."}
    ],
    "stream": true
  }'`;
  }

  return `# 1. 终端流式直连测试 (SSE data chunks)
curl -N -X POST "${PUBLIC_GATEWAY_URL}/v1/chat/completions" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${effectiveKey}" \\
  -d '{
    "model": "${modelName}",
    "messages": [
      {"role": "user", "content": "你好，请做个简短的自我介绍。"}
    ],
    "stream": true
  }'`;
}

/**
 * Generates the simplified Python SDK code snippet with built-in brand attribution.
 */
export function buildPythonSdkSnippet(
  route: MarketplaceRouteItem,
  locale: string = 'en',
  activeApiKey?: string
): string {
  const effectiveKey = activeApiKey || 'apk_live_xxx';
  const suggestedBudgetUnits = Math.max(
    250_000,
    (route.price_base_units || 50_000) * 5
  );
  const sampleParams =
    route.sample_params && Object.keys(route.sample_params).length > 0
      ? route.sample_params
      : (route.parameters && typeof route.parameters === 'object' && !Array.isArray(route.parameters) && Object.keys(route.parameters).length > 0)
      ? (route.parameters as Record<string, unknown>)
      : { city: locale === 'en' ? 'San Francisco' : '北京' };

  const paramsJson = JSON.stringify(sampleParams, null, 4);

  const primaryMethod = (route.methods?.[0] || 'GET').toUpperCase();
  const isPost =
    primaryMethod === 'POST' ||
    (!route.methods?.includes('GET') && route.methods?.includes('POST'));
  const httpMethod = isPost ? 'post' : 'get';
  const paramArg = isPost ? 'json' : 'params';
  const endpointUrl = `${PUBLIC_GATEWAY_URL}${route.path_pattern}`;

  const suggestedDailyUnits = Math.max(5_000_000, suggestedBudgetUnits * 20);
  const policyId = (route.slug || 'weather_agent').replace(/-/g, '_').slice(0, 32);

  if (locale === 'en') {
    return `import os
# Import AgentPayClient directly from top-level agentpay_core
from agentpay_core import AgentPayClient, PolicyConfig, SQLitePolicyEngine

# 1. Policy Firewall: configure per-call budget and domain whitelist
policy = SQLitePolicyEngine.create_in_memory(
    PolicyConfig(
        id="${policyId}",
        max_per_tx_base_units=${suggestedBudgetUnits},  # ${(suggestedBudgetUnits / 1_000_000).toFixed(2)} Credits
        rolling_budget_24h_base_units=${suggestedDailyUnits},  # ${(suggestedDailyUnits / 1_000_000).toFixed(2)} Credits
        whitelist_domains={"gate.aipmedia.cn"},
    )
)

# 2. Pass api_key directly; automatic virtual wallet & 402 handshake
client = AgentPayClient(
    api_key=os.getenv("AGENT_API_KEY", "${effectiveKey}"),
    policy=policy,
)

# 3. Invoke endpoint normally
response = client.${httpMethod}(
    "${endpointUrl}",
    ${paramArg}=${paramsJson}
)
print("Response:", response.json())`;
  }

  return `import os
# 从顶层 agentpay_core 直接导入 AgentPayClient
from agentpay_core import AgentPayClient, PolicyConfig, SQLitePolicyEngine

# 1. 策略防火墙：仅配置预算和白名单域名
policy = SQLitePolicyEngine.create_in_memory(
    PolicyConfig(
        id="${policyId}",
        max_per_tx_base_units=${suggestedBudgetUnits},  # ${(suggestedBudgetUnits / 1_000_000).toFixed(2)} Credits
        rolling_budget_24h_base_units=${suggestedDailyUnits},  # ${(suggestedDailyUnits / 1_000_000).toFixed(2)} Credits
        whitelist_domains={"gate.aipmedia.cn"},
    )
)

# 2. 客户端直接传入 api_key，自动开启虚拟钱包模式与 402 自动握手
client = AgentPayClient(
    api_key=os.getenv("AGENT_API_KEY", "${effectiveKey}"),
    policy=policy,
)

# 3. 正常发起调用
response = client.${httpMethod}(
    "${endpointUrl}",
    ${paramArg}=${paramsJson}
)
print("Response:", response.json())`;
}

/**
 * Generates the simplified TypeScript SDK code snippet with built-in brand attribution.
 */
export function buildTypescriptSdkSnippet(
  route: MarketplaceRouteItem,
  locale: string = 'en',
  activeApiKey?: string
): string {
  const effectiveKey = activeApiKey || 'apk_live_xxx';
  const suggestedBudgetUnits = Math.max(
    250_000,
    (route.price_base_units || 50_000) * 5
  );
  const sampleParams =
    route.sample_params && Object.keys(route.sample_params).length > 0
      ? route.sample_params
      : (route.parameters && typeof route.parameters === 'object' && !Array.isArray(route.parameters) && Object.keys(route.parameters).length > 0)
      ? (route.parameters as Record<string, unknown>)
      : { timeframe: '24h' };

  const paramsJson = JSON.stringify(sampleParams, null, 2);
  const indentedParamsJson = paramsJson
    .split('\n')
    .map((line, i) => (i === 0 ? line : `  ${line}`))
    .join('\n');

  const primaryMethod = (route.methods?.[0] || 'GET').toUpperCase();
  const isPost =
    primaryMethod === 'POST' ||
    (!route.methods?.includes('GET') && route.methods?.includes('POST'));
  const endpointUrl = `${PUBLIC_GATEWAY_URL}${route.path_pattern}`;

  const suggestedDailyUnits = Math.max(5_000_000, suggestedBudgetUnits * 20);
  const policyId = (route.slug || 'weather_agent').replace(/-/g, '_').slice(0, 32);

  const requestCall = isPost
    ? `const response = await client.post(\n  '${endpointUrl}',\n  ${indentedParamsJson}\n);`
    : `const response = await client.get('${endpointUrl}', {\n  params: ${indentedParamsJson.replace(/\n/g, '\n  ')},\n});`;

  if (locale === 'en') {
    return `import { AgentPayClient, PolicyConfig, SQLitePolicyEngine } from 'agentpay-core-sdk';

// 1. Policy Firewall configuration
const policy = SQLitePolicyEngine.createInMemory(
  new PolicyConfig({
    id: '${policyId}',
    maxPerTxBaseUnits: ${suggestedBudgetUnits}n,
    rollingBudget24hBaseUnits: ${suggestedDailyUnits}n,
    whitelistDomains: ['gate.aipmedia.cn'],
  })
);

// 2. Pass apiKey directly
const client = new AgentPayClient({
  apiKey: process.env.AGENT_API_KEY ?? '${effectiveKey}',
  policy,
});

// 3. Invoke endpoint normally
${requestCall}
console.log('Response:', await response.json());`;
  }

  return `import { AgentPayClient, PolicyConfig, SQLitePolicyEngine } from 'agentpay-core-sdk';

// 1. 策略防火墙配置
const policy = SQLitePolicyEngine.createInMemory(
  new PolicyConfig({
    id: '${policyId}',
    maxPerTxBaseUnits: ${suggestedBudgetUnits}n,
    rollingBudget24hBaseUnits: ${suggestedDailyUnits}n,
    whitelistDomains: ['gate.aipmedia.cn'],
  })
);

// 2. 客户端直接传入 apiKey
const client = new AgentPayClient({
  apiKey: process.env.AGENT_API_KEY ?? '${effectiveKey}',
  policy,
});

// 3. 正常发起调用
${requestCall}
console.log('Response:', await response.json());`;
}

/**
 * Generates ready-to-use MCP (Model Context Protocol) configuration snippet for desktop clients & SDKs.
 */
export function buildMcpConfigSnippet(
  route: MarketplaceRouteItem,
  platform: 'workbuddy' | 'claude_cursor' | 'sdk' = 'workbuddy',
  activeApiKey?: string,
  locale: string = 'en'
): string {
  const effectiveKey = activeApiKey || 'apk_live_your_api_key';
  const toolName = (route.slug || route.route_id || 'mcp_tool').replace(/[^a-zA-Z0-9_-]/g, '_');
  const sseUrl = `${PUBLIC_GATEWAY_URL}/v1/mcp/sse?route_id=${route.route_id || route.slug}`;

  if (platform === 'workbuddy') {
    const comment =
      locale === 'en'
        ? '// Copy to ~/.workbuddy/mcp.json (WorkBuddy official configuration)'
        : '// 复制至 ~/.workbuddy/mcp.json (腾讯 WorkBuddy 官方配置)';
    return `${comment}
{
  "mcpServers": {
    "${toolName}": {
      "url": "${sseUrl}",
      "headers": {
        "Authorization": "Bearer ${effectiveKey}"
      }
    }
  }
}`;
  }

  if (platform === 'claude_cursor') {
    const comment =
      locale === 'en'
        ? '// Copy to claude_desktop_config.json or Cursor MCP settings'
        : '// 复制至 claude_desktop_config.json 或 Cursor MCP 设置';
    return `${comment}
{
  "mcpServers": {
    "${toolName}": {
      "url": "${sseUrl}",
      "headers": {
        "Authorization": "Bearer ${effectiveKey}"
      }
    }
  }
}`;
  }

  // SDK Interceptor snippet with real gateway SSE transport and clean core boundaries
  const amountCny = (route.price_base_units || 50000) / 1000000.0;
  if (locale === 'en') {
    return `import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { SSEClientTransport } from '@modelcontextprotocol/sdk/client/sse.js';
import { McpToolInterceptor, InMemoryToolQuotaGuard } from 'agentpay-core-sdk';

// 1. Initialize MCP client with gateway address and API Key
const gatewaySseUrl = new URL('${sseUrl}');
const transport = new SSEClientTransport(gatewaySseUrl, {
  eventSourceInit: {
    headers: {
      Authorization: \`Bearer \${process.env.AGENT_API_KEY ?? '${effectiveKey}'}\`,
    },
  },
  requestInit: {
    headers: {
      Authorization: \`Bearer \${process.env.AGENT_API_KEY ?? '${effectiveKey}'}\`,
    },
  },
});
const mcpClient = new Client({ name: 'my-agent', version: '1.0.0' }, { capabilities: {} });
await mcpClient.connect(transport);

// 2. Initialize tool quota guard (Max ¥50.00/call, daily budget ¥500.00)
const guard = new InMemoryToolQuotaGuard({
  maxPerCallCny: 50.0,
  rollingBudgetCny: 500.0,
});

// 3. Mount MCP client interceptor (two-stage reservation & hung-prevention)
const interceptor = new McpToolInterceptor({
  guard,
  defaultCostCny: ${amountCny.toFixed(2)},
});

// 4. Safely invoke gateway MCP tool (over-budget automatically returns guidance)
const result = await interceptor.callTool(
  {
    toolName: '${toolName}',
    arguments: ${JSON.stringify(route.sample_params || { query: 'test' })},
    agentId: 'bot_worker_1',
  },
  async (name, args) => {
    // Executed via MCP protocol on gateway
    return await mcpClient.callTool({ name, arguments: args });
  }
);
console.log('Result:', result);`;
  }

  return `import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { SSEClientTransport } from '@modelcontextprotocol/sdk/client/sse.js';
import { McpToolInterceptor, InMemoryToolQuotaGuard } from 'agentpay-core-sdk';

// 1. 初始化标准 MCP 客户端，配置网关地址与 API Key
const gatewaySseUrl = new URL('${sseUrl}');
const transport = new SSEClientTransport(gatewaySseUrl, {
  eventSourceInit: {
    headers: {
      Authorization: \`Bearer \${process.env.AGENT_API_KEY ?? '${effectiveKey}'}\`,
    },
  },
  requestInit: {
    headers: {
      Authorization: \`Bearer \${process.env.AGENT_API_KEY ?? '${effectiveKey}'}\`,
    },
  },
});
const mcpClient = new Client({ name: 'my-agent', version: '1.0.0' }, { capabilities: {} });
await mcpClient.connect(transport);

// 2. 初始化核心通用配额守卫 (单笔上限 ¥50.00，日预算 ¥500.00)
const guard = new InMemoryToolQuotaGuard({
  maxPerCallCny: 50.0,
  rollingBudgetCny: 500.0,
});

// 3. 挂载 MCP 客户端拦截器 (两阶段预扣与自动防悬挂)
const interceptor = new McpToolInterceptor({
  guard,
  defaultCostCny: ${amountCny.toFixed(2)},
});

// 4. 安全调用网关 MCP 工具 (超额自动向大模型返回结构化引导，防死循环)
const result = await interceptor.callTool(
  {
    toolName: '${toolName}',
    arguments: ${JSON.stringify(route.sample_params || { query: 'test' })},
    agentId: 'bot_worker_1',
  },
  async (name, args) => {
    // 实际通过 MCP 协议发送至 gate.aipmedia.cn 网关执行
    return await mcpClient.callTool({ name, arguments: args });
  }
);
console.log('Result:', result);`;
}

/**
 * Generates pure Markdown documentation for a single route (used by /llms.txt and AI Bot content negotiation).
 */
export function buildRouteMarkdownDoc(route: MarketplaceRouteItem, locale: string = 'en'): string {
  const isLlm = isLlmRoute(route);
  const promptFen = formatTokenPriceFen(route.prompt_token_price_base_units ?? 1000);
  const completionFen = formatTokenPriceFen(route.completion_token_price_base_units ?? 2000);
  const priceDisplay = isLlm
    ? (locale === 'en'
        ? `Prompt: ${promptFen} cents/1k Tokens | Completion: ${completionFen} cents/1k Tokens (Streaming)`
        : `输入: ${promptFen} 分/1k Tokens | 输出: ${completionFen} 分/1k Tokens (流式计费)`)
    : `${formatCreditsPrice(route.price_base_units)} Credits / request`;

  const clusterLabel = formatClusterLabel(route.cluster, undefined, locale);
  const tldr = buildAnswerFirstTldr(route, locale);
  const sdkCode = isLlm ? buildOpenAiPythonSdkSnippet(route, locale) : buildPythonSdkSnippet(route, locale);
  const tsSdkCode = isLlm ? buildOpenAiNodeSdkSnippet(route, locale) : buildTypescriptSdkSnippet(route, locale);

  const effectiveSampleParams =
    route.sample_params && Object.keys(route.sample_params).length > 0
      ? route.sample_params
      : (route.parameters && typeof route.parameters === 'object' && !Array.isArray(route.parameters) && Object.keys(route.parameters).length > 0)
      ? (route.parameters as Record<string, unknown>)
      : null;

  let effectiveParams = Array.isArray(route.parameters) ? route.parameters : [];
  if (effectiveParams.length === 0 && effectiveSampleParams) {
    const flatten = (obj: Record<string, unknown>, prefix = ''): typeof effectiveParams => {
      const res: typeof effectiveParams = [];
      for (const [k, v] of Object.entries(obj)) {
        const fullKey = prefix ? `${prefix}.${k}` : k;
        if (v !== null && typeof v === 'object' && !Array.isArray(v)) {
          res.push({ name: fullKey, type: 'object', required: true, description: `Nested object ${fullKey}` });
          res.push(...flatten(v as Record<string, unknown>, fullKey));
        } else {
          res.push({
            name: fullKey,
            type: Array.isArray(v) ? 'array' : typeof v,
            required: true,
            description: `Parameter ${fullKey}`,
            example: typeof v === 'string' ? v : JSON.stringify(v)
          });
        }
      }
      return res;
    };
    effectiveParams = flatten(effectiveSampleParams);
  }

  const paramRows =
    effectiveParams.length > 0
      ? effectiveParams
          .map(
            (p) =>
              `| \`${p.name}\` | \`${p.type}\` | ${p.required ? 'Yes' : 'No'} | ${p.description || `Parameter ${p.name}`} |`
          )
          .join('\n')
      : `| \`params\` | \`object\` | No | Standard JSON query or body parameters forwarded to \`${route.path_pattern}\` |`;

  const sampleParamsBlock = effectiveSampleParams
    ? `\n### Request Payload Sample (application/json)\n\n\`\`\`json\n${JSON.stringify(effectiveSampleParams, null, 2)}\n\`\`\`\n`
    : '';

  return `# ${route.title} (\`${route.slug}\`)

> **TL;DR (Answer-First Summary):** ${tldr}

## 1. Core Specification & Gateway SLA

| Metric / Field | Value |
| :--- | :--- |
| **Canonical Hub URL** | ${PUBLIC_SITE_URL}/hub/${route.slug} |
| **Gateway Endpoint** | \`${PUBLIC_GATEWAY_URL}${route.path_pattern}\` |
| **Route Slug** | \`${route.slug}\` |
| **HTTP Methods** | \`${(route.methods || ['GET', 'POST']).join(', ')}\` |
| **Price** | \`${priceDisplay}\` |
| **Settlement Cluster** | \`${clusterLabel}\` |
| **24h 200 OK Success Rate** | \`${route.sla.success_rate_24h}%\` |
| **P95 Latency** | \`${route.sla.p95_latency_ms} ms\` |
| **24h Verified Paid Calls** | \`${route.sla.paid_calls_24h.toLocaleString()}\` |
| **Gateway Health Status** | \`${route.sla.health_status.toUpperCase()}\` |

## 2. Input Parameters Schema

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
${paramRows}
${sampleParamsBlock}
## 3. Zero-Friction SDK Quickstart (Python & TypeScript)

### Python SDK (\`pip install agentpay-core-sdk\`)

\`\`\`python
${sdkCode}
\`\`\`

### TypeScript SDK (\`npm install agentpay-core-sdk\`)

\`\`\`typescript
${tsSdkCode}
\`\`\`

## 4. MCP (Model Context Protocol)

${locale === 'en' ? 'Direct integration with **WorkBuddy**, **Claude Desktop**, and **Cursor** supporting standard MCP protocol:' : '支持直接接入 **腾讯 WorkBuddy**、**Claude Desktop**、**Cursor** 等支持标准 MCP 协议的智能体环境：'}

\`\`\`json
${buildMcpConfigSnippet(route, 'workbuddy', undefined, locale)}
\`\`\`

## 5. Detailed Documentation

${route.description_md}
`;
}

function applyLocalFilter(
  routes: MarketplaceRouteItem[],
  filter?: MarketplaceFilter
): MarketplaceRouteItem[] {
  if (!filter) return routes;
  return routes.filter((item) => {
    if (
      filter.category &&
      filter.category !== 'all' &&
      item.category.toLowerCase() !== filter.category.toLowerCase()
    ) {
      return false;
    }
    if (
      filter.network &&
      filter.network !== 'all' &&
      item.cluster &&
      !item.cluster.toLowerCase().includes(filter.network.toLowerCase())
    ) {
      return false;
    }
    if (filter.tag) {
      const hasTag = item.tags.some((t) => t.toLowerCase() === filter.tag!.toLowerCase());
      if (!hasTag) return false;
    }
    if (filter.q && filter.q.trim() !== '') {
      const q = filter.q.trim().toLowerCase();
      const haystack = [
        item.title,
        item.slug,
        item.summary || '',
        item.category,
        item.path_pattern,
        item.description_md,
        ...item.tags,
      ]
        .join(' ')
        .toLowerCase();
      if (!haystack.includes(q)) return false;
    }
    return true;
  });
}

/**
 * Fetches public marketplace routes from the Go gateway (`/api/v1/public/marketplace/routes`).
 * When live routes exist in the gateway/database, they are returned directly.
 * Gracefully falls back to seed catalog only when zero live routes are published or gateway is offline.
 */
export async function getMarketplaceRoutes(
  filter?: MarketplaceFilter
): Promise<MarketplaceRouteItem[]> {
  let liveRoutes: MarketplaceRouteItem[] = [];

  const candidateUrls = Array.from(
    new Set(
      [
        process.env.GATEWAY_INTERNAL_URL,
        process.env.NEXT_PUBLIC_GATEWAY_API_URL,
        'http://127.0.0.1:8000',
        'http://127.0.0.1:8080',
      ].filter(Boolean) as string[]
    )
  );

  for (const base of candidateUrls) {
    try {
      const url = new URL('/api/v1/public/marketplace/routes', base);
      if (filter?.q) url.searchParams.set('q', filter.q);
      if (filter?.category && filter.category !== 'all') {
        url.searchParams.set('category', filter.category);
      }
      if (filter?.network && filter.network !== 'all') {
        url.searchParams.set('network', filter.network);
      }
      if (filter?.tag) url.searchParams.set('tag', filter.tag);

      const res = await fetch(url.toString(), {
        next: { revalidate: 15 },
        headers: { Accept: 'application/json' },
        signal: AbortSignal.timeout(1800),
      });

      if (res.ok) {
        const data = (await res.json()) as { routes?: MarketplaceRouteItem[] };
        if (Array.isArray(data.routes) && data.routes.length > 0) {
          liveRoutes = data.routes;
          break;
        }
      }
    } catch {
      // try next candidate base URL
    }
  }

  // If live routes exist from Go gateway, display live data without mock seed data
  if (liveRoutes.length > 0) {
    return applyLocalFilter(liveRoutes, filter);
  }

  // Fallback gracefully to seed catalog when no public routes exist yet
  return applyLocalFilter(SEED_MARKETPLACE_ROUTES, filter);
}

/**
 * Fetches a single public marketplace route by slug from the Go gateway or seed catalog.
 */
export async function getRandomMarketplaceRoute(): Promise<MarketplaceRouteItem | null> {
  const routes = await getMarketplaceRoutes();
  if (!routes || routes.length === 0) return null;
  const randomIndex = Math.floor(Math.random() * routes.length);
  return routes[randomIndex];
}

export async function getMarketplaceRouteBySlug(
  slug: string
): Promise<MarketplaceRouteItem | null> {
  const cleanSlug = decodeURIComponent(slug || '').trim().toLowerCase();
  if (!cleanSlug) return null;

  const candidateUrls = Array.from(
    new Set(
      [
        process.env.GATEWAY_INTERNAL_URL,
        process.env.NEXT_PUBLIC_GATEWAY_API_URL,
        'http://127.0.0.1:8000',
        'http://127.0.0.1:8080',
      ].filter(Boolean) as string[]
    )
  );

  for (const base of candidateUrls) {
    try {
      const url = new URL(
        `/api/v1/public/marketplace/routes/${encodeURIComponent(cleanSlug)}`,
        base
      );
      const res = await fetch(url.toString(), {
        next: { revalidate: 15 },
        headers: { Accept: 'application/json' },
        signal: AbortSignal.timeout(1800),
      });

      if (res.ok) {
        const data = (await res.json()) as { route?: MarketplaceRouteItem };
        if (data.route) {
          return data.route;
        }
      }
    } catch {
      // try next candidate
    }
  }

  // Fallback to seed catalog if not found in live routes
  const found = SEED_MARKETPLACE_ROUTES.find(
    (item) =>
      item.slug.toLowerCase() === cleanSlug || item.route_id.toLowerCase() === cleanSlug
  );
  return found || null;
}
