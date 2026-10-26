'use client';

import React from 'react';
import Link from 'next/link';
import {
  PUBLIC_CONSOLE_URL,
  PUBLIC_DOCS_URL,
  PUBLIC_GATEWAY_URL,
  PUBLIC_GITHUB_URL,
  PUBLIC_TWITTER_URL,
  PUBLIC_DISCORD_URL,
} from '@/lib/marketplace';
import { useI18n } from '@/i18n/context';

export function FooterNav() {
  const { t, locale } = useI18n();

  return (
    <footer className="mt-16 border-t border-slate-200/80 bg-slate-50/80 dark:border-slate-800/80 dark:bg-[#050811] py-10 text-xs text-slate-500 dark:text-slate-400 transition-colors duration-200">
      <div className="mx-auto flex w-full max-w-[1600px] flex-col justify-between gap-6 px-4 sm:flex-row sm:items-center sm:px-6 lg:px-8 xl:px-10">
        <div className="space-y-1.5">
          <div className="flex items-center gap-2.5 font-semibold text-slate-900 dark:text-slate-200">
            <img src="/logo.png" alt="AgentPay" className="h-5 w-5 object-contain" />
            <span>{t.footer.brandTagline}</span>
          </div>
          <p className="text-slate-500 dark:text-slate-400">
            {t.footer.architectureNote}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-5 font-mono text-xs">
          <Link
            href="/"
            className="hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            {t.nav.home}
          </Link>
          <Link
            href="/marketplace"
            className="hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            {t.footer.storefront}
          </Link>
          <a
            href={PUBLIC_DOCS_URL}
            className="hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            {t.nav.docs}
          </a>
          <a
            href={`${PUBLIC_GATEWAY_URL}/api/v1/public/marketplace/routes`}
            target="_blank"
            rel="noreferrer"
            className="hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            {t.footer.registryApi}
          </a>
          <a
            href={PUBLIC_CONSOLE_URL}
            target="_blank"
            rel="noreferrer"
            className="hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            {t.footer.sellerConsole}
          </a>
          <a
            href={PUBLIC_GITHUB_URL}
            target="_blank"
            rel="noreferrer"
            className="hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            GitHub
          </a>
          <a
            href={PUBLIC_TWITTER_URL}
            target="_blank"
            rel="noreferrer"
            className="hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            X (Twitter)
          </a>
          <a
            href={PUBLIC_DISCORD_URL}
            target="_blank"
            rel="noreferrer"
            className="hover:text-slate-900 dark:hover:text-white transition-colors"
          >
            Discord
          </a>
        </div>
      </div>
    </footer>
  );
}
