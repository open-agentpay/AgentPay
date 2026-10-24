import React from 'react';
import type { Metadata } from 'next';
import {
  PUBLIC_GATEWAY_URL,
  PUBLIC_SITE_URL,
  formatUsdcPrice,
  getMarketplaceRoutes,
} from '@/lib/marketplace';
import { MarketplaceExplorer } from '@/components/MarketplaceExplorer';

export const revalidate = 15;

export const metadata: Metadata = {
  title: 'API Marketplace — X402 Pay-Per-Request APIs & SDK Playground',
  description:
    'Public directory of HTTP 402 (x402 V2) APIs and skills for autonomous AI agents with Circle USDC micro-payments, live gateway SLA metrics, and AgentPay SDK verification.',
  alternates: {
    canonical: `${PUBLIC_SITE_URL}/marketplace`,
  },
};

export default async function MarketplacePage() {
  const routes = await getMarketplaceRoutes();

  // Schema.org ItemList + WebAPI structured data for Generative Engine Optimization (GEO)
  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'ItemList',
    name: 'OpenAgentPay X402 Pay-Per-Request API Marketplace',
    description:
      'Public directory of HTTP 402 (x402 V2) APIs and skills for autonomous AI agents with Circle USDC micro-payments and live gateway SLA metrics.',
    url: `${PUBLIC_SITE_URL}/marketplace`,
    numberOfItems: routes.length,
    itemListElement: routes.map((route, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      item: {
        '@type': ['WebAPI', 'Product'],
        name: route.title,
        url: `${PUBLIC_SITE_URL}/hub/${route.slug}`,
        description: route.summary || route.description_md,
        category: route.category,
        documentation: `${PUBLIC_SITE_URL}/hub/${route.slug}`,
        provider: {
          '@type': 'Organization',
          name: route.provider_name,
        },
        offers: {
          '@type': 'Offer',
          price: formatUsdcPrice(route.onchain_price_base_units || route.price_base_units),
          priceCurrency: 'USDC',
          availability:
            route.sla.health_status === 'healthy'
              ? 'https://schema.org/InStock'
              : 'https://schema.org/LimitedAvailability',
          url: `${PUBLIC_GATEWAY_URL}${route.path_pattern}`,
        },
      },
    })),
  };

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <MarketplaceExplorer initialRoutes={routes} />
    </>
  );
}
