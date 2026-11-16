'use client';

import React from 'react';
import { Filter, Layers } from 'lucide-react';
import { MARKETPLACE_CATEGORIES } from '@/lib/seed-data';
import type { MarketplaceRouteItem } from '@/types/marketplace';
import { useI18n } from '@/i18n/context';

interface MarketplaceFilterBarProps {
  initialRoutes: MarketplaceRouteItem[];
  filteredCount: number;
  selectedCategory: string;
  selectedCluster: string;
  onSelectCategory: (catId: string) => void;
  onSelectCluster: (clusterId: string) => void;
}

export function MarketplaceFilterBar({
  initialRoutes,
  filteredCount,
  selectedCategory,
  selectedCluster,
  onSelectCategory,
  onSelectCluster,
}: MarketplaceFilterBarProps) {
  const { t } = useI18n();

  return (
    <div className="space-y-6">
      {/* Section Header */}
      <div className="border-t border-slate-200 pt-8 dark:border-slate-800/80">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="inline-flex items-center gap-2 rounded-full border border-indigo-500/30 bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-300">
              <Layers className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
              <span>{t.marketplaceHeader.badge}</span>
            </div>
            <h2 className="mt-2 text-2xl font-extrabold tracking-tight text-slate-900 sm:text-3xl dark:text-white [text-wrap:balance]">
              {t.marketplaceHeader.title}
            </h2>
            <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">
              {t.marketplaceHeader.subtitle}
            </p>
          </div>

          <span className="font-mono text-xs text-slate-500 dark:text-slate-400 [font-variant-numeric:tabular-nums]">
            {filteredCount} {t.hero.availableRoutes}
          </span>
        </div>
      </div>

      {/* Filter Tabs */}
      <section className="flex flex-col justify-between gap-4 border-b border-slate-200 pb-5 dark:border-slate-800/80 lg:flex-row lg:items-center">
        <div className="flex flex-wrap items-center gap-2">
          {MARKETPLACE_CATEGORIES.map((cat) => {
            const active = selectedCategory === cat.id;
            const count =
              cat.id === 'all'
                ? initialRoutes.length
                : initialRoutes.filter(
                    (r) => r.category.toLowerCase() === cat.id.toLowerCase()
                  ).length;
            const label = t.filter.categories[cat.id] || cat.label;
            return (
              <button
                key={cat.id}
                type="button"
                onClick={() => onSelectCategory(cat.id)}
                className={`inline-flex items-center gap-2 rounded-xl px-3.5 py-2 text-xs font-medium transition-all sm:text-sm whitespace-nowrap ${
                  active
                    ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/20'
                    : 'border border-slate-200 bg-white text-slate-700 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:bg-slate-900 dark:hover:text-white'
                }`}
              >
                <span>{label}</span>
                <span
                  className={`rounded-full px-1.5 py-0.2 font-mono text-[11px] [font-variant-numeric:tabular-nums] ${
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
          <span className="inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 whitespace-nowrap">
            <Filter className="h-3.5 w-3.5" />
            {t.filter.settlementNetwork}
          </span>
          {[
            { id: 'all', label: t.filter.allNetworks },
            { id: 'domestic-v1', label: '华东专线集群' },
          ].map((cluster) => (
            <button
              key={cluster.id}
              type="button"
              onClick={() => onSelectCluster(cluster.id)}
              className={`rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors whitespace-nowrap ${
                selectedCluster === cluster.id
                  ? 'border-cyan-500 bg-cyan-50 text-cyan-700 dark:border-cyan-500/50 dark:bg-cyan-500/15 dark:text-cyan-300'
                  : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400 dark:hover:border-slate-700 dark:hover:text-slate-200'
              }`}
            >
              {cluster.label}
            </button>
          ))}
        </div>
      </section>
    </div>
  );
}
