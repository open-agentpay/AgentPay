import React from 'react';
import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import {
  PUBLIC_GATEWAY_URL,
  PUBLIC_SITE_URL,
  buildAnswerFirstTldr,
  formatNetworkLabel,
  formatUsdcPrice,
  getMarketplaceRouteBySlug,
  getMarketplaceRoutes,
} from '@/lib/marketplace';
import { RouteDetailView } from '@/components/RouteDetailView';

export const revalidate = 15;

export async function generateMetadata(context: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await context.params;
  const route = await getMarketplaceRouteBySlug(slug);

  if (!route) {
    return {
      title: 'API Route Not Found',
    };
  }

  const priceStr = formatUsdcPrice(
    route.onchain_price_base_units || route.price_base_units
  );
  const tldr = buildAnswerFirstTldr(route);
  const canonicalUrl = `${PUBLIC_SITE_URL}/hub/${route.slug}`;

  return {
    title: `${route.title} (${priceStr} USDC/req) — X402 API`,
    description: tldr,
    keywords: [
      route.title,
      route.slug,
      route.category,
      'x402 API',
      'HTTP 402 Payment Required',
      'OpenAgentPay',
      ...route.tags,
    ],
    alternates: {
      canonical: canonicalUrl,
      types: {
        'text/markdown': `${canonicalUrl}/llms.txt`,
      },
    },
    openGraph: {
      title: `${route.title} — ${priceStr} USDC/req on OpenAgentPay`,
      description: tldr,
      url: canonicalUrl,
      type: 'article',
    },
  };
}

export default async function RouteDetailPage(context: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await context.params;
  const route = await getMarketplaceRouteBySlug(slug);

  if (!route) {
    notFound();
  }

  const allRoutes = await getMarketplaceRoutes();
  const relatedRoutes = allRoutes
    .filter((r) => r.slug !== route.slug)
    .slice(0, 3);

  const priceStr = formatUsdcPrice(
    route.onchain_price_base_units || route.price_base_units
  );
  const netLabel = formatNetworkLabel(route.network, route.network_label);
  const tldr = buildAnswerFirstTldr(route);
  const fullEndpoint = `${PUBLIC_GATEWAY_URL}${route.path_pattern}`;
  const isHealthy = route.sla.health_status === 'healthy';

  // Schema.org WebAPI + Product JSON-LD for Generative Engine Optimization (GEO)
  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': ['WebAPI', 'Product'],
    name: route.title,
    sku: route.slug,
    identifier: route.route_id,
    url: `${PUBLIC_SITE_URL}/hub/${route.slug}`,
    description: tldr,
    category: route.category,
    documentation: `${PUBLIC_SITE_URL}/hub/${route.slug}`,
    termsOfService: `${PUBLIC_SITE_URL}/llms.txt`,
    provider: {
      '@type': 'Organization',
      name: route.provider_name,
      url: PUBLIC_SITE_URL,
    },
    brand: {
      '@type': 'Brand',
      name: 'OpenAgentPay X402 Network',
    },
    offers: {
      '@type': 'Offer',
      price: priceStr,
      priceCurrency: 'USDC',
      availability: isHealthy
        ? 'https://schema.org/InStock'
        : 'https://schema.org/LimitedAvailability',
      url: fullEndpoint,
      priceSpecification: {
        '@type': 'UnitPriceSpecification',
        price: priceStr,
        priceCurrency: 'USDC',
        unitText: 'per HTTP request (x402 V2 EIP-3009)',
      },
    },
    additionalProperty: [
      {
        '@type': 'PropertyValue',
        name: 'Protocol',
        value: 'HTTP 402 Payment Required (x402 V2)',
      },
      {
        '@type': 'PropertyValue',
        name: 'SettlementNetwork',
        value: `${route.network} (${netLabel})`,
      },
      {
        '@type': 'PropertyValue',
        name: 'Gateway24hSuccessRate',
        value: `${route.sla.success_rate_24h}%`,
      },
      {
        '@type': 'PropertyValue',
        name: 'GatewayP95LatencyMs',
        value: `${route.sla.p95_latency_ms} ms`,
      },
    ],
  };

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <RouteDetailView route={route} relatedRoutes={relatedRoutes} />
    </>
  );
}
