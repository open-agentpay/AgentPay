'use client';

import React from 'react';
import Link from 'next/link';
import {
  Activity,
  ArrowLeft,
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
  buildPythonSdkSnippet,
  formatNetworkLabel,
  formatUsdcPrice,
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

  const priceStr = formatUsdcPrice(
    route.onchain_price_base_units || route.price_base_units
  );
  const netLabel = formatNetworkLabel(route.network, route.network_label);
  const tldr = buildAnswerFirstTldr(route, locale);
  const pythonSnippet = buildPythonSdkSnippet(route, locale);
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
            <span className="rounded-lg border border-indigo-500/30 bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300">
              {categoryLabel}
            </span>
            <span className="rounded-lg border border-slate-200 bg-slate-100 px-2.5 py-1 font-mono text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-800/80 dark:text-slate-300">
              route=&quot;{route.slug}&quot;
            </span>
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
                USDC / req
              </span>
            </div>
            <div className="mt-1 text-[11px] text-slate-500">{netLabel}</div>
          </div>

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
          {/* 1. Zero-Friction Python SDK Code Block (Always dark for crystal syntax) */}
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

              <CopyButton text={pythonSnippet} label={t.detail.copySdkCode} />
            </div>

            <div className="mt-4">
              <div className="mb-2 flex items-center justify-between text-xs text-slate-400">
                <span className="font-mono">pip install open-agentpay</span>
                <span className="font-mono text-indigo-300">Python 3.10+</span>
              </div>
              <pre className="overflow-x-auto rounded-xl border border-slate-800 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-slate-100 sm:text-sm">
                <code>{pythonSnippet}</code>
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
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.slugParam}</td>
                    <td className="px-4 py-3 font-mono text-xs text-indigo-600 dark:text-indigo-300">{route.slug}</td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.slugNote}
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
                      {priceStr} USDC / req ({route.onchain_price_base_units || route.price_base_units}{' '}
                      base units)
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.priceNote}
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.networkParam}</td>
                    <td className="px-4 py-3 font-mono text-xs text-slate-700 dark:text-slate-200">
                      {route.network} ({netLabel})
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.networkNote}
                    </td>
                  </tr>
                  <tr>
                    <td className="px-4 py-3 font-medium text-slate-700 dark:text-slate-300">{t.detail.payeeParam}</td>
                    <td className="px-4 py-3 font-mono text-xs text-slate-700 dark:text-slate-300">
                      {route.payee_address}
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500 dark:text-slate-400">
                      {t.detail.payeeNote}
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
                      {formatUsdcPrice(rel.onchain_price_base_units || rel.price_base_units)} USDC/req
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
