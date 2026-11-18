'use client';

import React from 'react';
import Link from 'next/link';
import { ExternalLink, FileText, Globe, Terminal } from 'lucide-react';
import { PUBLIC_CONSOLE_URL } from '@/lib/marketplace';
import { useI18n } from '@/i18n/context';

export function MarketplaceValueSection() {
  const { t } = useI18n();

  return (
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
        <div className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-50 text-cyan-600 dark:bg-cyan-500/15 dark:text-cyan-400">
          <Globe className="h-5 w-5" />
        </div>
        <h3 className="text-base font-bold text-slate-900 dark:text-white">
          {t.architecture.box2Title}
        </h3>
        <p className="text-sm leading-relaxed text-slate-600 dark:text-slate-400">
          {t.architecture.box2Desc}
        </p>
      </div>

      <div className="space-y-3 rounded-2xl border border-slate-200 bg-slate-50/70 p-6 dark:border-slate-800/80 dark:bg-slate-950/60">
        <div className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400">
          <FileText className="h-5 w-5" />
        </div>
        <h3 className="text-base font-bold text-slate-900 dark:text-white">
          {t.architecture.box3Title}
        </h3>
        <p className="text-sm leading-relaxed text-slate-600 dark:text-slate-400">
          {t.architecture.box3Desc}
        </p>
        <div className="pt-2 flex items-center gap-3">
          <Link
            href="/llms.txt"
            target="_blank"
            className="inline-flex items-center gap-1 font-mono text-xs text-indigo-600 hover:underline dark:text-indigo-400"
          >
            <span>/llms.txt</span>
            <ExternalLink className="h-3 w-3" />
          </Link>
          <span className="text-slate-300 dark:text-slate-700">|</span>
          <a
            href={PUBLIC_CONSOLE_URL}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 text-xs text-indigo-600 hover:underline dark:text-indigo-400 font-medium"
          >
            <span>{t.publishApi}</span>
            <ExternalLink className="h-3 w-3" />
          </a>
        </div>
      </div>
    </section>
  );
}
