'use client';

import React from 'react';
import katex from 'katex';
import 'katex/dist/katex.min.css';

interface MarkdownContentProps {
  content: string;
  className?: string;
}

type InlineToken =
  | { type: 'text'; text: string }
  | { type: 'code'; text: string }
  | { type: 'math'; formula: string; displayMode: boolean }
  | { type: 'bold'; children: InlineToken[] }
  | { type: 'italic'; children: InlineToken[] }
  | { type: 'link'; text: string; href: string };

type BlockToken =
  | { type: 'heading'; level: number; text: string }
  | { type: 'code'; lang?: string; code: string }
  | { type: 'math'; formula: string }
  | { type: 'blockquote'; lines: string[] }
  | { type: 'hr' }
  | { type: 'list'; ordered: boolean; items: string[] }
  | { type: 'table'; headers: string[]; rows: string[][] }
  | { type: 'paragraph'; text: string };

/**
 * Safely renders LaTeX math into HTML via KaTeX.
 */
function renderKaTeX(formula: string, displayMode: boolean): string {
  try {
    return katex.renderToString(formula, {
      displayMode,
      throwOnError: false,
    });
  } catch {
    return formula;
  }
}

/**
 * Tokenizes inline markdown text into formatted tokens (math, bold, italic, code, links).
 */
function parseInline(text: string): InlineToken[] {
  if (!text) return [];

  // Match: $$display-math$$, $inline-math$, `inline code`, **bold**, __bold__, [link](url), *italic*, _italic_
  const tokenRegex = /(\$\$[^\$]+\$\$|\$[^$\n]+\$|`[^`]+`|\*\*[^*]+\*\*|__[^_]+__|\[[^\]]+\]\([^)]+\)|\*[^*]+\*|_[^_]+_)/g;
  const parts = text.split(tokenRegex);

  return parts
    .map((part): InlineToken | null => {
      if (!part) return null;

      // 1. $$inline-block math$$
      if (part.startsWith('$$') && part.endsWith('$$') && part.length > 4) {
        return {
          type: 'math',
          formula: part.slice(2, -2).trim(),
          displayMode: true,
        };
      }

      // 2. $inline math$ (distinguish from plain currency like $100 or $10,000)
      if (part.startsWith('$') && part.endsWith('$') && part.length > 2) {
        const inner = part.slice(1, -1).trim();
        // Skip purely numeric currency formats: $100, $10,000, $0.05
        if (!/^\d+(?:,\d+)*(?:\.\d+)?$/.test(inner)) {
          return {
            type: 'math',
            formula: inner,
            displayMode: false,
          };
        }
      }

      // 3. Inline code
      if (part.startsWith('`') && part.endsWith('`') && part.length >= 2) {
        return { type: 'code', text: part.slice(1, -1) };
      }

      // 4. Bold
      if (
        (part.startsWith('**') && part.endsWith('**') && part.length >= 4) ||
        (part.startsWith('__') && part.endsWith('__') && part.length >= 4)
      ) {
        return { type: 'bold', children: parseInline(part.slice(2, -2)) };
      }

      // 5. Italic
      if (
        (part.startsWith('*') && part.endsWith('*') && part.length >= 2) ||
        (part.startsWith('_') && part.endsWith('_') && part.length >= 2)
      ) {
        return { type: 'italic', children: parseInline(part.slice(1, -1)) };
      }

      // 6. Link
      const linkMatch = part.match(/^\[(.*?)\]\((.*?)\)$/);
      if (linkMatch) {
        return { type: 'link', text: linkMatch[1], href: linkMatch[2] };
      }

      return { type: 'text', text: part };
    })
    .filter((token): token is InlineToken => token !== null);
}

/**
 * Renders inline tokens into React elements.
 */
function renderInline(tokens: InlineToken[], keyPrefix = 'inl'): React.ReactNode {
  return tokens.map((token, index) => {
    const key = `${keyPrefix}-${index}`;
    switch (token.type) {
      case 'math': {
        const html = renderKaTeX(token.formula, token.displayMode);
        return (
          <span
            key={key}
            className={
              token.displayMode
                ? 'my-2.5 block overflow-x-auto py-1 text-center font-serif'
                : 'inline-block px-1 align-baseline font-serif'
            }
            dangerouslySetInnerHTML={{ __html: html }}
          />
        );
      }
      case 'code':
        return (
          <code
            key={key}
            className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs font-semibold text-indigo-600 dark:bg-slate-800 dark:text-indigo-400 border border-slate-200/80 dark:border-slate-700/60"
          >
            {token.text}
          </code>
        );
      case 'bold':
        return (
          <strong key={key} className="font-bold text-slate-900 dark:text-white">
            {renderInline(token.children, `${key}-b`)}
          </strong>
        );
      case 'italic':
        return (
          <em key={key} className="italic text-slate-800 dark:text-slate-200">
            {renderInline(token.children, `${key}-i`)}
          </em>
        );
      case 'link':
        return (
          <a
            key={key}
            href={token.href}
            target="_blank"
            rel="noopener noreferrer"
            className="font-medium text-indigo-600 underline decoration-indigo-400 underline-offset-2 hover:text-indigo-500 dark:text-indigo-400 dark:hover:text-indigo-300"
          >
            {token.text}
          </a>
        );
      case 'text':
      default:
        return <React.Fragment key={key}>{token.text}</React.Fragment>;
    }
  });
}

function isTableSeparator(line: string): boolean {
  const trimmed = line.trim();
  if (!trimmed.startsWith('|') || !trimmed.endsWith('|')) return false;
  const inner = trimmed.slice(1, -1).split('|');
  return inner.every((cell) => /^[\s:-]+$/.test(cell.trim()) && cell.includes('-'));
}

function parseTableRow(line: string): string[] {
  return line
    .trim()
    .slice(1, -1)
    .split('|')
    .map((s) => s.trim());
}

/**
 * Parses markdown source into block tokens.
 */
function parseBlocks(markdown: string): BlockToken[] {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const blocks: BlockToken[] = [];

  let currentList: { ordered: boolean; items: string[] } | null = null;
  let inCode = false;
  let codeLang = '';
  let codeLines: string[] = [];

  let inMath = false;
  let mathLines: string[] = [];

  let tableHeaders: string[] | null = null;
  let tableRows: string[][] = [];

  function flushList() {
    if (currentList && currentList.items.length > 0) {
      blocks.push({
        type: 'list',
        ordered: currentList.ordered,
        items: [...currentList.items],
      });
      currentList = null;
    }
  }

  function flushTable() {
    if (tableHeaders && tableHeaders.length > 0) {
      blocks.push({
        type: 'table',
        headers: tableHeaders,
        rows: tableRows,
      });
      tableHeaders = null;
      tableRows = [];
    }
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();

    // 1. Multi-line or single-line Math block: $$ ... $$
    if (trimmed.startsWith('$$')) {
      if (inMath) {
        const after = trimmed.endsWith('$$') ? trimmed.slice(0, -2).trim() : trimmed.slice(2).trim();
        if (after) mathLines.push(after);
        blocks.push({
          type: 'math',
          formula: mathLines.join('\n').trim(),
        });
        inMath = false;
        mathLines = [];
        continue;
      }

      flushList();
      flushTable();

      // Check if it's single line $$formula$$
      if (trimmed.endsWith('$$') && trimmed.length > 4) {
        blocks.push({
          type: 'math',
          formula: trimmed.slice(2, -2).trim(),
        });
        continue;
      }

      // Multi-line math block opening with $$
      inMath = true;
      const contentAfter = trimmed.slice(2).trim();
      mathLines = contentAfter ? [contentAfter] : [];
      continue;
    }

    if (inMath) {
      if (trimmed.endsWith('$$')) {
        const contentBefore = trimmed.slice(0, -2).trim();
        if (contentBefore) mathLines.push(contentBefore);
        blocks.push({
          type: 'math',
          formula: mathLines.join('\n').trim(),
        });
        inMath = false;
        mathLines = [];
      } else {
        mathLines.push(line);
      }
      continue;
    }

    // 2. Fenced Code Block
    if (line.startsWith('```')) {
      if (inCode) {
        blocks.push({
          type: 'code',
          lang: codeLang,
          code: codeLines.join('\n'),
        });
        inCode = false;
        codeLines = [];
        codeLang = '';
      } else {
        flushList();
        flushTable();
        inCode = true;
        codeLang = trimmed.slice(3).trim();
      }
      continue;
    }

    if (inCode) {
      codeLines.push(line);
      continue;
    }

    // 3. Empty Line
    if (!trimmed) {
      flushList();
      flushTable();
      continue;
    }

    // 4. Table detection
    if (trimmed.startsWith('|') && trimmed.endsWith('|')) {
      flushList();
      if (!tableHeaders) {
        if (i + 1 < lines.length && isTableSeparator(lines[i + 1])) {
          tableHeaders = parseTableRow(line);
          i++; // Skip separator line
          tableRows = [];
          continue;
        }
      } else {
        tableRows.push(parseTableRow(line));
        continue;
      }
    } else if (tableHeaders) {
      flushTable();
    }

    // 5. Horizontal Rule
    if (/^(---+|\*\*\*+|___+)$/.test(trimmed)) {
      flushList();
      flushTable();
      blocks.push({ type: 'hr' });
      continue;
    }

    // 6. Headings (#)
    const headingMatch = line.match(/^(#{1,6})\s+(.*)$/);
    if (headingMatch) {
      flushList();
      flushTable();
      blocks.push({
        type: 'heading',
        level: headingMatch[1].length,
        text: headingMatch[2].trim(),
      });
      continue;
    }

    // 7. Blockquote (>)
    if (line.startsWith('>')) {
      flushList();
      flushTable();
      blocks.push({
        type: 'blockquote',
        lines: [line.replace(/^>\s?/, '').trim()],
      });
      continue;
    }

    // 8. Unordered List (- or * or +)
    const ulMatch = line.match(/^(\s*)[-*+]\s+(.*)$/);
    if (ulMatch) {
      flushTable();
      if (!currentList || currentList.ordered) {
        flushList();
        currentList = { ordered: false, items: [] };
      }
      currentList.items.push(ulMatch[2].trim());
      continue;
    }

    // 9. Ordered List (1. 2.)
    const olMatch = line.match(/^(\s*)\d+\.\s+(.*)$/);
    if (olMatch) {
      flushTable();
      if (!currentList || !currentList.ordered) {
        flushList();
        currentList = { ordered: true, items: [] };
      }
      currentList.items.push(olMatch[2].trim());
      continue;
    }

    // Regular paragraph
    flushList();
    flushTable();
    blocks.push({
      type: 'paragraph',
      text: trimmed,
    });
  }

  flushList();
  flushTable();
  if (inCode) {
    blocks.push({
      type: 'code',
      lang: codeLang,
      code: codeLines.join('\n'),
    });
  }
  if (inMath && mathLines.length > 0) {
    blocks.push({
      type: 'math',
      formula: mathLines.join('\n').trim(),
    });
  }

  return blocks;
}

export function MarkdownContent({ content, className = '' }: MarkdownContentProps) {
  if (!content || !content.trim()) {
    return null;
  }

  const blocks = parseBlocks(content);

  return (
    <div className={`space-y-3.5 text-sm leading-relaxed text-slate-700 dark:text-slate-300 ${className}`}>
      {blocks.map((block, idx) => {
        const key = `block-${idx}`;

        switch (block.type) {
          case 'math': {
            const html = renderKaTeX(block.formula, true);
            return (
              <div
                key={key}
                className="my-4 overflow-x-auto rounded-2xl border border-indigo-200/50 bg-indigo-50/30 p-5 text-center shadow-sm dark:border-indigo-900/40 dark:bg-slate-950/60"
              >
                <div
                  className="inline-block max-w-full font-serif"
                  dangerouslySetInnerHTML={{ __html: html }}
                />
              </div>
            );
          }

          case 'heading': {
            const inlineNodes = renderInline(parseInline(block.text), `${key}-h`);
            if (block.level === 1) {
              return (
                <h1
                  key={key}
                  className="mt-6 mb-3 text-xl font-extrabold tracking-tight text-slate-900 dark:text-white"
                >
                  {inlineNodes}
                </h1>
              );
            }
            if (block.level === 2) {
              return (
                <h2
                  key={key}
                  className="mt-5 mb-2.5 pb-1 text-lg font-bold tracking-tight text-slate-900 dark:text-white border-b border-slate-200 dark:border-slate-800"
                >
                  {inlineNodes}
                </h2>
              );
            }
            if (block.level === 3) {
              return (
                <h3
                  key={key}
                  className="mt-4 mb-2 text-base font-bold tracking-tight text-slate-900 dark:text-white"
                >
                  {inlineNodes}
                </h3>
              );
            }
            return (
              <h4
                key={key}
                className="mt-3 mb-1.5 text-sm font-semibold tracking-tight text-slate-800 dark:text-slate-200"
              >
                {inlineNodes}
              </h4>
            );
          }

          case 'code':
            return (
              <div key={key} className="my-3 overflow-hidden rounded-xl border border-slate-800 bg-[#050811] shadow-sm">
                {block.lang && (
                  <div className="border-b border-slate-800/80 bg-slate-950/80 px-3.5 py-1 text-[11px] font-mono text-slate-400">
                    {block.lang}
                  </div>
                )}
                <pre className="overflow-x-auto p-4 font-mono text-xs leading-relaxed text-cyan-300">
                  <code>{block.code}</code>
                </pre>
              </div>
            );

          case 'list': {
            const ListTag = block.ordered ? 'ol' : 'ul';
            return (
              <ListTag
                key={key}
                className={`my-2.5 space-y-1.5 pl-5 ${
                  block.ordered ? 'list-decimal' : 'list-disc'
                } text-slate-700 dark:text-slate-300 marker:text-indigo-500`}
              >
                {block.items.map((item, itemIdx) => (
                  <li key={`${key}-item-${itemIdx}`} className="leading-relaxed">
                    {renderInline(parseInline(item), `${key}-li-${itemIdx}`)}
                  </li>
                ))}
              </ListTag>
            );
          }

          case 'blockquote':
            return (
              <blockquote
                key={key}
                className="my-3 rounded-r-xl border-l-4 border-indigo-500 bg-slate-50 py-2.5 px-4 italic text-slate-700 dark:bg-slate-950/40 dark:text-slate-300"
              >
                {block.lines.map((l, lIdx) => (
                  <p key={`${key}-q-${lIdx}`}>
                    {renderInline(parseInline(l), `${key}-qp-${lIdx}`)}
                  </p>
                ))}
              </blockquote>
            );

          case 'hr':
            return <hr key={key} className="my-5 border-slate-200 dark:border-slate-800" />;

          case 'table':
            return (
              <div key={key} className="my-4 overflow-x-auto rounded-xl border border-slate-200 dark:border-slate-800">
                <table className="w-full border-collapse text-left text-xs">
                  <thead>
                    <tr className="border-b border-slate-200 bg-slate-50 font-semibold text-slate-800 dark:border-slate-800 dark:bg-slate-950/80 dark:text-slate-200">
                      {block.headers.map((h, hIdx) => (
                        <th key={`${key}-th-${hIdx}`} className="px-3.5 py-2.5">
                          {renderInline(parseInline(h), `${key}-th-in-${hIdx}`)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                    {block.rows.map((row, rIdx) => (
                      <tr
                        key={`${key}-tr-${rIdx}`}
                        className="hover:bg-slate-50/50 dark:hover:bg-slate-900/40 transition-colors"
                      >
                        {row.map((cell, cIdx) => (
                          <td key={`${key}-td-${rIdx}-${cIdx}`} className="px-3.5 py-2">
                            {renderInline(parseInline(cell), `${key}-td-in-${rIdx}-${cIdx}`)}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            );

          case 'paragraph':
          default:
            return (
              <p key={key} className="text-sm leading-relaxed text-slate-700 dark:text-slate-300">
                {renderInline(parseInline(block.text), `${key}-p`)}
              </p>
            );
        }
      })}
    </div>
  );
}
