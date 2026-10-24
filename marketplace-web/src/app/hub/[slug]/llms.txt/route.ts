import { buildRouteMarkdownDoc, getMarketplaceRouteBySlug } from '@/lib/marketplace';

export const dynamic = 'force-dynamic';

export async function GET(
  _request: Request,
  context: { params: Promise<{ slug: string }> }
) {
  const { slug } = await context.params;
  const route = await getMarketplaceRouteBySlug(slug);

  if (!route) {
    return new Response(`# 404 Not Found\n\nRoute "${slug}" was not found in OpenAgentPay Marketplace.\n`, {
      status: 404,
      headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
    });
  }

  const md = buildRouteMarkdownDoc(route);
  return new Response(md, {
    status: 200,
    headers: {
      'Content-Type': 'text/markdown; charset=utf-8',
      'Cache-Control': 'public, s-maxage=30, stale-while-revalidate=60',
    },
  });
}
