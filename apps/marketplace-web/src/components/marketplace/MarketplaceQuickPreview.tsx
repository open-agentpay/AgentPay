'use client';

import React from 'react';
import Link from 'next/link';
import { ArrowUpRight } from 'lucide-react';
import {
  buildOpenAiNodeSdkSnippet,
  buildOpenAiPythonSdkSnippet,
  buildPythonSdkSnippet,
  buildTypescriptSdkSnippet,
  isLlmRoute,
} from '@/lib/marketplace';
import type { MarketplaceRouteItem } from '@/types/marketplace';
import { CopyButton } from '@/components/CopyButton';
import { useI18n } from '@/i18n/context';

interface MarketplaceQuickPreviewProps {
  previewRoute: MarketplaceRouteItem;
  sdkLang: 'python' | 'typescript';
  onLangChange: (lang: 'python' | 'typescript') => void;
  onClose: () => void;
}

export function MarketplaceQuickPreview({
  previewRoute,
  sdkLang,
  onLangChange,
  onClose,
}: MarketplaceQuickPreviewProps) {
  const { t, locale } = useI18n();

  const isLlm = isLlmRoute(previewRoute);
  const activeInstallCmd = isLlm
    ? sdkLang === 'python'
      ? 'pip install openai'
      : 'npm install openai'
    : sdkLang === 'python'
    ? 'pip install agentpay'
    : 'npm install agentpay-core-sdk';

  const previewSdkSnippet = isLlm
    ? sdkLang === 'python'
      ? buildOpenAiPythonSdkSnippet(previewRoute, locale)
      : buildOpenAiNodeSdkSnippet(previewRoute, locale)
    : sdkLang === 'python'
    ? buildPythonSdkSnippet(previewRoute, locale)
    : buildTypescriptSdkSnippet(previewRoute, locale);

  return (
    <section className="rounded-2xl border border-indigo-500/40 bg-slate-900 p-6 text-white shadow-2xl">
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-4">
        <div>
          <div className="flex items-center gap-2">
            <span className="rounded-md bg-indigo-500/20 px-2 py-0.5 font-mono text-xs text-indigo-300">
              {isLlm ? t.openAiSdkBadge : t.preview.sdkTitle}
            </span>
            <h3 className="text-base font-bold text-white">{previewRoute.title}</h3>
          </div>
          <p className="mt-1 text-xs text-slate-400">
            {t.preview.sdkSubtitle
              .replace('{endpoint}', previewRoute.path_pattern)
              .replace('{slug}', previewRoute.slug)}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex rounded-lg border border-slate-700 bg-slate-950 p-0.5 text-xs">
            <button
              type="button"
              onClick={() => onLangChange('python')}
              className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                sdkLang === 'python'
                  ? 'bg-indigo-600 text-white shadow-sm'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              Python
            </button>
            <button
              type="button"
              onClick={() => onLangChange('typescript')}
              className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                sdkLang === 'typescript'
                  ? 'bg-indigo-600 text-white shadow-sm'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              TypeScript
            </button>
          </div>

          <CopyButton text={previewSdkSnippet} label={t.preview.copyCode} />

          <Link
            href={`/hub/${previewRoute.slug}`}
            className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-indigo-500"
          >
            <span>{t.preview.viewDetails}</span>
            <ArrowUpRight className="h-3.5 w-3.5" />
          </Link>

          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-slate-700 px-2.5 py-1.5 text-xs text-slate-400 hover:bg-slate-800 hover:text-white"
          >
            {t.preview.close}
          </button>
        </div>
      </div>

      <div className="mt-3 flex items-center justify-between text-xs text-slate-400">
        <span className="font-mono">$ {activeInstallCmd}</span>
        <span className="font-mono text-indigo-300">
          {isLlm
            ? sdkLang === 'python'
              ? 'Python 3.8+ / openai SDK'
              : 'Node.js 18+ / openai SDK'
            : sdkLang === 'python'
            ? 'Python 3.10+'
            : 'Node.js 20+ / TypeScript'}
        </span>
      </div>

      <pre className="mt-2 overflow-x-auto rounded-xl border border-slate-800 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-slate-200">
        <code>{previewSdkSnippet}</code>
      </pre>
    </section>
  );
}
