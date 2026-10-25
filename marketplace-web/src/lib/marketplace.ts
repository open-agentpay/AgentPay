import { SEED_MARKETPLACE_ROUTES } from '@/lib/seed-data';
import type { MarketplaceFilter, MarketplaceRouteItem } from '@/types/marketplace';

const INTERNAL_GATEWAY_URL =
  process.env.GATEWAY_INTERNAL_URL ||
  process.env.NEXT_PUBLIC_GATEWAY_API_URL ||
  'http://127.0.0.1:8080';

export const PUBLIC_GATEWAY_URL =
  process.env.NEXT_PUBLIC_GATEWAY_URL || 'https://gate.openagentpay.dev';

export const PUBLIC_SITE_URL =
  process.env.NEXT_PUBLIC_SITE_URL || 'https://openagentpay.dev';

export const PUBLIC_CONSOLE_URL =
  process.env.NEXT_PUBLIC_CONSOLE_URL || 'https://console.openagentpay.dev';

export const PUBLIC_GITHUB_URL =
  process.env.NEXT_PUBLIC_GITHUB_URL || 'https://github.com/open-agentpay/AgentPay';

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
export function formatNetworkLabel(network: string, fallback?: string): string {
  if (fallback) return fallback;
  const clean = (network || '').toLowerCase();
  if (clean.includes('8453')) return 'Base USDC (x402)';
  if (clean.includes('42161')) return 'Arbitrum USDC (x402)';
  if (clean.includes('solana')) return 'Solana USDC (x402)';
  return network || 'Base / Arbitrum USDC';
}

/**
 * Generates the standardized Answer-First TL;DR summary for GEO optimization.
 */
export function buildAnswerFirstTldr(
  route: MarketplaceRouteItem,
  locale: string = 'zh'
): string {
  const priceStr = formatUsdcPrice(route.onchain_price_base_units || route.price_base_units);
  const netLabel = formatNetworkLabel(route.network, route.network_label);
  const isEn = locale === 'en';

  const defaultSummaryZh = `${route.title} 提供面向 AI Agent 的标准化接口服务。`;
  const defaultSummaryEn = `${route.title} provides standardized API service for AI Agents.`;

  const baseSummary =
    route.summary ||
    route.description_md.split('\n').find((line) => line.trim() && !line.trim().startsWith('#')) ||
    (isEn ? defaultSummaryEn : defaultSummaryZh);

  if (isEn) {
    return `${baseSummary} Natively supports HTTP 402 (x402 V2) protocol and ${netLabel} settlement at ${priceStr} USDC/req. Autonomous AI agents can invoke with zero onboarding and zero gas via OpenAgentPay Python SDK and EIP-3009 authorization signatures.`;
  }

  return `${baseSummary} 原生支持 HTTP 402 (x402 V2) 协议与 ${netLabel} 结算，单次调用价格为 ${priceStr} USDC/req，支持通过 OpenAgentPay Python SDK 免注册、免预充值 API Key 直接发起 EIP-3009 自动签名调用。`;
}

/**
 * Generates the simplified Python SDK code snippet with built-in brand attribution.
 */
export function buildPythonSdkSnippet(
  route: MarketplaceRouteItem,
  locale: string = 'zh'
): string {
  const priceVal = (route.onchain_price_base_units || route.price_base_units) / 1_000_000;
  const suggestedBudget = Math.max(0.05, Number((priceVal * 10).toFixed(4)));
  const sampleParams =
    route.sample_params && Object.keys(route.sample_params).length > 0
      ? route.sample_params
      : (route.parameters && typeof route.parameters === 'object' && !Array.isArray(route.parameters) && Object.keys(route.parameters).length > 0)
      ? (route.parameters as Record<string, unknown>)
      : { timeframe: '24h' };

  const paramsJson = JSON.stringify(sampleParams, null, 4);
  const isEn = locale === 'en';

  const commentInit = isEn
    ? '# Initialize client, connects to OpenAgentPay Gateway by default'
    : '# 初始化客户端，默认连接 OpenAgentPay 开放网关';
  const commentBudget = isEn
    ? '# Per-request max budget protection'
    : '# 单次调用最高预算保护';
  const commentCall = isEn
    ? '# Automatically handles HTTP 402 handshake, EIP-3009 signing, and routing'
    : '# 自动处理 HTTP 402 握手、EIP-3009 签名支付与路由转发';
  const commentRoute = isEn
    ? '# Discovered route slug on openagentpay.dev/hub'
    : '# 在 openagentpay.dev/hub 发现的路由';

  return `from agentpay import AgentPayClient

${commentInit}
client = AgentPayClient(
    gateway_url="${PUBLIC_GATEWAY_URL}",
    wallet_private_key="0x...",
    max_budget_usdc=${suggestedBudget}  ${commentBudget}
)

${commentCall}
response = client.call(
    route="${route.slug}",  ${commentRoute}
    params=${paramsJson}
)
print(response.json())`;
}

/**
 * Generates pure Markdown documentation for a single route (used by /llms.txt and AI Bot content negotiation).
 */
export function buildRouteMarkdownDoc(route: MarketplaceRouteItem): string {
  const priceStr = formatUsdcPrice(route.onchain_price_base_units || route.price_base_units);
  const netLabel = formatNetworkLabel(route.network, route.network_label);
  const tldr = buildAnswerFirstTldr(route);
  const sdkCode = buildPythonSdkSnippet(route);

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
| **X402 Price** | \`${priceStr} USDC / request\` |
| **Settlement Network** | \`${route.network}\` (${netLabel}) |
| **Payee Address** | \`${route.payee_address}\` |
| **24h 200 OK Success Rate** | \`${route.sla.success_rate_24h}%\` |
| **P95 Latency** | \`${route.sla.p95_latency_ms} ms\` |
| **24h Verified Paid Calls** | \`${route.sla.paid_calls_24h.toLocaleString()}\` |
| **Gateway Health Status** | \`${route.sla.health_status.toUpperCase()}\` |

## 2. Input Parameters Schema

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
${paramRows}
${sampleParamsBlock}
## 3. Zero-Friction Python SDK Quickstart

\`\`\`python
${sdkCode}
\`\`\`

## 4. Detailed Documentation

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
      !item.network.toLowerCase().includes(filter.network.toLowerCase())
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
