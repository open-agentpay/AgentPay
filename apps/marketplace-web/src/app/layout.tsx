import type { Metadata } from 'next';
import { cookies } from 'next/headers';
import { PUBLIC_SITE_URL } from '@/lib/marketplace';
import { I18nProvider } from '@/i18n/context';
import { ThemeProvider } from '@/theme/ThemeContext';
import { HeaderNav } from '@/components/HeaderNav';
import { FooterNav } from '@/components/FooterNav';
import type { SupportedLocale } from '@/i18n/types';
import type { ThemeMode } from '@/theme/ThemeContext';
import './globals.css';

export const metadata: Metadata = {
  metadataBase: new URL(PUBLIC_SITE_URL),
  title: {
    default: 'AgentPay — Autonomous Payment Network & Financial Firewall for AI Agents',
    template: '%s | AgentPay',
  },
  description:
    'Discover verified HTTP 402 (x402 V2) APIs and Agent Skills. Pay per request in Circle USDC with zero gas, live gateway SLA endorsement, and built-in local financial firewall protection.',
  keywords: [
    'AgentPay',
    'OpenAgentPay',
    'x402',
    'HTTP 402 Payment Required',
    'AI Agent Marketplace',
    'USDC Micro-Payments',
    'EIP-3009',
    'MCP Tools',
    'Agentic Web',
  ],
  icons: {
    icon: [
      { url: '/favicon.ico', sizes: 'any' },
      { url: '/favicon.png', type: 'image/png' },
    ],
    apple: '/logo.png',
  },
  alternates: {
    canonical: PUBLIC_SITE_URL,
    types: {
      'text/markdown': `${PUBLIC_SITE_URL}/llms.txt`,
    },
  },
  openGraph: {
    title: 'AgentPay — Autonomous Payment Network & Financial Firewall for AI Agents',
    description:
      'Zero-onboarding HTTP 402 APIs with live Go gateway SLA metrics, instant Python SDK snippets, and /llms.txt machine discovery.',
    url: PUBLIC_SITE_URL,
    siteName: 'AgentPay',
    type: 'website',
  },
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  let initialLocale: SupportedLocale = 'en';
  let initialTheme: ThemeMode = 'dark';

  try {
    const cookieStore = await cookies();
    const loc = cookieStore.get('NEXT_LOCALE')?.value;
    if (loc === 'zh' || loc === 'en') {
      initialLocale = loc;
    }
    const th = cookieStore.get('NEXT_THEME')?.value;
    if (th === 'light' || th === 'dark') {
      initialTheme = th;
    }
  } catch {
    // fallback
  }

  return (
    <html
      lang={initialLocale === 'zh' ? 'zh-CN' : 'en'}
      className={initialTheme === 'dark' ? 'dark' : ''}
      suppressHydrationWarning
    >
      <body className="min-h-screen flex flex-col">
        <ThemeProvider initialTheme={initialTheme}>
          <I18nProvider initialLocale={initialLocale}>
            <HeaderNav />
            <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-8 sm:px-6 lg:px-8">
              {children}
            </main>
            <FooterNav />
          </I18nProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
