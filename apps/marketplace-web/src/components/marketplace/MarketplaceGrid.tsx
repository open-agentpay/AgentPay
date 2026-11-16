'use client';

import React from 'react';
import type { MarketplaceRouteItem } from '@/types/marketplace';
import { MarketplaceRouteCard } from './MarketplaceRouteCard';
import { useI18n } from '@/i18n/context';

interface MarketplaceGridProps {
  routes: MarketplaceRouteItem[];
  searchQuery: string;
  onResetFilters: () => void;
  onOpenQuickSdk: (slug: string) => void;
}

export function MarketplaceGrid({
  routes,
  searchQuery,
  onResetFilters,
  onOpenQuickSdk,
}: MarketplaceGridProps) {
  const { t } = useI18n();

  if (routes.length === 0) {
    return (
      <div className="rounded-3xl border border-slate-200 bg-white p-12 text-center shadow-sm dark:border-slate-800/80 dark:bg-slate-900/40">
        <p className="text-base font-medium text-slate-700 dark:text-slate-300">
          {t.empty.noMatchTitle.replace('{query}', searchQuery)}
        </p>
        <p className="mt-2 text-sm text-slate-500">
          {t.empty.noMatchDesc}
        </p>
        <button
          type="button"
          onClick={onResetFilters}
          className="mt-5 rounded-xl bg-indigo-600 px-4 py-2 text-xs font-medium text-white hover:bg-indigo-500"
        >
          {t.empty.resetBtn}
        </button>
      </div>
    );
  }

  return (
    <section className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
      {routes.map((route) => (
        <MarketplaceRouteCard
          key={route.slug}
          route={route}
          onOpenQuickSdk={onOpenQuickSdk}
        />
      ))}
    </section>
  );
}
