'use client';

import React from 'react';
import { Globe } from 'lucide-react';
import { useI18n } from '@/i18n/context';

export function LanguageSwitcher() {
  const { locale, setLocale } = useI18n();

  return (
    <div className="flex items-center gap-1 rounded-xl border border-slate-200 bg-white/90 p-1 font-mono text-xs text-slate-600 shadow-sm dark:border-slate-800 dark:bg-slate-900/80 dark:text-slate-300">
      <Globe className="ml-1.5 h-3.5 w-3.5 text-slate-400 dark:text-slate-500" />
      <button
        type="button"
        onClick={() => setLocale('zh')}
        className={`rounded-lg px-2 py-1 text-xs font-medium transition-all ${
          locale === 'zh'
            ? 'bg-indigo-600 text-white shadow-sm'
            : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200'
        }`}
        title="切换为简体中文"
      >
        中文
      </button>
      <button
        type="button"
        onClick={() => setLocale('en')}
        className={`rounded-lg px-2 py-1 text-xs font-medium transition-all ${
          locale === 'en'
            ? 'bg-indigo-600 text-white shadow-sm'
            : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200'
        }`}
        title="Switch to English"
      >
        EN
      </button>
    </div>
  );
}
