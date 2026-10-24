# OpenAgentPay Marketplace (`marketplace-web`)

The public Storefront & Generative Engine Optimization (GEO) portal for **OpenAgentPay** (`openagentpay.dev`), built with **Next.js 15 (App Router) + React 19 + Tailwind CSS + TypeScript**.

## Core Responsibilities

1. **`/` (Storefront & Search Hub)**:
   - Interactive search, category filtering, and chain network filtering across verified X402 APIs.
   - Displays live Go Gateway SLA telemetry (`24h 200 OK` success rate, `P95` latency, 24h paid call volume, and health status).
2. **`/hub/[slug]` (GEO-Optimized API Detail Page)**:
   - **Answer-First TL;DR**: Standardized opening summary for AI search engines (ChatGPT, Claude, Perplexity, Cursor).
   - **Zero-Friction Python SDK Snippet**: Dynamically generated `AgentPayClient` snippet with one-click copy.
   - **Native HTML `<table>` Specifications**: High signal-to-noise parameter and SLA tables.
   - **Schema.org `JSON-LD`**: Embedded `WebAPI` + `Product` structured data with `offers.price` and `priceCurrency: "USDC"`.
3. **`/llms.txt` & Content Negotiation**:
   - Dynamic `/llms.txt` and `/hub/[slug]/llms.txt` endpoints serving pure Markdown specifications.
   - Next.js Middleware (`src/middleware.ts`) automatically detects AI crawlers (`GPTBot`, `ClaudeBot`, `PerplexityBot`, `Cursor`) or `Accept: text/markdown` headers and returns clean Markdown.

## Local Development

```bash
# Install dependencies from monorepo root or inside marketplace-web
npm install

# Start development server on http://localhost:3000
npm run dev

# Build for production
npm run build
```

## Environment Variables

- `GATEWAY_INTERNAL_URL`: Internal URL to fetch live public routes from Go Gateway (`default: http://127.0.0.1:8080`)
- `NEXT_PUBLIC_GATEWAY_URL`: Public X402 Gateway URL shown in SDK snippets (`default: https://gate.openagentpay.dev`)
- `NEXT_PUBLIC_SITE_URL`: Canonical Marketplace domain (`default: https://openagentpay.dev`)
- `NEXT_PUBLIC_CONSOLE_URL`: Seller/Buyer Console URL (`default: https://console.openagentpay.dev`)
