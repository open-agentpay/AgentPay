'use client';

import React, { useMemo, useState } from 'react';
import Link from 'next/link';
import {
  Activity,
  ArrowUpRight,
  CheckCircle2,
  Clock,
  Code2,
  Cpu,
  ExternalLink,
  FileText,
  Filter,
  Flame,
  FlaskConical,
  Globe,
  Layers,
  Search,
  Shield,
  ShieldCheck,
  Sparkles,
  Terminal,
  Zap,
} from 'lucide-react';
import { MARKETPLACE_CATEGORIES } from '@/lib/seed-data';
import {
  PUBLIC_CONSOLE_URL,
  PUBLIC_GATEWAY_URL,
  formatNetworkLabel,
  formatUsdcPrice,
  buildPythonSdkSnippet,
} from '@/lib/marketplace';
import type { MarketplaceRouteItem } from '@/types/marketplace';
import { CopyButton } from '@/components/CopyButton';
import { useI18n } from '@/i18n/context';

interface MarketplaceExplorerProps {
  initialRoutes: MarketplaceRouteItem[];
}

export function MarketplaceExplorer({ initialRoutes }: MarketplaceExplorerProps) {
  const { t, locale } = useI18n();
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [selectedNetwork, setSelectedNetwork] = useState<string>('all');
  const [activeQuickPreviewSlug, setActiveQuickPreviewSlug] = useState<string | null>(null);

  const filteredRoutes = useMemo(() => {
    return initialRoutes.filter((route) => {
      if (
        selectedCategory !== 'all' &&
        route.category.toLowerCase() !== selectedCategory.toLowerCase()
      ) {
        return false;
      }
      if (selectedNetwork !== 'all') {
        const net = route.network.toLowerCase();
        if (selectedNetwork === 'base' && !net.includes('8453')) return false;
        if (selectedNetwork === 'arbitrum' && !net.includes('42161')) return false;
      }
      if (searchQuery.trim() !== '') {
        const q = searchQuery.trim().toLowerCase();
        const haystack = [
          route.title,
          route.slug,
          route.summary || '',
          route.category,
          route.path_pattern,
          route.provider_name,
          ...route.tags,
        ]
          .join(' ')
          .toLowerCase();
        if (!haystack.includes(q)) return false;
      }
      return true;
    });
  }, [initialRoutes, searchQuery, selectedCategory, selectedNetwork]);

  const totalPaidCalls24h = useMemo(
    () => initialRoutes.reduce((acc, r) => acc + (r.sla?.paid_calls_24h || 0), 0),
    [initialRoutes]
  );

  const avgSuccessRate = useMemo(() => {
    if (initialRoutes.length === 0) return '99.9';
    const sum = initialRoutes.reduce((acc, r) => acc + (r.sla?.success_rate_24h || 99.8), 0);
    return (sum / initialRoutes.length).toFixed(2);
  }, [initialRoutes]);

  const previewRoute = useMemo(() => {
    if (!activeQuickPreviewSlug) return null;
    return initialRoutes.find((r) => r.slug === activeQuickPreviewSlug) || null;
  }, [initialRoutes, activeQuickPreviewSlug]);

  const sampleQuickVerifySnippet = useMemo(() => {
    const isEn = locale === 'en';
    const comment1 = isEn
      ? '# 1. Initialize client & configure local financial firewall (private key never leaves local machine)'
      : '# 1. 初始化客户端并配置本地财务防火墙（私钥永不离机）';
    const comment2 = isEn
      ? '# Per-request cap, local policy engine blocks execution if budget exceeded'
      : '# 单次最高预算，超额直接由本地策略引擎拦截';
    const comment3 = isEn
      ? '# 2. Test & validate calling any API from this storefront (e.g. Solana Smart Money, DeepSeek, etc.)'
      : '# 2. 验证调用本功能商店中的任意 API（如 Solana 聪明钱、DeepSeek 推理等）';

    return `from agentpay import AgentPayClient

${comment1}
client = AgentPayClient(
    gateway_url="${PUBLIC_GATEWAY_URL}",
    wallet_private_key="0x...",
    max_budget_usdc=0.05  ${comment2}
)

${comment3}
response = client.call(route="solana-smart-money-tracker-x402-api")
print(response.json())`;
  }, [locale]);

  return (
    <div className="space-y-12">
      {/* Hero Banner & Search Bar */}
      <section className="relative overflow-hidden rounded-3xl border border-slate-200 bg-white/80 dark:border-slate-800/90 dark:bg-gradient-to-b dark:from-slate-900/90 dark:via-[#0B1120] dark:to-[#080D1A] p-8 shadow-xl sm:p-12 transition-colors duration-200">
        <div className="pointer-events-none absolute -top-24 left-1/2 h-72 w-[680px] -translate-x-1/2 rounded-full bg-indigo-500/10 dark:bg-indigo-500/15 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-24 right-10 h-60 w-80 rounded-full bg-emerald-500/10 blur-3xl" />

        <div className="relative z-10 mx-auto max-w-4xl text-center">
          <div className="mb-5 inline-flex flex-wrap items-center justify-center gap-2 rounded-full border border-indigo-500/30 bg-indigo-500/10 px-4 py-1.5 text-xs font-medium text-indigo-700 dark:text-indigo-300">
            <Sparkles className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
            <span>{t.hero.badgeEconomy}</span>
            <span className="text-slate-400 dark:text-slate-600">•</span>
            <span className="text-emerald-700 dark:text-emerald-300">{t.hero.badgeGas}</span>
            <span className="text-slate-400 dark:text-slate-600">•</span>
            <span className="inline-flex items-center gap-1 text-slate-600 dark:text-slate-300">
              <Shield className="h-3 w-3 text-indigo-600 dark:text-indigo-400" />
              <span>{locale === 'zh' ? '自主 AI Agent 专属' : 'Built for Autonomous Agents'}</span>
            </span>
          </div>

          <h1 className="text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-5xl sm:leading-[1.15]">
            {t.hero.titlePre}{' '}
            <span className="bg-gradient-to-r from-indigo-600 via-cyan-600 to-emerald-600 dark:from-indigo-400 dark:via-cyan-300 dark:to-emerald-400 bg-clip-text text-transparent">
              {t.hero.titleHighlight}
            </span>
          </h1>

          <p className="mx-auto mt-4 max-w-2xl text-base leading-relaxed text-slate-600 dark:text-slate-300 sm:text-lg">
            {t.hero.description}
          </p>

          {/* Search Input Box */}
          <div className="mx-auto mt-8 max-w-2xl">
            <div className="group relative flex items-center rounded-2xl border border-slate-300 bg-white p-2 shadow-sm transition-all focus-within:border-indigo-500 focus-within:ring-2 focus-within:ring-indigo-500/20 dark:border-slate-700/80 dark:bg-slate-950/90 dark:shadow-inner">
              <Search className="ml-3.5 h-5 w-5 shrink-0 text-slate-400 group-focus-within:text-indigo-600 dark:group-focus-within:text-indigo-400" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder={t.hero.searchPlaceholder}
                className="w-full bg-transparent px-3.5 py-2.5 text-sm text-slate-900 placeholder-slate-400 focus:outline-none dark:text-white dark:placeholder-slate-500 sm:text-base"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => setSearchQuery('')}
                  className="mr-2 rounded-lg px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 hover:text-slate-800 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
                >
                  {t.hero.clear}
                </button>
              )}
              <span className="hidden shrink-0 rounded-xl border border-slate-200 bg-slate-100 px-3 py-1.5 font-mono text-xs text-slate-600 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-400 sm:inline-block">
                {filteredRoutes.length} {t.hero.availableRoutes}
              </span>
            </div>

            {/* Quick Search Hot Tags */}
            <div className="mt-3.5 flex flex-wrap items-center justify-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <span className="inline-flex items-center gap-1 text-slate-600 dark:text-slate-500">
                <Flame className="h-3.5 w-3.5 text-amber-500 dark:text-amber-400" />
                {t.hero.hotSearches}
              </span>
              {['Solana', 'DeepSeek-R1', 'Markdown Scraper', 'SEC 10-Q', 'Honeypot', 'Python Sandbox'].map(
                (tag) => (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => setSearchQuery(tag === searchQuery ? '' : tag)}
                    className={`rounded-full border px-2.5 py-0.5 transition-colors ${
                      searchQuery.toLowerCase() === tag.toLowerCase()
                        ? 'border-indigo-500/60 bg-indigo-50 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-200'
                        : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-white'
                    }`}
                  >
                    {tag}
                  </button>
                )
              )}
            </div>
          </div>

          {/* Live Platform Telemetry Strip */}
          <div className="mt-9 grid grid-cols-2 gap-3 border-t border-slate-200 pt-7 dark:border-slate-800/80 sm:grid-cols-4">
            <div className="rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left dark:border-slate-800/60 dark:bg-slate-900/40">
              <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                <Layers className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
                <span>{t.telemetry.listedRoutes}</span>
              </div>
              <div className="mt-1 font-mono text-xl font-bold text-slate-900 dark:text-white">
                {initialRoutes.length} <span className="text-xs font-normal text-emerald-600 dark:text-emerald-400">{t.telemetry.online100}</span>
              </div>
            </div>

            <div className="rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left dark:border-slate-800/60 dark:bg-slate-900/40">
              <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                <span>{t.telemetry.successRate24h}</span>
              </div>
              <div className="mt-1 font-mono text-xl font-bold text-emerald-600 dark:text-emerald-400">
                {avgSuccessRate}% <span className="text-xs font-normal text-slate-500 dark:text-slate-400">200 OK</span>
              </div>
            </div>

            <div className="rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left dark:border-slate-800/60 dark:bg-slate-900/40">
              <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                <Zap className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                <span>{t.telemetry.paidCalls24h}</span>
              </div>
              <div className="mt-1 font-mono text-xl font-bold text-slate-900 dark:text-white">
                {totalPaidCalls24h.toLocaleString()}{' '}
                <span className="text-xs font-normal text-slate-500 dark:text-slate-400">reqs</span>
              </div>
            </div>

            <div className="rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left dark:border-slate-800/60 dark:bg-slate-900/40">
              <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                <Cpu className="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
                <span>{t.telemetry.discoveryProtocols}</span>
              </div>
              <div className="mt-1 flex items-center gap-2 font-mono text-sm font-semibold text-cyan-600 dark:text-cyan-300">
                <span>x402 V2</span>
                <span className="text-slate-400 dark:text-slate-600">•</span>
                <span>JSON-LD</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===================================================================== */}
      {/* AgentPay Core Product Overview & SDK Verification Hub */}
      {/* ===================================================================== */}
      <section className="relative overflow-hidden rounded-3xl border border-indigo-200 bg-white/90 p-8 shadow-xl dark:border-indigo-500/30 dark:bg-gradient-to-b dark:from-[#0B1120] dark:via-slate-900/90 dark:to-[#070B14] sm:p-10 transition-colors duration-200">
        <div className="pointer-events-none absolute -right-20 -top-20 h-64 w-64 rounded-full bg-indigo-500/10 blur-3xl" />
        <div className="pointer-events-none absolute -left-20 -bottom-20 h-64 w-64 rounded-full bg-cyan-500/10 blur-3xl" />

        <div className="relative z-10 space-y-8">
          {/* Header & Product Vision */}
          <div>
            <div className="inline-flex items-center gap-2 rounded-full border border-indigo-500/30 bg-indigo-50 px-3.5 py-1 text-xs font-semibold text-indigo-700 dark:border-indigo-500/40 dark:bg-indigo-500/15 dark:text-indigo-300">
              <Shield className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
              <span>{t.agentPayIntro.badge}</span>
            </div>
            <h2 className="mt-3 text-2xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-3xl">
              {t.agentPayIntro.title}
            </h2>
            <p className="mt-2 text-sm leading-relaxed text-slate-600 dark:text-slate-300 sm:text-base">
              {t.agentPayIntro.subtitle}
            </p>
          </div>

          {/* Role of this Marketplace Box (Highlighting SDK verification relationship) */}
          <div className="rounded-2xl border border-cyan-300/60 bg-gradient-to-r from-cyan-50/70 via-indigo-50/40 to-slate-50 p-5 shadow-sm dark:border-cyan-500/30 dark:bg-gradient-to-r dark:from-cyan-950/30 dark:via-slate-900/90 dark:to-indigo-950/30 dark:shadow-lg sm:p-6">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-cyan-500/30 bg-cyan-500/10 text-cyan-700 dark:border-cyan-500/40 dark:bg-cyan-500/20 dark:text-cyan-300 shadow-sm">
                <FlaskConical className="h-6 w-6 text-cyan-600 dark:text-cyan-400" />
              </div>
              <div className="space-y-1.5">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="rounded-md border border-cyan-500/30 bg-cyan-100 px-2 py-0.5 font-mono text-[11px] font-semibold text-cyan-800 dark:bg-cyan-500/20 dark:text-cyan-300">
                    {t.agentPayIntro.marketplaceRoleBadge}
                  </span>
                  <h3 className="text-base font-bold text-slate-900 dark:text-white">
                    {t.agentPayIntro.marketplaceRoleTitle}
                  </h3>
                </div>
                <p className="text-sm leading-relaxed text-slate-700 dark:text-slate-300">
                  {t.agentPayIntro.marketplaceRoleDesc}
                </p>
              </div>
            </div>
          </div>

          {/* Three Core Pillars */}
          <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
            <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-indigo-400 dark:border-slate-800 dark:bg-slate-950/70 dark:hover:border-indigo-500/40">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600 dark:bg-indigo-500/15 dark:text-indigo-400">
                <ShieldCheck className="h-5 w-5" />
              </div>
              <h4 className="mt-3.5 text-sm font-bold text-slate-900 dark:text-white">
                {t.agentPayIntro.pillar1Title}
              </h4>
              <p className="mt-2 text-xs leading-relaxed text-slate-600 dark:text-slate-400">
                {t.agentPayIntro.pillar1Desc}
              </p>
            </div>

            <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-emerald-400 dark:border-slate-800 dark:bg-slate-950/70 dark:hover:border-emerald-500/40">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400">
                <Zap className="h-5 w-5" />
              </div>
              <h4 className="mt-3.5 text-sm font-bold text-slate-900 dark:text-white">
                {t.agentPayIntro.pillar2Title}
              </h4>
              <p className="mt-2 text-xs leading-relaxed text-slate-600 dark:text-slate-400">
                {t.agentPayIntro.pillar2Desc}
              </p>
            </div>

            <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-cyan-400 dark:border-slate-800 dark:bg-slate-950/70 dark:hover:border-cyan-500/40">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-cyan-50 text-cyan-600 dark:bg-cyan-500/15 dark:text-cyan-400">
                <Terminal className="h-5 w-5" />
              </div>
              <h4 className="mt-3.5 text-sm font-bold text-slate-900 dark:text-white">
                {t.agentPayIntro.pillar3Title}
              </h4>
              <p className="mt-2 text-xs leading-relaxed text-slate-600 dark:text-slate-400">
                {t.agentPayIntro.pillar3Desc}
              </p>
            </div>
          </div>

          {/* Quick SDK Verification Code Card (Kept dark for crystal-clear syntax) */}
          <div className="rounded-2xl border border-slate-200 bg-slate-900 p-5 text-white dark:border-slate-800 dark:bg-slate-950/80">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 pb-3">
              <div>
                <span className="text-xs font-bold text-slate-100">
                  {t.agentPayIntro.codeSnippetTitle}
                </span>
                <p className="mt-0.5 text-xs text-slate-400">
                  {t.agentPayIntro.codeSnippetDesc}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <span className="font-mono text-xs text-indigo-300">pip install open-agentpay</span>
                <CopyButton text={sampleQuickVerifySnippet} label={t.preview.copyCode} />
              </div>
            </div>
            <pre className="mt-3.5 overflow-x-auto rounded-xl border border-slate-800/80 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-slate-200">
              <code>{sampleQuickVerifySnippet}</code>
            </pre>
          </div>
        </div>
      </section>

      {/* ===================================================================== */}
      {/* Section Header for [API 功能商店] */}
      {/* ===================================================================== */}
      <div className="border-t border-slate-200 pt-8 dark:border-slate-800/80">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="inline-flex items-center gap-2 rounded-full border border-indigo-500/30 bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-300">
              <Layers className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
              <span>{t.marketplaceHeader.badge}</span>
            </div>
            <h2 className="mt-2 text-2xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-3xl">
              {t.marketplaceHeader.title}
            </h2>
            <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">
              {t.marketplaceHeader.subtitle}
            </p>
          </div>

          <span className="font-mono text-xs text-slate-500 dark:text-slate-400">
            {filteredRoutes.length} {t.hero.availableRoutes}
          </span>
        </div>
      </div>

      {/* Filter Bar (Categories & Network Selector) */}
      <section className="flex flex-col justify-between gap-4 border-b border-slate-200 pb-5 dark:border-slate-800/80 lg:flex-row lg:items-center">
        <div className="flex flex-wrap items-center gap-2">
          {MARKETPLACE_CATEGORIES.map((cat) => {
            const active = selectedCategory === cat.id;
            const count =
              cat.id === 'all'
                ? initialRoutes.length
                : initialRoutes.filter((r) => r.category.toLowerCase() === cat.id.toLowerCase())
                    .length;
            const label = t.filter.categories[cat.id] || cat.label;
            return (
              <button
                key={cat.id}
                type="button"
                onClick={() => setSelectedCategory(cat.id)}
                className={`inline-flex items-center gap-2 rounded-xl px-3.5 py-2 text-xs font-medium transition-all sm:text-sm ${
                  active
                    ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/20'
                    : 'border border-slate-200 bg-white text-slate-700 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:bg-slate-900 dark:hover:text-white'
                }`}
              >
                <span>{label}</span>
                <span
                  className={`rounded-full px-1.5 py-0.2 font-mono text-[11px] ${
                    active
                      ? 'bg-indigo-500/40 text-indigo-100'
                      : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'
                  }`}
                >
                  {count}
                </span>
              </button>
            );
          })}
        </div>

        <div className="flex items-center gap-2.5 self-start lg:self-auto">
          <span className="inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400">
            <Filter className="h-3.5 w-3.5" />
            {t.filter.settlementNetwork}
          </span>
          {[
            { id: 'all', label: t.filter.allNetworks },
            { id: 'base', label: 'Base USDC' },
            { id: 'arbitrum', label: 'Arbitrum USDC' },
          ].map((net) => (
            <button
              key={net.id}
              type="button"
              onClick={() => setSelectedNetwork(net.id)}
              className={`rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors ${
                selectedNetwork === net.id
                  ? 'border-cyan-500 bg-cyan-50 text-cyan-700 dark:border-cyan-500/50 dark:bg-cyan-500/15 dark:text-cyan-300'
                  : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400 dark:hover:border-slate-700 dark:hover:text-slate-200'
              }`}
            >
              {net.label}
            </button>
          ))}
        </div>
      </section>

      {/* Quick SDK Code Modal / Drawer Banner if a route is selected for quick preview */}
      {previewRoute && (
        <section className="rounded-2xl border border-indigo-500/40 bg-slate-900 p-6 shadow-2xl text-white">
          <div className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-4">
            <div>
              <div className="flex items-center gap-2">
                <span className="rounded-md bg-indigo-500/20 px-2 py-0.5 font-mono text-xs text-indigo-300">
                  {t.preview.sdkTitle}
                </span>
                <h3 className="text-base font-bold text-white">{previewRoute.title}</h3>
              </div>
              <p className="mt-1 text-xs text-slate-400">
                {t.preview.sdkSubtitle.replace('{slug}', previewRoute.slug)}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <CopyButton text={buildPythonSdkSnippet(previewRoute)} label={t.preview.copyCode} />
              <Link
                href={`/hub/${previewRoute.slug}`}
                className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-indigo-500"
              >
                <span>{t.preview.viewDetails}</span>
                <ArrowUpRight className="h-3.5 w-3.5" />
              </Link>
              <button
                type="button"
                onClick={() => setActiveQuickPreviewSlug(null)}
                className="rounded-lg border border-slate-700 px-2.5 py-1.5 text-xs text-slate-400 hover:bg-slate-800 hover:text-white"
              >
                {t.preview.close}
              </button>
            </div>
          </div>
          <pre className="mt-4 overflow-x-auto rounded-xl border border-slate-800 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-slate-200">
            <code>{buildPythonSdkSnippet(previewRoute)}</code>
          </pre>
        </section>
      )}

      {/* Marketplace Route Cards Grid */}
      {filteredRoutes.length === 0 ? (
        <div className="rounded-3xl border border-slate-200 bg-white p-12 text-center shadow-sm dark:border-slate-800/80 dark:bg-slate-900/40">
          <p className="text-base font-medium text-slate-700 dark:text-slate-300">
            {t.empty.noMatchTitle.replace('{query}', searchQuery)}
          </p>
          <p className="mt-2 text-sm text-slate-500">
            {t.empty.noMatchDesc}
          </p>
          <button
            type="button"
            onClick={() => {
              setSearchQuery('');
              setSelectedCategory('all');
              setSelectedNetwork('all');
            }}
            className="mt-5 rounded-xl bg-indigo-600 px-4 py-2 text-xs font-medium text-white hover:bg-indigo-500"
          >
            {t.empty.resetBtn}
          </button>
        </div>
      ) : (
        <section className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
          {filteredRoutes.map((route) => {
            const priceUsdc = formatUsdcPrice(
              route.onchain_price_base_units || route.price_base_units
            );
            const netLabel = formatNetworkLabel(route.network, route.network_label);
            const isHealthy = route.sla?.health_status === 'healthy';
            const isDegraded = route.sla?.health_status === 'degraded';
            const categoryLabel = t.filter.categories[route.category] || route.category;

            return (
              <article
                key={route.slug}
                className="group relative flex flex-col justify-between rounded-2xl border border-slate-200 bg-white p-6 shadow-sm transition-all duration-200 hover:-translate-y-1 hover:border-indigo-400 hover:shadow-xl dark:border-slate-800/90 dark:bg-slate-900/60 dark:hover:border-indigo-500/50 dark:hover:bg-slate-900/90 dark:hover:shadow-2xl dark:hover:shadow-indigo-950/40"
              >
                <div>
                  {/* Top Row: Category + Gateway Health Status */}
                  <div className="flex items-center justify-between gap-2">
                    <span className="inline-flex items-center rounded-lg border border-slate-200 bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700 dark:border-slate-700/80 dark:bg-slate-800/70 dark:text-slate-300">
                      {categoryLabel}
                    </span>

                    <span
                      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium ${
                        isHealthy
                          ? 'border-emerald-500/30 bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300'
                          : isDegraded
                            ? 'border-amber-500/30 bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300'
                            : 'border-rose-500/30 bg-rose-50 text-rose-700 dark:bg-rose-500/10 dark:text-rose-300'
                      }`}
                    >
                      <span
                        className={`h-1.5 w-1.5 rounded-full ${
                          isHealthy
                            ? 'bg-emerald-500 dark:bg-emerald-400 animate-pulse'
                            : isDegraded
                              ? 'bg-amber-500 dark:bg-amber-400'
                              : 'bg-rose-500 dark:bg-rose-400'
                        }`}
                      />
                      {isHealthy
                        ? t.routeCard.verified
                        : isDegraded
                          ? t.routeCard.degraded
                          : t.routeCard.maintenance}
                    </span>
                  </div>

                  {/* Title & Semantic Slug */}
                  <div className="mt-4">
                    <Link
                      href={`/hub/${route.slug}`}
                      className="block text-lg font-bold text-slate-900 transition-colors group-hover:text-indigo-600 dark:text-white dark:group-hover:text-indigo-300"
                    >
                      {route.title}
                    </Link>
                    <div className="mt-1.5 flex items-center gap-2 font-mono text-xs text-slate-500 dark:text-slate-400">
                      <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] font-semibold text-cyan-700 dark:bg-slate-800 dark:text-cyan-300">
                        {(route.methods || ['GET', 'POST']).join('/')}
                      </span>
                      <span className="truncate">{route.path_pattern}</span>
                    </div>
                  </div>

                  {/* Description Summary */}
                  <p className="mt-3.5 line-clamp-3 text-sm leading-relaxed text-slate-600 dark:text-slate-300">
                    {route.summary || route.description_md}
                  </p>

                  {/* Tags */}
                  <div className="mt-4 flex flex-wrap gap-1.5">
                    {route.tags.slice(0, 4).map((tag) => (
                      <span
                        key={tag}
                        className="rounded-md border border-slate-200 bg-slate-100/70 px-2 py-0.5 font-mono text-[11px] text-slate-600 dark:border-slate-800 dark:bg-slate-950/60 dark:text-slate-400"
                      >
                        #{tag}
                      </span>
                    ))}
                  </div>
                </div>

                <div className="mt-6 space-y-4 border-t border-slate-200 pt-4 dark:border-slate-800/80">
                  {/* Gateway Live SLA Endorsement Box */}
                  <div className="grid grid-cols-3 gap-2 rounded-xl border border-slate-200 bg-slate-50 p-2.5 text-center dark:border-slate-800/80 dark:bg-slate-950/70">
                    <div>
                      <div className="flex items-center justify-center gap-1 text-[11px] text-slate-500 dark:text-slate-400">
                        <CheckCircle2 className="h-3 w-3 text-emerald-600 dark:text-emerald-400" />
                        <span>{t.routeCard.successRate}</span>
                      </div>
                      <div className="mt-0.5 font-mono text-xs font-bold text-emerald-600 dark:text-emerald-400">
                        {route.sla.success_rate_24h}%
                      </div>
                    </div>

                    <div className="border-x border-slate-200 dark:border-slate-800/80">
                      <div className="flex items-center justify-center gap-1 text-[11px] text-slate-500 dark:text-slate-400">
                        <Clock className="h-3 w-3 text-cyan-600 dark:text-cyan-400" />
                        <span>{t.routeCard.p95Latency}</span>
                      </div>
                      <div className="mt-0.5 font-mono text-xs font-bold text-slate-900 dark:text-white">
                        {route.sla.p95_latency_ms} ms
                      </div>
                    </div>

                    <div>
                      <div className="flex items-center justify-center gap-1 text-[11px] text-slate-500 dark:text-slate-400">
                        <Activity className="h-3 w-3 text-indigo-600 dark:text-indigo-400" />
                        <span>{t.routeCard.paidCalls}</span>
                      </div>
                      <div className="mt-0.5 font-mono text-xs font-bold text-slate-800 dark:text-slate-200">
                        {route.sla.paid_calls_24h.toLocaleString()}
                      </div>
                    </div>
                  </div>

                  {/* Price & Action Buttons */}
                  <div className="flex items-center justify-between gap-2">
                    <div>
                      <div className="text-[11px] text-slate-500 dark:text-slate-400">{netLabel}</div>
                      <div className="flex items-baseline gap-1">
                        <span className="font-mono text-lg font-extrabold text-slate-900 dark:text-white">
                          {priceUsdc}
                        </span>
                        <span className="font-mono text-xs font-semibold text-indigo-600 dark:text-indigo-400">
                          {t.routeCard.perReq}
                        </span>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() =>
                          setActiveQuickPreviewSlug(
                            activeQuickPreviewSlug === route.slug ? null : route.slug
                          )
                        }
                        className="inline-flex items-center gap-1 rounded-xl border border-slate-300 bg-slate-100 px-3 py-2 text-xs font-medium text-slate-700 transition-colors hover:border-indigo-400 hover:text-indigo-700 dark:border-slate-700/80 dark:bg-slate-800/80 dark:text-slate-200 dark:hover:border-indigo-500/50 dark:hover:text-white"
                        title="Python SDK"
                      >
                        <Code2 className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
                        <span>{t.routeCard.sdkCode}</span>
                      </button>

                      <Link
                        href={`/hub/${route.slug}`}
                        className="inline-flex items-center gap-1 rounded-xl bg-indigo-600 px-3.5 py-2 text-xs font-semibold text-white shadow-md shadow-indigo-600/20 transition-all hover:bg-indigo-500"
                      >
                        <span>{t.routeCard.details}</span>
                        <ArrowUpRight className="h-3.5 w-3.5" />
                      </Link>
                    </div>
                  </div>
                </div>
              </article>
            );
          })}
        </section>
      )}

      {/* Dual-Layer Architecture & GEO Value Section */}
      <section className="grid grid-cols-1 gap-6 rounded-3xl border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800/90 dark:bg-slate-900/40 lg:grid-cols-3">
        <div className="space-y-3 rounded-2xl border border-slate-200 bg-slate-50/70 p-6 dark:border-slate-800/80 dark:bg-slate-950/60">
          <div className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600 dark:bg-indigo-500/15 dark:text-indigo-400">
            <Terminal className="h-5 w-5" />
          </div>
          <h3 className="text-base font-bold text-slate-900 dark:text-white">
            {t.architecture.box1Title}
          </h3>
          <p className="text-sm leading-relaxed text-slate-600 dark:text-slate-400">
            {t.architecture.box1Desc}
          </p>
        </div>

        <div className="space-y-3 rounded-2xl border border-slate-200 bg-slate-50/70 p-6 dark:border-slate-800/80 dark:bg-slate-950/60">
          <div className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400">
            <ShieldCheck className="h-5 w-5" />
          </div>
          <h3 className="text-base font-bold text-slate-900 dark:text-white">
            {t.architecture.box2Title}
          </h3>
          <p className="text-sm leading-relaxed text-slate-600 dark:text-slate-400">
            {t.architecture.box2Desc}
          </p>
        </div>

        <div className="space-y-3 rounded-2xl border border-slate-200 bg-slate-50/70 p-6 dark:border-slate-800/80 dark:bg-slate-950/60">
          <div className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-50 text-cyan-600 dark:bg-cyan-500/15 dark:text-cyan-400">
            <Globe className="h-5 w-5" />
          </div>
          <h3 className="text-base font-bold text-slate-900 dark:text-white">
            {t.architecture.box3Title}
          </h3>
          <p className="text-sm leading-relaxed text-slate-600 dark:text-slate-400">
            {t.architecture.box3Desc}
          </p>
        </div>
      </section>

      {/* Seller Onboarding CTA */}
      <section className="flex flex-col items-start justify-between gap-6 rounded-3xl border border-indigo-200 bg-gradient-to-r from-indigo-50/90 via-slate-50 to-white p-8 shadow-sm dark:border-indigo-500/30 dark:bg-gradient-to-r dark:from-indigo-950/60 dark:via-slate-900 dark:to-slate-900 sm:flex-row sm:items-center">
        <div>
          <span className="rounded-full bg-indigo-100 px-3 py-1 text-xs font-semibold text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-300">
            {t.sellerCta.badge}
          </span>
          <h2 className="mt-3 text-xl font-bold text-slate-900 dark:text-white sm:text-2xl">
            {t.sellerCta.heading}
          </h2>
          <p className="mt-1.5 max-w-2xl text-sm text-slate-600 dark:text-slate-300">
            {t.sellerCta.description}
          </p>
        </div>

        <div className="flex shrink-0 flex-wrap items-center gap-3">
          <a
            href={PUBLIC_CONSOLE_URL}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white shadow-md hover:bg-indigo-500"
          >
            <span>{t.sellerCta.buttonConsole}</span>
            <ExternalLink className="h-4 w-4" />
          </a>
          <Link
            href="/"
            className="inline-flex items-center gap-1.5 rounded-xl border border-slate-300 bg-white px-4 py-2.5 text-sm font-medium text-slate-700 hover:border-slate-400 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-900/90 dark:text-slate-200 dark:hover:border-slate-600 dark:hover:text-white"
          >
            <ShieldCheck className="h-4 w-4 text-indigo-600 dark:text-indigo-400" />
            <span>{locale === 'zh' ? '了解 AgentPay 核心架构' : 'AgentPay Architecture'}</span>
          </Link>
        </div>
      </section>
    </div>
  );
}
