import { NextResponse, type NextRequest } from 'next/server';

const AI_AGENT_BOT_REGEX =
  /(GPTBot|ChatGPT-User|OAI-SearchBot|ClaudeBot|Claude-Web|PerplexityBot|Cursor|Anthropic-ai|Cohere-ai|Google-Extended)/i;

export function middleware(request: NextRequest) {
  const { pathname, searchParams } = request.nextUrl;
  const acceptHeader = request.headers.get('accept') || '';
  const userAgent = request.headers.get('user-agent') || '';
  const formatParam = searchParams.get('format');

  const wantsMarkdown =
    formatParam === 'md' ||
    formatParam === 'markdown' ||
    acceptHeader.includes('text/markdown') ||
    AI_AGENT_BOT_REGEX.test(userAgent);

  if (!wantsMarkdown) {
    return NextResponse.next();
  }

  // Content negotiation: rewrite root `/` or `/marketplace` to `/llms.txt` for AI bots / text/markdown clients
  if (pathname === '/' || pathname === '/marketplace') {
    const url = request.nextUrl.clone();
    url.pathname = '/llms.txt';
    return NextResponse.rewrite(url);
  }

  // Rewrite `/hub/[slug]` to `/hub/[slug]/llms.txt` for pure Markdown specification output
  const hubMatch = pathname.match(/^\/hub\/([^/]+)$/);
  if (hubMatch && hubMatch[1] && ! pathname.endsWith('/llms.txt')) {
    const url = request.nextUrl.clone();
    url.pathname = `/hub/${hubMatch[1]}/llms.txt`;
    return NextResponse.rewrite(url);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/', '/marketplace', '/hub/:slug*'],
};
