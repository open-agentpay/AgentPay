'use client';

import React, { useState } from 'react';
import { FlaskConical, Shield, ShieldCheck, Terminal, Zap } from 'lucide-react';
import { CopyButton } from '@/components/CopyButton';
import { useI18n } from '@/i18n/context';

export function MarketplaceIntro() {
  const { t } = useI18n();
  const [sdkLang, setSdkLang] = useState<'python' | 'typescript'>('python');

  const pythonSnippet = `import os
from agentpay_core import AgentPayClient, PolicyConfig, SQLitePolicyEngine

# 1. 策略防火墙：仅配置预算和白名单域名
policy = SQLitePolicyEngine.create_in_memory(
    PolicyConfig(
        id="weather_agent",
        max_per_tx_base_units=250000,  # 0.25 Credits
        rolling_budget_24h_base_units=5000000,  # 5.00 Credits
        whitelist_domains={"gate.aipmedia.cn"},
    )
)

# 2. 客户端直接传入 api_key，自动开启虚拟钱包模式与 402 自动握手
client = AgentPayClient(
    api_key=os.getenv("AGENT_API_KEY", "apk_live_xxx"),
    policy=policy,
)

# 3. 正常发起调用
response = client.get(
    "https://gate.aipmedia.cn/api/v1/simple-weather",
    params={"city": "北京"}
)
print("Response:", response.json())`;

  const tsSnippet = `import { AgentPayClient, PolicyConfig, SQLitePolicyEngine } from 'agentpay-core-sdk';

// 1. 策略防火墙配置
const policy = SQLitePolicyEngine.createInMemory(
  new PolicyConfig({
    id: 'weather_agent',
    maxPerTxBaseUnits: 250000n,
    rollingBudget24hBaseUnits: 5000000n,
    whitelistDomains: ['gate.aipmedia.cn'],
  })
);

// 2. 客户端直接传入 apiKey
const client = new AgentPayClient({
  apiKey: process.env.AGENT_API_KEY ?? 'apk_live_xxx',
  policy,
});

// 3. 正常发起调用
const response = await client.get('https://gate.aipmedia.cn/api/v1/simple-weather', {
  params: { timeframe: '24h' },
});
console.log('Response:', await response.json());`;

  const activeSnippet = sdkLang === 'python' ? pythonSnippet : tsSnippet;
  const activeInstallCmd =
    sdkLang === 'python' ? 'pip install agentpay-core-sdk' : 'npm install agentpay-core-sdk';

  return (
    <section className="relative overflow-hidden rounded-3xl border border-slate-200 bg-gradient-to-b from-slate-50/50 via-white to-slate-50/50 p-6 shadow-sm transition-colors duration-200 dark:border-slate-800/80 dark:bg-gradient-to-b dark:from-[#0B1120] dark:via-slate-900/40 dark:to-[#080D1A] sm:p-10">
      <div className="space-y-8">
        {/* Header & Product Vision */}
        <div>
          <div className="inline-flex items-center gap-2 rounded-full border border-indigo-500/30 bg-indigo-50 px-3.5 py-1 text-xs font-semibold text-indigo-700 dark:border-indigo-500/40 dark:bg-indigo-500/15 dark:text-indigo-300">
            <Shield className="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
            <span>{t.agentPayIntro.badge}</span>
          </div>
          <h2 className="mt-3 text-2xl font-extrabold tracking-tight text-slate-900 sm:text-3xl dark:text-white [text-wrap:balance]">
            {t.agentPayIntro.title}
          </h2>
          <p className="mt-2 text-sm leading-relaxed text-slate-600 sm:text-base dark:text-slate-300 [text-wrap:pretty]">
            {t.agentPayIntro.subtitle}
          </p>
        </div>

        {/* Role of this Marketplace Box */}
        <div className="rounded-2xl border border-cyan-300/60 bg-gradient-to-r from-cyan-50/70 via-indigo-50/40 to-slate-50 p-5 shadow-sm dark:border-cyan-500/30 dark:bg-gradient-to-r dark:from-cyan-950/30 dark:via-slate-900/90 dark:to-indigo-950/30 sm:p-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-cyan-500/30 bg-cyan-500/10 text-cyan-700 shadow-sm dark:border-cyan-500/40 dark:bg-cyan-500/20 dark:text-cyan-300">
              <FlaskConical className="h-6 w-6 text-cyan-600 dark:text-cyan-400" />
            </div>
            <div className="space-y-1.5">
              <div className="flex flex-wrap items-center gap-2">
                <span className="rounded-md border border-cyan-500/30 bg-cyan-100 px-2 py-0.5 font-mono text-[11px] font-semibold text-cyan-800 dark:bg-cyan-500/20 dark:text-cyan-300">
                  {t.agentPayIntro.marketplaceRoleBadge}
                </span>
                <h3 className="text-base font-bold text-slate-900 dark:text-white">
                  {t.agentPayIntro.marketplaceRoleTitle}
                </h3>
              </div>
              <p className="text-sm leading-relaxed text-slate-700 dark:text-slate-300">
                {t.agentPayIntro.marketplaceRoleDesc}
              </p>
            </div>
          </div>
        </div>

        {/* Three Core Pillars */}
        <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
          <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-indigo-400 dark:border-slate-800 dark:bg-slate-950/70 dark:hover:border-indigo-500/40">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600 dark:bg-indigo-500/15 dark:text-indigo-400">
              <ShieldCheck className="h-5 w-5" />
            </div>
            <h4 className="mt-3.5 text-sm font-bold text-slate-900 dark:text-white">
              {t.agentPayIntro.pillar1Title}
            </h4>
            <p className="mt-2 text-xs leading-relaxed text-slate-600 dark:text-slate-400">
              {t.agentPayIntro.pillar1Desc}
            </p>
          </div>

          <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-emerald-400 dark:border-slate-800 dark:bg-slate-950/70 dark:hover:border-emerald-500/40">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400">
              <Zap className="h-5 w-5" />
            </div>
            <h4 className="mt-3.5 text-sm font-bold text-slate-900 dark:text-white">
              {t.agentPayIntro.pillar2Title}
            </h4>
            <p className="mt-2 text-xs leading-relaxed text-slate-600 dark:text-slate-400">
              {t.agentPayIntro.pillar2Desc}
            </p>
          </div>

          <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-cyan-400 dark:border-slate-800 dark:bg-slate-950/70 dark:hover:border-cyan-500/40">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-cyan-50 text-cyan-600 dark:bg-cyan-500/15 dark:text-cyan-400">
              <Terminal className="h-5 w-5" />
            </div>
            <h4 className="mt-3.5 text-sm font-bold text-slate-900 dark:text-white">
              {t.agentPayIntro.pillar3Title}
            </h4>
            <p className="mt-2 text-xs leading-relaxed text-slate-600 dark:text-slate-400">
              {t.agentPayIntro.pillar3Desc}
            </p>
          </div>
        </div>

        {/* Quick SDK Verification Code Card */}
        <div className="rounded-2xl border border-slate-200 bg-slate-900 p-5 text-white dark:border-slate-800 dark:bg-slate-950/80">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 pb-3">
            <div>
              <span className="text-xs font-bold text-slate-100">
                {t.agentPayIntro.codeSnippetTitle}
              </span>
              <p className="mt-0.5 text-xs text-slate-400">
                {t.agentPayIntro.codeSnippetDesc}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <div className="inline-flex rounded-lg border border-slate-700 bg-slate-950 p-0.5 text-xs">
                <button
                  type="button"
                  onClick={() => setSdkLang('python')}
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
                  onClick={() => setSdkLang('typescript')}
                  className={`rounded-md px-2.5 py-1 font-mono font-medium transition ${
                    sdkLang === 'typescript'
                      ? 'bg-indigo-600 text-white shadow-sm'
                      : 'text-slate-400 hover:text-white'
                  }`}
                >
                  TypeScript
                </button>
              </div>
              <span className="font-mono text-xs text-indigo-300">{activeInstallCmd}</span>
              <CopyButton text={activeSnippet} label={t.preview.copyCode} />
            </div>
          </div>
          <pre className="mt-3.5 overflow-x-auto rounded-xl border border-slate-800/80 bg-[#050811] p-4 font-mono text-xs leading-relaxed text-slate-200">
            <code>{activeSnippet}</code>
          </pre>
        </div>
      </div>
    </section>
  );
}
