import { NextResponse } from 'next/server';
import { getRandomMarketplaceRoute } from '@/lib/marketplace';

export const dynamic = 'force-dynamic';

export async function GET(request: Request) {
  const randomRoute = await getRandomMarketplaceRoute();
  if (randomRoute?.slug) {
    return NextResponse.redirect(new URL(`/hub/${randomRoute.slug}`, request.url), {
      status: 307,
      headers: {
        'Cache-Control': 'no-store, max-age=0, must-revalidate',
      },
    });
  }
  return NextResponse.redirect(new URL('/marketplace', request.url), 307);
}
