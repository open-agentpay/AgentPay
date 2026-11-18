'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import {
  Activity,
  ArrowLeft,
  Bot,
  BrainCircuit,
  CheckCircle2,
  Clock,
  Code2,
  Cpu,
  ExternalLink,
  Layers,
  Lock,
  ShieldCheck,
  Sparkles,
  Terminal,
  Zap,
} from 'lucide-react';
import {
  PUBLIC_CONSOLE_URL,
  PUBLIC_GATEWAY_URL,
  PUBLIC_SITE_URL,
  buildAnswerFirstTldr,
  buildCurlLlmSnippet,
  buildMcpConfigSnippet,
  buildOpenAiNodeSdkSnippet,
  buildOpenAiPythonSdkSnippet,
  buildPythonSdkSnippet,
  buildTypescriptSdkSnippet,
  formatClusterLabel,
  formatCreditsPrice,
  formatTokenPriceFen,
  getLlmModelName,
  isLlmRoute,
} from '@/lib/marketplace';
import type { MarketplaceParameter, MarketplaceRouteItem } from '@/types/marketplace';
import { CopyButton } from '@/components/CopyButton';
import { useI18n } from '@/i18n/context';

function flattenObjectToParams(
  obj: Record<string, unknown>,
  prefix = ''
): MarketplaceParameter[] {
  const result: MarketplaceParameter[] = [];
  for (const [key, val] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (val !== null && typeof val === 'object' && !Array.isArray(val)) {
      result.push({
        name: fullKey,
        type: 'object',
        required: true,
        description: `Nested object ${fullKey}`,
      });
      result.push(...flattenObjectToParams(val as Record<string, unknown>, fullKey));
    } else {
      let valType: string = typeof val;
      if (Array.isArray(val)) valType = 'array';
      else if (val === null) valType = 'null';
      result.push({
        name: fullKey,
        type: valType,
        required: true,
        description: `Parameter ${fullKey}`,
        example: typeof val === 'string' ? val : JSON.stringify(val),
      });
    }
  }
  return result;
}

export function resolveRouteParameters(
  parameters?: unknown,
  sampleParams?: Record<string, unknown>
): MarketplaceParameter[] {
  if (Array.isArray(parameters) && parameters.length > 0) {
    return parameters;
  }
  if (parameters && typeof parameters === 'object' && !Array.isArray(parameters) && Object.keys(parameters).length > 0) {
    const flattened = flattenObjectToParams(parameters as Record<string, unknown>);
    if (flattened.length > 0) return flattened;
  }
  if (sampleParams && typeof sampleParams === 'object' && Object.keys(sampleParams).length > 0) {
    const flattened = flattenObjectToParams(sampleParams);
    if (flattened.length > 0) return flattened;
  }
  return [];
}

interface RouteDetailViewProps {
  route: MarketplaceRouteItem;
  relatedRoutes: MarketplaceRouteItem[];
}

export function RouteDetailView({ route, relatedRoutes }: RouteDetailViewProps) {
  const { t, locale } = useI18n();
  const isLlm = isLlmRoute(route);
  const llmModelName = getLlmModelName(route);
  const promptTokenPriceFen = formatTokenPriceFen(route.prompt_token_price_base_units ?? 1000);
  const completionTokenPriceFen = formatTokenPriceFen(route.completion_token_price_base_units ?? 2000);

  const [sdkLang, setSdkLang] = useState<string>(isLlm ? 'openai_python' : 'python');
  const [mcpPlatform, setMcpPlatform] = useState<'workbuddy' | 'claude_cursor' | 'sdk'>('workbuddy');

  const activeApiKey = 'apk_live_xxx';
  const priceStr = formatCreditsPrice(route.price_base_units);
  const clusterLabel = formatClusterLabel(route.cluster, undefined, locale);
  const tldr = buildAnswerFirstTldr(route, locale);

  const openAiPythonSnippet = buildOpenAiPythonSdkSnippet(route, locale, activeApiKey);
  const openAiNodeSnippet = buildOpenAiNodeSdkSnippet(route, locale, activeApiKey);
  const curlLlmSnippet = buildCurlLlmSnippet(route, locale, activeApiKey);
  const pythonSnippet = buildPythonSdkSnippet(route, locale, activeApiKey);
  const tsSnippet = buildTypescriptSdkSnippet(route, locale, activeApiKey);
  const mcpSnippet = buildMcpConfigSnippet(route, mcpPlatform, activeApiKey, locale);

  let activeSnippet = pythonSnippet;
  let activeInstallCmd = 'pip install agentpay-core-sdk';

  if (isLlm) {
    if (sdkLang === 'openai_python') {
      activeSnippet = openAiPythonSnippet;
      activeInstallCmd = 'pip install openai';
    } else if (sdkLang === 'openai_node') {
      activeSnippet = openAiNodeSnippet;
      activeInstallCmd = 'npm install openai';
    } else if (sdkLang === 'curl') {
      activeSnippet = curlLlmSnippet;
      activeInstallCmd = `curl -N -X POST "${PUBLIC_GATEWAY_URL}/v1/chat/completions"`;
    } else if (sdkLang === 'mcp') {
      activeSnippet = mcpSnippet;
      activeInstallCmd =
        mcpPlatform === 'workbuddy'
          ? 'cat ~/.workbuddy/mcp.json'
          : mcpPlatform === 'claude_cursor'
          ? 'cat ~/.config/claude/claude_desktop_config.json'
          : 'npm install agentpay-core-sdk';
    } else {
      activeSnippet = pythonSnippet;
      activeInstallCmd = 'pip install agentpay-core-sdk';
    }
  } else {
    activeSnippet =
      sdkLang === 'python' ? pythonSnippet : sdkLang === 'typescript' ? tsSnippet : mcpSnippet;
    activeInstallCmd =
      sdkLang === 'python'
        ? 'pip install agentpay-core-sdk'
        : sdkLang === 'typescript'
        ? 'npm install agentpay-core-sdk'
        : mcpPlatform === 'workbuddy'
        ? 'cat ~/.workbuddy/mcp.json'
        : mcpPlatform === 'claude_cursor'
        ? 'cat ~/.config/claude/claude_desktop_config.json'
        : 'npm install agentpay-core-sdk';
  }
  const mcpSseUrl = `${PUBLIC_GATEWAY_URL}/v1/mcp/sse?route_id=${route.route_id || route.slug}`;
  const mcpCliCmd = `npx agentpay-cli mcp add --name ${route.slug || 'tool'} --url "${mcpSseUrl}" --key ${activeApiKey}`;
  const fullEndpoint = `${PUBLIC_GATEWAY_URL}${route.path_pattern}`;
  const isHealthy = route.sla.health_status === 'healthy';
  const categoryLabel = t.filter.categories[route.category] || route.category;

  const effectiveParameters = resolveRouteParameters(route.parameters, route.sample_params);
  const effectiveSampleParams =
    route.sample_params && Object.keys(route.sample_params).length > 0
      ? route.sample_params
      : (route.parameters && typeof route.parameters === 'object' && !Array.isArray(route.parameters) && Object.keys(route.parameters).length > 0)
      ? (route.parameters as Record<string, unknown>)
      : null;

  const curlHandshakeSnippet = `# 1. Directly call gateway endpoint to trigger standard HTTP 402 challenge
curl -i -X ${(route.methods || ['GET'])[0]} "${fullEndpoint}"

# 2. Or request machine-friendly Markdown specification
curl -H "Accept: text/markdown" "${PUBLIC_SITE_URL}/hub/${route.slug}"`;

  return (
    <div className="space-y-10">
      {/* Top Breadcrumb */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Link
          href="/marketplace"
          className="inline-flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-3.5 py-2 text-xs font-medium text-slate-700 shadow-sm transition hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-white"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>{t.detail.backToStore}</span>
        </Link>
      </div>

      {/* Header & Answer-First TL;DR Hero Card (GEO Core) */}
      <section className="rounded-3xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-800/90 dark:bg-gradient-to-b dark:from-slate-900/90 dark:to-[#090E1C] dark:shadow-2xl sm:p-10 transition-colors duration-200">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-2.5">
            {isLlm && (
              <span className="inline-flex items-center gap-1.5 rounded-lg border border-purple-500/40 bg-gradient-to-r from-purple-500/20 to-indigo-500/20 px-3 py-1 text-xs font-bold text-purple-700 dark:text-purple-300 shadow-sm">
                <BrainCircuit className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400" />
                <span>{locale === 'en' ? 'LLM Model' : 'LLM 大模型'}</span>
              </span>
            )}
            <span className="rounded-lg border border-indigo-500/30 bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300">
              {categoryLabel}
            </span>
            <span className="rounded-lg border border-slate-200 bg-slate-100 px-2.5 py-1 font-mono text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-800/80 dark:text-slate-300">
              {(route.methods || ['GET'])[0]} {route.path_pattern}
            </span>
            {isLlm && (
              <span className="rounded-lg border border-purple-500/30 bg-purple-50 px-2.5 py-1 font-mono text-xs font-semibold text-purple-700 dark:border-purple-800/60 dark:bg-purple-950/40 dark:text-purple-300">
                model: {llmModelName}
              </span>
            )}
            <span className="text-xs text-slate-500 dark:text-slate-400">
              {t.detail.provider} <strong className="text-slate-900 dark:text-slate-200">{route.provider_name}</strong>
            </span>
          </div>

          <span
            className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold ${
              isHealthy
                ? 'border-emerald-500/40 bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
                : 'border-amber-500/40 bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
            }`}
          >
            <span
              className={`h-2 w-2 rounded-full ${
                isHealthy
                  ? 'bg-emerald-500 dark:bg-emerald-400 animate-pulse'
                  : 'bg-amber-500 dark:bg-amber-400'
              }`}
            />
            {isHealthy ? t.detail.gatewayOnline : t.detail.gatewayDegraded}
          </span>
        </div>

        <h1 className="mt-5 text-2xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
          {route.title}
        </h1>

        {/* Answer-First TL;DR Block — Optimized for AI Search Engine Citations */}
        <div className="mt-6 rounded-2xl border border-indigo-200 bg-indigo-50/70 p-5 dark:border-indigo-500/30 dark:bg-indigo-950/25">
          <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-indigo-700 dark:text-indigo-300">
            <Sparkles className="h-4 w-4 text-indigo-600 dark:text-indigo-400" />
            <span>{t.detail.tldrTitle}</span>
          </div>
          <p className="mt-2 text-sm leading-relaxed text-slate-800 dark:text-slate-200 sm:text-base">
            {tldr}
          </p>
        </div>

        {/* Key Price & Live Gateway SLA Metrics Grid */}
        <div className="mt-7 grid grid-cols-2 gap-4 sm:grid-cols-4">
          {isLlm ? (
            <div className="col-span-2 rounded-2xl border border-purple-500/30 bg-purple-50/60 p-4 shadow-sm dark:border-purple-500/30 dark:bg-purple-950/20">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-1.5 text-xs font-bold text-purple-700 dark:text-purple-300">
                  <Sparkles className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400" />
                  <span>{locale === 'en' ? 'Millisecond Per-1k-Token Streaming Pricing' : '千 Tokens 毫秒级流式定价'}</span>
                </div>
                <span className="rounded-md bg-purple-100 px-2 py-0.5 font-mono text-[11px] font-semibold text-purple-800 dark:bg-purple-900/60 dark:text-purple-200">
                  {locale === 'en' ? 'No Monthly Fee · Pay As You Go' : '无月租 · 按量实收'}
                </span>
              </div>
              <div className="mt-2.5 grid grid-cols-2 gap-3 border-t border-purple-200/70 pt-2.5 dark:border-purple-800/50">
                <div>
                  <div className="text-[11px] font-medium text-slate-500 dark:text-slate-400">{locale === 'en' ? 'Input (Prompt)' : '输入 (Prompt)'}</div>
                  <div className="mt-1 flex items-baseline gap-1">
                    <span className="font-mono text-2xl font-extrabold text-slate-900 dark:text-white">
                      {promptTokenPriceFen}
                    </span>
                    <span className="font-mono text-xs font-semibold text-purple-600 dark:text-purple-400">
                      {locale === 'en' ? 'cents / 1k Tokens' : '分 / 1k Tokens'}
                    </span>
                  </div>
                </div>
                <div>
                  <div className="text-[11px] font-medium text-slate-500 dark:text-slate-400">{locale === 'en' ? 'Output (Completion)' : '输出 (Completion)'}</div>
                  <div className="mt-1 flex items-baseline gap-1">
                    <span className="font-mono text-2xl font-extrabold text-slate-900 dark:text-white">
                      {completionTokenPriceFen}
                    </span>
                    <span className="font-mono text-xs font-semibold text-purple-600 dark:text-purple-400">
                      {locale === 'en' ? 'cents / 1k Tokens' : '分 / 1k Tokens'}
                    </span>
                  </div>
                </div>
              </div>
              <div className="mt-2.5 flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400 border-t border-purple-200/40 pt-2 dark:border-purple-800/30">
                <span>{clusterLabel}</span>
                <span className="font-medium text-emerald-600 dark:text-emerald-400">
                  {locale === 'en'
                    ? '⚡ Millisecond cutoff protection, zero runaway token loss'
                    : '⚡ 毫秒级断流保护，绝不多扣 1 个 Token'}
                </span>
              </div>
            </div>
          ) : (
            <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950/70">
              <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                <Zap className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                <span>{t.detail.pricePerCall}</span>
              </div>
              <div className="mt-1.5 flex items-baseline gap-1.5">
                <span className="font-mono text-2xl font-extrabold text-slate-900 dark:text-white">
                  {priceStr}
                </span>
                <span className="font-mono text-xs font-semibold text-indigo-600 dark:text-indigo-400">
                  Credits / req
                </span>
              </div>
              <div className="mt-1 text-[11px] text-slate-500">{clusterLabel}</div>
            </div>
          )}

          <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950/70">
            <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
              <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
              <span>{t.detail.successRate24h}</span>
            </div>
            <div className="mt-1.5 font-mono text-2xl font-extrabold text-emerald-600 dark:text-emerald-400">
              {route.sla.success_rate_24h}%
            </div>
            <div className="mt-1 text-[11px] text-slate-500">
              {t.detail.successRateNote}
            </div>
          </div>

          <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950/70">
            <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
              <Clock className="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
              <span>{t.detail.p95Latency}</span>
            </div>
            <div className="mt-1.5 font-mono text-2xl font-extrabold text-slate-900 dark:text-white">
              {route.sla.p95_latency_ms}{' '}
              <span className="text-sm font-normal text-slate-500 dark:text-slate-400">ms</span>
            </div>
            <div className="mt-1 text-[11px] text-slate-500">
              {t.detail.timeoutNote.replace(
                '{timeout}',
                ((route.timeout_ms || 30000) / 1000).toString()
              )}
            </div>
          </div>

          <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950/70">
            <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
              <Activity className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
              <span>{t.detail.paidCalls24h}</span>
            </div>
            <div className="mt-1.5 font-mono text-2xl font-extrabold text-slate-900 dark:text-white">
              {route.sla.paid_calls_24h.toLocaleString()}
            </div>
            <div className="mt-1 text-[11px] text-slate-500">
              {t.detail.totalCallsNote.replace(
                '{total}',
                route.sla.total_calls.toLocaleString()
              )}
            </div>
          </div>
        </div>
      </section>

      {/* Main Two-Column Content: Left (Code & Native HTML Tables), Right (Architecture & Firewall Guarantee) */}
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        <div className="space-y-8 lg:col-span-2">
          {/* 1. Zero-Friction SDK Code Block (Python & TypeScript) */}
          <section className="rounded-2xl border border-slate-200 bg-slate-900 p-6 shadow-sm dark:border-slate-800/90 dark:bg-slate-900/60 text-white">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 pb-4">
              <div className="flex items-center gap-2.5">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-500/20 text-indigo-400">
                  <Code2 className="h-4 w-4" />
                </div>
                <div>
                  <h2 className="text-base font-bold text-white">
                    {t.detail.codeSectionTitle}
                  </h2>
                  <p className="text-xs text-slate-400">
                    {t.detail.codeSectionSubtitle}
                  </p>
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <div className="inline-flex rounded-lg border border-slate-700 bg-slate-950 p-0.5 text-xs">
                  {isLlm ? (
                    <>
                      <button
                        type="button"
                        onClick={() => setSdkLang('openai_python')}
                        className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                          sdkLang === 'openai_python'
                            ? 'bg-purple-600 text-white shadow-sm'
                            : 'text-slate-400 hover:text-white'
                        }`}
                      >
                        OpenAI Python
                      </button>
                      <button
                        type="button"
                        onClick={() => setSdkLang('openai_node')}
                        className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                          sdkLang === 'openai_node'
                            ? 'bg-purple-600 text-white shadow-sm'
                            : 'text-slate-400 hover:text-white'
                        }`}
                      >
                        OpenAI Node.js
                      </button>
                      <button
                        type="button"
                        onClick={() => setSdkLang('curl')}
                        className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                          sdkLang === 'curl'
                            ? 'bg-purple-600 text-white shadow-sm'
                            : 'text-slate-400 hover:text-white'
                        }`}
                      >
                        {locale === 'en' ? 'cURL Stream' : 'cURL 流式'}
                      </button>
                      <button
                        type="button"
                        onClick={() => setSdkLang('mcp')}
                        className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                          sdkLang === 'mcp'
                            ? 'bg-gradient-to-r from-emerald-600 to-teal-600 text-white shadow-sm'
                            : 'text-slate-400 hover:text-white'
                        }`}
                      >
                        {t.detail.mcpTabLabel}
                      </button>
                      <button
                        type="button"
                        onClick={() => setSdkLang('agentpay')}
                        className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                          sdkLang === 'agentpay'
                            ? 'bg-indigo-600 text-white shadow-sm'
                            : 'text-slate-400 hover:text-white'
                        }`}
                      >
                        {locale === 'en' ? 'Policy Firewall' : '策略防火墙'}
                      </button>
                    </>
                  ) : (
                    <>
                      <button
                        type="button"
                        onClick={() => setSdkLang('python')}
                        className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                          sdkLang === 'python'
                            ? 'bg-indigo-600 text-white shadow-sm'
                            : 'text-slate-400 hover:text-white'
                        }`}
                      >
                        Python
                      </button>
                      <button
                        type="button"
                        onClick={() => setSdkLang('typescript')}
                        className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                          sdkLang === 'typescript'
                            ? 'bg-indigo-600 text-white shadow-sm'
                            : 'text-slate-400 hover:text-white'
                        }`}
                      >
                        TypeScript
                      </button>
                      <button
                        type="button"
                        onClick={() => setSdkLang('mcp')}
                        className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                          sdkLang === 'mcp'
                            ? 'bg-gradient-to-r from-emerald-600 to-teal-600 text-white shadow-sm'
                            : 'text-slate-400 hover:text-white'
                        }`}
                      >
                        {t.detail.mcpTabLabel}
                      </button>
                    </>
                  )}
                </div>
                <CopyButton text={activeSnippet} label={t.detail.copySdkCode} />
              </div>
            </div>

            {isLlm && (
              <div className="mt-4 flex flex-wrap items-center justify-between gap-2 rounded-xl border border-purple-500/30 bg-purple-950/20 px-3.5 py-2.5 text-xs text-purple-200">
                <div className="flex items-center gap-2">
                  <Bot className="h-4 w-4 text-purple-400 shrink-0" />
                  <span>
                    {locale === 'en' ? (
                      <>
                        <strong>OpenAI SDK Native Compatible</strong>: set{' '}
                        <code>base_url=&quot;{PUBLIC_GATEWAY_URL}/v1&quot;</code> and <code>api_key</code> to invoke directly with built-in SSE typewriter streaming.
                      </>
                    ) : (
                      <>
                        <strong>OpenAI 官方 SDK 原生兼容</strong>：设置{' '}
                        <code>base_url=&quot;{PUBLIC_GATEWAY_URL}/v1&quot;</code> 与 <code>api_key</code> 即可直接调用，开箱支持 SSE 打字机流式输出。
                      </>
                    )}
                  </span>
                </div>
                <span className="font-mono text-purple-300 bg-purple-900/60 px-2 py-0.5 rounded text-[11px] border border-purple-700/50">
                  model: {llmModelName}
                </span>
              </div>
            )}

            {sdkLang === 'mcp' && (
              <div className="mt-4 space-y-3">
                {/* MCP Platform Sub-switch */}
                <div className="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 pb-3">
                  <div className="flex items-center gap-1.5 text-xs">
                    <span className="text-slate-400">
                      {locale === 'en' ? 'Client Environment:' : '客户端环境:'}
                    </span>
                    <button
                      type="button"
                      onClick={() => setMcpPlatform('workbuddy')}
                      className={`rounded-lg px-2.5 py-1 font-medium transition ${
                        mcpPlatform === 'workbuddy'
                          ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
                          : 'text-slate-400 hover:text-slate-200 border border-transparent'
                      }`}
                    >
                      {t.detail.mcpWorkBuddyLabel}
                    </button>
                    <button
                      type="button"
                      onClick={() => setMcpPlatform('claude_cursor')}
                      className={`rounded-lg px-2.5 py-1 font-medium transition ${
                        mcpPlatform === 'claude_cursor'
                          ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
                          : 'text-slate-400 hover:text-slate-200 border border-transparent'
                      }`}
                    >
                      {t.detail.mcpClaudeCursorLabel}
                    </button>
                    <button
                      type="button"
                      onClick={() => setMcpPlatform('sdk')}
                      className={`rounded-lg px-2.5 py-1 font-medium transition ${
                        mcpPlatform === 'sdk'
                          ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
                          : 'text-slate-400 hover:text-slate-200 border border-transparent'
                      }`}
                    >
                      {t.detail.mcpSdkLabel}
                    </button>
                  </div>
                  <span className="text-xs font-mono text-emerald-400">
                    SSE Endpoint: {mcpSseUrl}
                  </span>
                </div>

                {/* Financial Guardrail Alert */}
                <div className="rounded-xl border border-emerald-500/30 bg-emerald-950/30 p-3 text-xs text-emerald-300">
                  <div className="flex items-center gap-2">
                    <ShieldCheck className="h-4 w-4 shrink-0 text-emerald-400" />
                    <span>
                      {t.detail.mcpProtectedBadge.replace('{price}', priceStr)}
                    </span>
                  </div>
                </div>

                {/* Quick CLI command */}
                <div className="rounded-xl border border-slate-800 bg-slate-950 p-2.5 text-xs text-slate-300 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2 font-mono truncate">
                    <span className="text-slate-500">$</span>
                    <span className="truncate">{mcpCliCmd}</span>
                  </div>
                  <CopyButton text={mcpCliCmd} label={locale === 'en' ? 'Copy CLI' : '复制 CLI'} />
                </div>
              </div>
            )}

            <div className="mt-4">
              <div className="mb-2 flex items-center justify-between text-xs text-slate-400">
                <div className="flex items-center gap-2 font-mono">
                  <span>$ {activeInstallCmd}</span>
                  <CopyButton text={activeInstallCmd} />
                </div>
                <span className="font-mono text-indigo-300">
                  {sdkLang === 'openai_python'
                    ? 'Python 3.8+ / openai >= 1.0.0'
                    : sdkLang === 'openai_node'
                    ? 'Node.js 18+ / openai >= 4.0.0'
                    : sdkLang === 'curl'
                    ? 'Bash / zsh / cURL (HTTP SSE)'
                    : sdkLang === 'python' || sdkLang === 'agentpay'
                    ? (locale === 'en' ? 'Python 3.10+ (Policy Firewall)' : 'Python 3.10+ (策略防火墙)')
                    : sdkLang === 'typescript'
                    ? 'Node.js 20+ / TypeScript'
                    : mcpPlatform === 'workbuddy'
                    ? (locale === 'en' ? 'WorkBuddy Desktop' : '腾讯 WorkBuddy Desktop')
                    : mcpPlatform === 'claude_cursor'
                    ? 'Cursor / Claude Desktop'
                    : 'agentpay-core-sdk 0.3.1+'}
                </span>
              </div>
              <pre className="overflow-x-auto rounded-xl border border-slate-800 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-slate-100 sm:text-sm">
                <code>{activeSnippet}</code>
              </pre>
            </div>
          </section>

          {/* 2. Native HTML <table> for Core Endpoint & X402 Protocol Parameters */}
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800/90 dark:bg-slate-900/60">
            <div className="flex items-center gap-2.5 border-b border-slate-200 pb-4 dark:border-slate-800">
              <Layers className="h-5 w-5 text-cyan-600 dark:text-cyan-400" />
              <div>
                <h2 className="text-base font-bold text-slate-900 dark:text-white">
                  {t.detail.tableSpecsTitle}
                </h2>
                <p className="text-xs text-slate-500 dark:text-slate-400">
                  {t.detail.tableSpecsSubtitle}
                </p>
              </div>
            </div>

            <div className="mt-4 overflow-x-auto">
              <table className="w-full border-collapse text-left text-sm">
                <thead>
                  <tr className="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wider text-slate-600 dark:border-slate-800 dark:bg-slate-950/60 dark:text-slate-400">
                    <th className="px-4 py-3 font-semibold">{t.detail.paramCol}</th>
                    <th className="px-4 py-3 font-semibold">{t.detail.valueCol}</th>
                    <th className="px-4 py-3 font-semibold">{t.detail.notesCol}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200 text-slate-800 dark:divide-slate-800/70 dark:text-slate-200">
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.routeIdParam}</td>
                    <td className="px-4 py-3 font-mono text-xs font-semibold text-amber-600 dark:text-amber-400">
                      <div className="flex items-center gap-2">
                        <span>{route.route_id}</span>
                        <CopyButton text={route.route_id} label={locale === 'en' ? 'Copy' : '复制'} />
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.routeIdNote}
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.slugParam}</td>
                    <td className="px-4 py-3 font-mono text-xs text-indigo-600 dark:text-indigo-300">{route.slug}</td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.slugNote}
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.mcpParam}</td>
                    <td className="px-4 py-3 font-mono text-xs text-emerald-600 dark:text-emerald-300">
                      <div className="flex items-center gap-2 truncate max-w-xs sm:max-w-md">
                        <span className="truncate">{mcpSseUrl}</span>
                        <CopyButton text={mcpSseUrl} label={locale === 'en' ? 'Copy' : '复制'} />
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.mcpNote}
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.endpointParam}</td>
                    <td className="px-4 py-3 font-mono text-xs text-cyan-700 dark:text-cyan-300">{fullEndpoint}</td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.endpointNote.replace(
                        '{methods}',
                        (route.methods || ['GET', 'POST']).join(' / ')
                      )}
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.priceParam}</td>
                    <td className="px-4 py-3 font-mono text-xs font-bold text-emerald-600 dark:text-emerald-400">
                      {priceStr} Credits / req ({route.price_base_units} base units)
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.priceNote}
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.networkParam}</td>
                    <td className="px-4 py-3 font-mono text-xs text-slate-700 dark:text-slate-200">
                      {clusterLabel}
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.networkNote}
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.slaParam}</td>
                    <td className="px-4 py-3 font-mono text-xs text-emerald-600 dark:text-emerald-300">
                      {route.sla.success_rate_24h}% | P95 {route.sla.p95_latency_ms}ms
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.slaNote
                        .replace('{rpm}', (route.rate_limit_rpm || 600).toString())
                        .replace('{timeout}', ((route.timeout_ms || 30000) / 1000).toString())}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          {/* 3. Native HTML <table> for Input Parameters Schema */}
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800/90 dark:bg-slate-900/60">
            <div className="flex items-center justify-between gap-2 border-b border-slate-200 pb-4 dark:border-slate-800">
              <div>
                <h2 className="text-base font-bold text-slate-900 dark:text-white">
                  {t.detail.tableInputTitle}
                </h2>
                <p className="text-xs text-slate-500 dark:text-slate-400">
                  {t.detail.tableInputSubtitle}
                </p>
              </div>
            </div>

            <div className="mt-4 overflow-x-auto">
              <table className="w-full border-collapse text-left text-sm">
                <thead>
                  <tr className="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wider text-slate-600 dark:border-slate-800 dark:bg-slate-950/60 dark:text-slate-400">
                    <th className="px-4 py-3 font-semibold">{t.detail.fieldCol}</th>
                    <th className="px-4 py-3 font-semibold">{t.detail.typeCol}</th>
                    <th className="px-4 py-3 font-semibold">{t.detail.requiredCol}</th>
                    <th className="px-4 py-3 font-semibold">{t.detail.descCol}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200 text-slate-800 dark:divide-slate-800/70 dark:text-slate-200">
                  {effectiveParameters.length > 0 ? (
                    effectiveParameters.map((param) => (
                      <tr key={param.name}>
                        <td className="px-4 py-3 font-mono text-xs font-semibold text-indigo-600 dark:text-indigo-300">
                          {param.name.includes('.') ? (
                            <div className="flex items-center gap-1.5" style={{ paddingLeft: `${(param.name.split('.').length - 1) * 12}px` }}>
                              <span className="text-slate-400 dark:text-slate-500 font-sans">↳</span>
                              <span>{param.name}</span>
                            </div>
                          ) : (
                            param.name
                          )}
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-cyan-700 dark:text-cyan-300">{param.type}</td>
                        <td className="px-4 py-3 text-xs">
                          {param.required ? (
                            <span className="rounded bg-rose-50 px-2 py-0.5 font-mono text-[11px] text-rose-700 dark:bg-rose-500/20 dark:text-rose-300">
                              {t.detail.requiredBadge}
                            </span>
                          ) : (
                            <span className="rounded bg-slate-100 px-2 py-0.5 font-mono text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-400">
                              {t.detail.optionalBadge}
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-xs text-slate-700 dark:text-slate-300">
                          <div>{param.description || `Parameter ${param.name}`}</div>
                          {param.example && (
                            <div className="mt-1 font-mono text-[11px] text-slate-500">
                              e.g.: <code className="text-slate-700 dark:text-slate-300">{param.example}</code>
                            </div>
                          )}
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td className="px-4 py-3 font-mono text-xs text-indigo-600 dark:text-indigo-300">params</td>
                      <td className="px-4 py-3 font-mono text-xs text-cyan-700 dark:text-cyan-300">object (JSON)</td>
                      <td className="px-4 py-3 text-xs text-slate-500">{t.detail.optionalBadge}</td>
                      <td className="px-4 py-3 text-xs text-slate-700 dark:text-slate-300">
                        {t.detail.descCol}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {/* Calling input sample: Sample Request Payload */}
            {effectiveSampleParams && (
              <div className="mt-6">
                <div className="mb-2 flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
                  <span className="font-semibold text-slate-800 dark:text-slate-300">
                    {t.detail.sampleInputTitle}
                  </span>
                  <span className="font-mono text-cyan-600 dark:text-cyan-400">application/json</span>
                </div>
                <pre className="overflow-x-auto rounded-xl border border-slate-800 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-cyan-300">
                  <code>{JSON.stringify(effectiveSampleParams, null, 2)}</code>
                </pre>
              </div>
            )}

            {/* 200 OK Sample Output */}
            {route.sample_response && (
              <div className="mt-6">
                <div className="mb-2 flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
                  <span className="font-semibold text-slate-800 dark:text-slate-300">
                    {t.detail.sampleOutputTitle}
                  </span>
                  <span className="font-mono text-emerald-600 dark:text-emerald-400">application/json</span>
                </div>
                <pre className="overflow-x-auto rounded-xl border border-slate-800 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-emerald-300">
                  <code>{JSON.stringify(route.sample_response, null, 2)}</code>
                </pre>
              </div>
            )}
          </section>

          {/* 4. Semantic Description & HTTP 402 Raw cURL Verification */}
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800/90 dark:bg-slate-900/60 space-y-6">
            <div>
              <h2 className="text-base font-bold text-slate-900 dark:text-white">
                {t.detail.docsTitle}
              </h2>
              <div className="mt-3 whitespace-pre-line text-sm leading-relaxed text-slate-700 dark:text-slate-300">
                {route.description_md}
              </div>
            </div>

            <div className="border-t border-slate-200 pt-5 dark:border-slate-800">
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-semibold text-slate-800 dark:text-slate-300">
                  {t.detail.curlTitle}
                </span>
                <CopyButton text={curlHandshakeSnippet} label={t.detail.copyCurl} />
              </div>
              <pre className="mt-2.5 overflow-x-auto rounded-xl border border-slate-800 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-slate-300">
                <code>{curlHandshakeSnippet}</code>
              </pre>
            </div>
          </section>
        </div>

        {/* Right Sidebar: Financial Firewall Guarantees, Tags & Related X402 Routes */}
        <aside className="space-y-6">
          {/* Financial Firewall Card */}
          <div className="rounded-2xl border border-indigo-200 bg-white p-6 shadow-sm dark:border-indigo-500/30 dark:bg-slate-900/80">
            <div className="flex items-center gap-2.5">
              <ShieldCheck className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />
              <h3 className="text-sm font-bold text-slate-900 dark:text-white">
                {t.detail.firewallTitle}
              </h3>
            </div>

            <ul className="mt-4 space-y-3.5 text-xs leading-relaxed text-slate-700 dark:text-slate-300">
              <li className="flex items-start gap-2.5">
                <Lock className="mt-0.5 h-4 w-4 shrink-0 text-indigo-600 dark:text-indigo-400" />
                <span>
                  <strong className="text-slate-900 dark:text-white">{t.detail.firewall1Title}</strong>
                  {t.detail.firewall1Desc}
                </span>
              </li>
              <li className="flex items-start gap-2.5">
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
                <span>
                  <strong className="text-slate-900 dark:text-white">{t.detail.firewall2Title}</strong>
                  {t.detail.firewall2Desc}
                </span>
              </li>
              <li className="flex items-start gap-2.5">
                <Cpu className="mt-0.5 h-4 w-4 shrink-0 text-cyan-600 dark:text-cyan-400" />
                <span>
                  <strong className="text-slate-900 dark:text-white">{t.detail.firewall3Title}</strong>
                  {t.detail.firewall3Desc}
                </span>
              </li>
            </ul>

            <div className="mt-6 space-y-2.5 border-t border-slate-200 pt-4 dark:border-slate-800">
              <a
                href={PUBLIC_CONSOLE_URL}
                target="_blank"
                rel="noreferrer"
                className="flex w-full items-center justify-center gap-2 rounded-xl bg-indigo-600 px-4 py-2.5 text-xs font-semibold text-white shadow-sm transition hover:bg-indigo-500"
              >
                <Terminal className="h-4 w-4" />
                <span>{t.detail.btnBudgetPolicy}</span>
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            </div>
          </div>

          {/* Semantic Tags Card */}
          <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900/60">
            <h3 className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
              {t.detail.tagsTitle}
            </h3>
            <div className="mt-3 flex flex-wrap gap-2">
              {route.tags.map((tag) => (
                <span
                  key={tag}
                  className="rounded-lg border border-slate-200 bg-slate-100 px-2.5 py-1 font-mono text-xs text-slate-700 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-300"
                >
                  #{tag}
                </span>
              ))}
            </div>
          </div>

          {/* Related X402 APIs */}
          <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900/60">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
                {t.detail.relatedTitle}
              </h3>
              <Link href="/marketplace" className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline">
                {t.detail.viewAll}
              </Link>
            </div>

            <div className="mt-4 space-y-3">
              {relatedRoutes.map((rel) => (
                <Link
                  key={rel.slug}
                  href={`/hub/${rel.slug}`}
                  className="block rounded-xl border border-slate-200 bg-slate-50 p-3.5 transition hover:border-indigo-400 hover:bg-white dark:border-slate-800/80 dark:bg-slate-950/60 dark:hover:border-indigo-500/40 dark:hover:bg-slate-900"
                >
                  <div className="text-xs font-bold text-slate-900 dark:text-slate-100">{rel.title}</div>
                  <div className="mt-1 flex items-center justify-between font-mono text-[11px] text-slate-500 dark:text-slate-400">
                    <span>
                      {formatCreditsPrice(rel.price_base_units)} Credits/req
                    </span>
                    <span className="text-emerald-600 dark:text-emerald-400">{rel.sla.success_rate_24h}% OK</span>
                  </div>
                </Link>
              ))}
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}
