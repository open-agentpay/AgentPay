'use client';

import React from 'react';
import Link from 'next/link';
import {
  ExternalLink,
  Terminal,
} from 'lucide-react';
import {
  PUBLIC_CONSOLE_URL,
  PUBLIC_GATEWAY_URL,
} from '@/lib/marketplace';
import { useI18n } from '@/i18n/context';
import { LanguageSwitcher } from './LanguageSwitcher';
import { ThemeSwitcher } from './ThemeSwitcher';

export function HeaderNav() {
  const { t, locale } = useI18n();

  return (
    <header className="sticky top-0 z-50 border-b border-slate-200/80 bg-white/85 dark:border-slate-800/80 dark:bg-[#070B14]/85 backdrop-blur-xl transition-colors duration-200">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-3 px-4 py-3 sm:px-6 lg:px-8">
        {/* Left: Brand Logo & Navigation Links */}
        <div className="flex items-center gap-5 lg:gap-8 min-w-0">
          <Link href="/" className="flex items-center gap-3 group shrink-0">
            {/* Real AgentPay Official Logo */}
            <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-indigo-500/20 bg-indigo-500/5 p-1 shadow-sm transition-transform group-hover:scale-105 dark:border-indigo-500/40 dark:bg-slate-900/80 shrink-0">
              <img
                src="/logo.png"
                alt="AgentPay Logo"
                className="h-8 w-8 object-contain"
              />
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-base font-extrabold tracking-tight text-slate-900 dark:text-white whitespace-nowrap">
                  AgentPay
                </span>
                <span className="rounded-md border border-indigo-500/30 bg-indigo-500/10 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-indigo-600 dark:text-indigo-300 whitespace-nowrap">
                  {t.nav.hubTitle}
                </span>
              </div>
              <p className="hidden xl:block text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap truncate max-w-[260px]">
                {t.nav.subtitle}
              </p>
            </div>
          </Link>

          <nav className="hidden md:flex items-center gap-4 lg:gap-6 text-sm font-medium text-slate-600 dark:text-slate-300 shrink-0">
            <Link
              href="/"
              className="whitespace-nowrap transition-colors hover:text-indigo-600 dark:hover:text-white"
            >
              {t.nav.home}
            </Link>
            <Link
              href="/marketplace"
              className="whitespace-nowrap transition-colors hover:text-indigo-600 dark:hover:text-white"
            >
              {t.nav.storefront}
            </Link>
            <Link
              href="/hub/solana-smart-money-tracker-x402-api"
              className="whitespace-nowrap transition-colors hover:text-indigo-600 dark:hover:text-white"
            >
              {t.nav.exampleRoute}
            </Link>
            <Link
              href="/docs/"
              className="whitespace-nowrap transition-colors hover:text-indigo-600 dark:hover:text-white"
            >
              {t.nav.docs}
            </Link>
          </nav>
        </div>

        {/* Right: Controls & Actions */}
        <div className="flex items-center gap-2 sm:gap-2.5 shrink-0">
          <div className="hidden 2xl:flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-100/70 px-3 py-1.5 font-mono text-xs text-slate-500 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-400 whitespace-nowrap">
            <span className="h-2 w-2 rounded-full bg-emerald-500 dark:bg-emerald-400 animate-pulse" />
            <span>{t.nav.gatewayStatus}</span>
            <span className="text-slate-800 dark:text-slate-200 font-medium">
              {PUBLIC_GATEWAY_URL.replace(/^https?:\/\//, '')}
            </span>
          </div>

          <ThemeSwitcher />
          <LanguageSwitcher />

          <a
            href={PUBLIC_CONSOLE_URL}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 rounded-xl border border-indigo-500/40 bg-indigo-600 px-3.5 py-2 text-xs font-semibold text-white shadow-sm transition-all hover:bg-indigo-500 whitespace-nowrap"
          >
            <Terminal className="h-3.5 w-3.5 shrink-0" />
            <span>{t.nav.consoleBtn}</span>
            <ExternalLink className="h-3 w-3 opacity-80 shrink-0" />
          </a>
        </div>
      </div>
    </header>
  );
}
