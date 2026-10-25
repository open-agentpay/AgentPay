'use client';

import React from 'react';
import Link from 'next/link';
import {
  ExternalLink,
  Terminal,
} from 'lucide-react';
import {
  PUBLIC_CONSOLE_URL,
  PUBLIC_DOCS_URL,
  PUBLIC_GATEWAY_URL,
  PUBLIC_GITHUB_URL,
  PUBLIC_TWITTER_URL,
  PUBLIC_DISCORD_URL,
} from '@/lib/marketplace';
import { useI18n } from '@/i18n/context';
import { LanguageSwitcher } from './LanguageSwitcher';
import { ThemeSwitcher } from './ThemeSwitcher';

function GithubIcon({ className = 'h-4 w-4' }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
      />
    </svg>
  );
}

function XIcon({ className = 'h-4 w-4' }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
    </svg>
  );
}

function DiscordIcon({ className = 'h-4 w-4' }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994.021-.041.001-.09-.041-.106a13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.929 1.793 8.18 1.793 12.061 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.894.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.028zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"
      />
    </svg>
  );
}

export function HeaderNav() {
  const { t, locale } = useI18n();

  return (
    <header className="sticky top-0 z-50 border-b border-slate-200/80 bg-white/85 dark:border-slate-800/80 dark:bg-[#070B14]/85 backdrop-blur-xl transition-colors duration-200">
      <div className="mx-auto flex w-full max-w-[1600px] items-center justify-between gap-4 px-4 py-3 sm:px-6 lg:px-8 xl:px-10">
        {/* Left: Brand Logo & Navigation Links */}
        <div className="flex items-center gap-4 lg:gap-6 xl:gap-8 shrink-0">
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
              <p className="hidden 2xl:block text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap truncate max-w-[220px]">
                {t.nav.subtitle}
              </p>
            </div>
          </Link>

          <nav className="hidden lg:flex items-center gap-4 xl:gap-6 text-sm font-medium text-slate-600 dark:text-slate-300 shrink-0">
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
            <a
              href={PUBLIC_DOCS_URL}
              className="whitespace-nowrap transition-colors hover:text-indigo-600 dark:hover:text-white"
            >
              {t.nav.docs}
            </a>
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

          {/* Social Links: GitHub, X, Discord */}
          <div className="flex items-center gap-1.5">
            <a
              href={PUBLIC_GITHUB_URL}
              target="_blank"
              rel="noreferrer"
              className="flex h-9 w-9 items-center justify-center rounded-xl border border-slate-200 bg-white p-2 text-slate-600 shadow-sm transition-all hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/80 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-white"
              title="GitHub"
              aria-label="GitHub Repository"
            >
              <GithubIcon className="h-4 w-4" />
            </a>
            <a
              href={PUBLIC_TWITTER_URL}
              target="_blank"
              rel="noreferrer"
              className="flex h-9 w-9 items-center justify-center rounded-xl border border-slate-200 bg-white p-2 text-slate-600 shadow-sm transition-all hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/80 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-white"
              title="X (Twitter)"
              aria-label="Follow @OpenAgentPay on X"
            >
              <XIcon className="h-3.5 w-3.5" />
            </a>
            <a
              href={PUBLIC_DISCORD_URL}
              target="_blank"
              rel="noreferrer"
              className="flex h-9 w-9 items-center justify-center rounded-xl border border-slate-200 bg-white p-2 text-slate-600 shadow-sm transition-all hover:border-slate-300 hover:text-[#5865F2] dark:border-slate-800 dark:bg-slate-900/80 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-[#5865F2]"
              title="Discord"
              aria-label="Join AgentPay Discord"
            >
              <DiscordIcon className="h-4 w-4" />
            </a>
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
