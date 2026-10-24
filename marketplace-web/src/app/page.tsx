import React from 'react';
import type { Metadata } from 'next';
import { PUBLIC_SITE_URL } from '@/lib/marketplace';
import { HomePageView } from '@/components/HomePageView';

export const metadata: Metadata = {
  title: 'AgentPay — Autonomous Payment Network & Financial Firewall for AI Agents',
  description:
    'Open-source autonomous payment network & financial firewall for AI Agents. Dual-rail micro-payments with zero-gas Circle USDC and local budget enforcement.',
  alternates: {
    canonical: PUBLIC_SITE_URL,
  },
};

export default function HomePage() {
  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'SoftwareApplication',
    name: 'AgentPay',
    applicationCategory: 'PaymentApplication',
    operatingSystem: 'All',
    description:
      'Open-source autonomous payment network & financial firewall for AI Agents. Dual-rail micro-payments, local hard budget enforcement, zero-gas USDC, and x402 gateway.',
    url: PUBLIC_SITE_URL,
    offers: {
      '@type': 'Offer',
      price: '0',
      priceCurrency: 'USD',
    },
  };

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <HomePageView />
    </>
  );
}
