'use client';

import React from 'react';
import Link from 'next/link';
import {
  Activity,
  ArrowUpRight,
  BrainCircuit,
  CheckCircle2,
  Clock,
  Code2,
  Sparkles,
  Zap,
} from 'lucide-react';
import {
  formatCreditsPrice,
  formatTokenPriceFen,
  getLlmModelName,
  isLlmRoute,
} from '@/lib/marketplace';
import type { MarketplaceRouteItem } from '@/types/marketplace';
import { useI18n } from '@/i18n/context';

interface MarketplaceRouteCardProps {
  route: MarketplaceRouteItem;
  onOpenQuickSdk: (slug: string) => void;
}

export function MarketplaceRouteCard({
  route,
  onOpenQuickSdk,
}: MarketplaceRouteCardProps) {
  const { t } = useI18n();

  const isLlm = isLlmRoute(route);
  const llmModel = getLlmModelName(route);
  const promptFen = formatTokenPriceFen(route.prompt_token_price_base_units ?? 1000);
  const completionFen = formatTokenPriceFen(route.completion_token_price_base_units ?? 2000);
  const priceCredits = formatCreditsPrice(route.price_base_units);
  const clusterLabel = route.cluster || 'domestic-v1';
  const isHealthy = route.sla?.health_status === 'healthy';
  const isDegraded = route.sla?.health_status === 'degraded';
  const categoryLabel = t.filter.categories[route.category] || route.category;

  return (
    <article className="group relative flex flex-col justify-between rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-all duration-200 hover:-translate-y-1 hover:border-indigo-400 hover:shadow-xl dark:border-slate-800/90 dark:bg-slate-900/60 dark:hover:border-indigo-500/50 dark:hover:bg-slate-900/90 dark:hover:shadow-2xl dark:hover:shadow-indigo-950/40">
      <div>
        {/* 1. Top Badges & Status */}
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5 flex-wrap">
            {isLlm && (
              <span className="inline-flex items-center gap-1 rounded-lg border border-purple-500/40 bg-purple-500/10 px-2 py-0.5 text-xs font-bold text-purple-700 dark:text-purple-300">
                <BrainCircuit className="h-3 w-3 text-purple-600 dark:text-purple-400" />
                <span>{t.card.llmBadge}</span>
              </span>
            )}
            <span className="inline-flex items-center rounded-lg border border-slate-200 bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-700 dark:border-slate-700/80 dark:bg-slate-800/70 dark:text-slate-300">
              {categoryLabel}
            </span>
          </div>

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

        {/* 2. Title & Semantic Path */}
        <div className="mt-3.5">
          <Link
            href={`/hub/${route.slug}`}
            className="block text-lg font-bold text-slate-900 transition-colors group-hover:text-indigo-600 dark:text-white dark:group-hover:text-indigo-300 [text-wrap:balance]"
          >
            {route.title}
          </Link>

          <div className="mt-1.5 flex items-center gap-2 font-mono text-xs text-slate-500 dark:text-slate-400 flex-wrap">
            <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] font-semibold text-cyan-700 dark:bg-slate-800 dark:text-cyan-300">
              {(route.methods || ['GET', 'POST']).join('/')}
            </span>
            <span className="truncate">{route.path_pattern}</span>
            {isLlm && (
              <span className="rounded bg-purple-100 px-1.5 py-0.5 text-[11px] font-semibold text-purple-800 dark:bg-purple-950/70 dark:text-purple-300 border border-purple-200/60 dark:border-purple-800/40">
                model: {llmModel}
              </span>
            )}
          </div>
        </div>

        {/* 3. Description Summary */}
        <p className="mt-3 line-clamp-2 text-sm leading-relaxed text-slate-600 dark:text-slate-300 [text-wrap:pretty]">
          {route.summary || route.description_md}
        </p>

        {/* 4. Tags */}
        <div className="mt-3 flex flex-wrap gap-1.5">
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

      {/* 5. Card Footer: SLA Grid + Protected Price Box + Two-Column Action Buttons */}
      <div className="mt-5 space-y-3 border-t border-slate-200 pt-3.5 dark:border-slate-800/80">
        {/* SLA Endorsement */}
        <div className="grid grid-cols-3 gap-2 rounded-xl border border-slate-200 bg-slate-50 p-2 text-center dark:border-slate-800/80 dark:bg-slate-950/70">
          <div>
            <div className="flex items-center justify-center gap-1 text-[11px] text-slate-500 dark:text-slate-400">
              <CheckCircle2 className="h-3 w-3 text-emerald-600 dark:text-emerald-400" />
              <span>{t.routeCard.successRate}</span>
            </div>
            <div className="mt-0.5 font-mono text-xs font-bold text-emerald-600 dark:text-emerald-400 [font-variant-numeric:tabular-nums]">
              {route.sla.success_rate_24h}%
            </div>
          </div>

          <div className="border-x border-slate-200 dark:border-slate-800/80">
            <div className="flex items-center justify-center gap-1 text-[11px] text-slate-500 dark:text-slate-400">
              <Clock className="h-3 w-3 text-cyan-600 dark:text-cyan-400" />
              <span>{t.routeCard.p95Latency}</span>
            </div>
            <div className="mt-0.5 font-mono text-xs font-bold text-slate-900 dark:text-white [font-variant-numeric:tabular-nums]">
              {route.sla.p95_latency_ms} ms
            </div>
          </div>

          <div>
            <div className="flex items-center justify-center gap-1 text-[11px] text-slate-500 dark:text-slate-400">
              <Activity className="h-3 w-3 text-indigo-600 dark:text-indigo-400" />
              <span>{t.routeCard.paidCalls}</span>
            </div>
            <div className="mt-0.5 font-mono text-xs font-bold text-slate-800 dark:text-slate-200 [font-variant-numeric:tabular-nums]">
              {route.sla.paid_calls_24h.toLocaleString()}
            </div>
          </div>
        </div>

        {/* Dedicated Pricing Strip (Guaranteed single line, no wrapping, precision alignment) */}
        <div className="flex items-center justify-between rounded-xl border border-slate-200/80 bg-slate-50/80 px-3 py-2 dark:border-slate-800 dark:bg-slate-950/60">
          {isLlm ? (
            <>
              <div className="flex items-center gap-1 text-[11px] font-semibold text-purple-700 dark:text-purple-300 whitespace-nowrap">
                <Sparkles className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400 shrink-0" />
                <span>{t.card.tokenStreaming}</span>
              </div>
              <div className="flex items-baseline gap-1 font-mono text-xs whitespace-nowrap shrink-0 [font-variant-numeric:tabular-nums]">
                <div className="flex items-baseline gap-1 font-bold text-slate-900 dark:text-white whitespace-nowrap">
                  <span className="text-[10px] text-slate-400 font-normal">{t.card.promptShort}:</span>
                  <span>{promptFen}</span>
                  <span className="text-[10px] text-slate-400 font-normal ml-0.5">{t.card.completionShort}:</span>
                  <span>{completionFen}</span>
                </div>
                <span className="text-xs font-bold text-purple-600 dark:text-purple-400 whitespace-nowrap shrink-0">
                  {t.card.fenPer1k}
                </span>
              </div>
            </>
          ) : (
            <>
              <div className="flex items-center gap-1 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <Zap className="h-3.5 w-3.5 text-amber-500 shrink-0" />
                <span className="truncate max-w-[120px]">{clusterLabel}</span>
              </div>
              <div className="flex items-baseline gap-1 font-mono whitespace-nowrap shrink-0 [font-variant-numeric:tabular-nums]">
                <span className="text-base font-extrabold text-slate-900 dark:text-white">
                  {priceCredits}
                </span>
                <span className="text-xs font-semibold text-indigo-600 dark:text-indigo-400 whitespace-nowrap shrink-0">
                  {t.routeCard.perReq}
                </span>
              </div>
            </>
          )}
        </div>

        {/* Action Buttons: 2-column balanced grid layout, full-width, zero text wrapping */}
        <div className="grid grid-cols-2 gap-2">
          <button
            type="button"
            onClick={() => onOpenQuickSdk(route.slug)}
            className="inline-flex w-full items-center justify-center gap-1.5 rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs font-medium text-slate-700 transition-colors hover:border-indigo-400 hover:bg-white hover:text-indigo-700 dark:border-slate-700/80 dark:bg-slate-800/80 dark:text-slate-200 dark:hover:border-indigo-500/50 dark:hover:bg-slate-800 dark:hover:text-white whitespace-nowrap"
            title={isLlm ? t.card.viewOpenAiCode : t.card.viewSdkCode}
          >
            <Code2
              className={`h-3.5 w-3.5 shrink-0 ${
                isLlm
                  ? 'text-purple-600 dark:text-purple-400'
                  : 'text-indigo-600 dark:text-indigo-400'
              }`}
            />
            <span className="truncate">{isLlm ? 'OpenAI SDK' : t.routeCard.sdkCode}</span>
          </button>

          <Link
            href={`/hub/${route.slug}`}
            className={`inline-flex w-full items-center justify-center gap-1 rounded-xl px-3 py-2 text-xs font-semibold text-white shadow-sm transition-all whitespace-nowrap ${
              isLlm
                ? 'bg-gradient-to-r from-purple-600 to-indigo-600 hover:from-purple-500 hover:to-indigo-500 shadow-purple-600/20'
                : 'bg-indigo-600 hover:bg-indigo-500 shadow-indigo-600/20'
            }`}
          >
            <span>{t.routeCard.details}</span>
            <ArrowUpRight className="h-3.5 w-3.5 shrink-0" />
          </Link>
        </div>
      </div>
    </article>
  );
}
