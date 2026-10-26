'use client';

import Link from 'next/link';
import { ArrowLeft, SearchX } from 'lucide-react';
import { useI18n } from '@/i18n/context';

export default function NotFound() {
  const { t } = useI18n();

  return (
    <div className="mx-auto my-16 max-w-lg rounded-3xl border border-slate-200 bg-white p-10 text-center shadow-lg dark:border-slate-800 dark:bg-slate-900/60 transition-colors duration-200">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-rose-50 text-rose-600 dark:bg-rose-500/15 dark:text-rose-400">
        <SearchX className="h-6 w-6" />
      </div>
      <h1 className="mt-4 text-xl font-bold text-slate-900 dark:text-white">
        {t.notFound.title}
      </h1>
      <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
        {t.notFound.description}
      </p>
      <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 rounded-xl bg-indigo-600 px-4 py-2 text-xs font-semibold text-white shadow hover:bg-indigo-500"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>{t.nav.home}</span>
        </Link>
        <Link
          href="/marketplace"
          className="inline-flex items-center rounded-xl border border-slate-300 bg-white px-4 py-2 text-xs font-medium text-slate-700 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:text-white"
        >
          {t.nav.storefront}
        </Link>
      </div>
    </div>
  );
}
