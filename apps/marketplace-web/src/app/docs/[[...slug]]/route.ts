import { NextRequest, NextResponse } from 'next/server';
import fs from 'fs';
import path from 'path';

const MIME_TYPES: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.mjs': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.webp': 'image/webp',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.txt': 'text/plain; charset=utf-8',
  '.xml': 'application/xml; charset=utf-8',
};

export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ slug?: string[] }> }
) {
  const { slug } = await context.params;
  const docsDir = path.join(process.cwd(), 'public', 'docs');

  if (!fs.existsSync(docsDir)) {
    return new NextResponse(
      'Documentation build artifacts not found in public/docs. Please run "npm run docs:build".',
      { status: 404, headers: { 'Content-Type': 'text/plain; charset=utf-8' } }
    );
  }

  const parts = slug && slug.length > 0 ? slug : [];
  const relativePath = parts.join('/');

  let filePath = path.join(docsDir, relativePath);

  // Resolution order:
  // 1. Directory -> append index.html
  // 2. Exact file exists
  // 3. filePath + .html exists (for VitePress cleanUrls)
  // 4. filePath + /index.html exists
  if (fs.existsSync(filePath) && fs.statSync(filePath).isDirectory()) {
    filePath = path.join(filePath, 'index.html');
  } else if (!fs.existsSync(filePath)) {
    if (fs.existsSync(`${filePath}.html`)) {
      filePath = `${filePath}.html`;
    } else if (fs.existsSync(path.join(filePath, 'index.html'))) {
      filePath = path.join(filePath, 'index.html');
    }
  }

  // If still not found, try 404.html in docsDir
  if (!fs.existsSync(filePath) || fs.statSync(filePath).isDirectory()) {
    const notFoundHtml = path.join(docsDir, '404.html');
    if (fs.existsSync(notFoundHtml)) {
      const fallbackContent = fs.readFileSync(notFoundHtml);
      return new NextResponse(fallbackContent, {
        status: 404,
        headers: { 'Content-Type': 'text/html; charset=utf-8' },
      });
    }
    return new NextResponse('Documentation page not found', { status: 404 });
  }

  const ext = path.extname(filePath).toLowerCase();
  const contentType = MIME_TYPES[ext] || 'application/octet-stream';
  const fileBuffer = fs.readFileSync(filePath);

  return new NextResponse(fileBuffer, {
    status: 200,
    headers: {
      'Content-Type': contentType,
      'Cache-Control':
        ext === '.html'
          ? 'public, max-age=60'
          : 'public, max-age=31536000, immutable',
    },
  });
}
