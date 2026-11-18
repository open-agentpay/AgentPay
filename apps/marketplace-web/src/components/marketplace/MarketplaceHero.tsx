'use client';

import React from 'react';
import { Search, Shield, Sparkles } from 'lucide-react';
import { useI18n } from '@/i18n/context';

interface MarketplaceHeroProps {
  searchQuery: string;
  onSearchChange: (val: string) => void;
}

export function MarketplaceHero({
  searchQuery,
  onSearchChange,
}: MarketplaceHeroProps) {
  const { t } = useI18n();

  return (
    <section className="relative overflow-hidden rounded-3xl border border-slate-200 bg-white/80 p-8 shadow-xl transition-colors duration-200 dark:border-slate-800/90 dark:bg-gradient-to-b dark:from-slate-900/90 dark:via-[#0B1120] dark:to-[#080D1A] sm:p-12">
      <div className="pointer-events-none absolute -top-24 left-1/2 h-72 w-[680px] -translate-x-1/2 rounded-full bg-indigo-500/10 blur-3xl dark:bg-indigo-500/15" />
      <div className="pointer-events-none absolute -bottom-24 right-10 h-60 w-80 rounded-full bg-emerald-500/10 blur-3xl" />

      <div className="relative z-10 mx-auto max-w-4xl text-center">
        {/* Top Badges */}
        <div className="mb-5 inline-flex flex-wrap items-center justify-center gap-2 rounded-full border border-indigo-500/30 bg-indigo-500/10 px-4 py-1.5 text-xs font-medium text-indigo-700 dark:text-indigo-300">
          <Sparkles className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
          <span>{t.hero.badgeEconomy}</span>
          <span className="text-slate-400 dark:text-slate-600">•</span>
          <span className="text-emerald-700 dark:text-emerald-300">{t.hero.badgeGas}</span>
          <span className="text-slate-400 dark:text-slate-600">•</span>
          <span className="inline-flex items-center gap-1 text-slate-600 dark:text-slate-300">
            <Shield className="h-3 w-3 text-indigo-600 dark:text-indigo-400" />
            <span>{t.heroForAgents}</span>
          </span>
        </div>

        {/* Main Title */}
        <h1 className="text-3xl font-extrabold tracking-tight text-slate-900 sm:text-5xl sm:leading-[1.15] dark:text-white [text-wrap:balance]">
          {t.hero.titlePre}{' '}
          <span className="bg-gradient-to-r from-indigo-600 via-cyan-600 to-emerald-600 bg-clip-text text-transparent dark:from-indigo-400 dark:via-cyan-300 dark:to-emerald-400">
            {t.hero.titleHighlight}
          </span>
        </h1>

        {/* Subtitle Description */}
        <p className="mx-auto mt-4 max-w-2xl text-base leading-relaxed text-slate-600 sm:text-lg dark:text-slate-300 [text-wrap:pretty]">
          {t.hero.description}
        </p>

        {/* Search Input Box */}
        <div className="mx-auto mt-8 max-w-2xl">
          <div className="group relative flex items-center rounded-2xl border border-slate-300 bg-white p-2 shadow-sm transition-all focus-within:border-indigo-500 focus-within:ring-2 focus-within:ring-indigo-500/20 dark:border-slate-700/80 dark:bg-slate-950/90 dark:shadow-inner">
            <Search className="ml-3.5 h-5 w-5 shrink-0 text-slate-400 group-focus-within:text-indigo-600 dark:group-focus-within:text-indigo-400" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => onSearchChange(e.target.value)}
              placeholder={t.hero.searchPlaceholder}
              className="w-full bg-transparent px-3 py-1.5 text-sm text-slate-900 placeholder-slate-400 focus:outline-none dark:text-white"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => onSearchChange('')}
                className="mr-2 rounded-lg px-2 py-1 text-xs text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                {t.heroClear || t.hero.clear}
              </button>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
