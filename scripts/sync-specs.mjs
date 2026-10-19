#!/usr/bin/env node
// 将仓库根目录 specs/*.md（唯一事实来源）同步到 docs/specs/ 与 docs/zh/specs/，并：
//  1) 按 MAP 改成站点友好的文件名
//  2) 重写链接：规范之间 -> 站内相对链接；JSON/DDL/测试向量等 -> GitHub 链接
//  3) 清理 file:///Users/... 本机绝对路径
// 同步产物不进 Git（见 .gitignore），手写页面（index.md、reports/）不受影响。
import { readFile, writeFile, mkdir, readdir, rm } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const SRC = path.join(ROOT, 'specs')
const OUT_DIRS = [
  path.join(ROOT, 'docs', 'specs'),
  path.join(ROOT, 'docs', 'zh', 'specs'),
]
const REPO = 'https://github.com/open-agentpay/AgentPay'
const BRANCH = 'main'

// 源文件名 -> docs/specs 下的输出路径。按你仓库里的实际文件名调整。
const MAP = {
  'payment-state-machine-v2.md': 'state-machine.md',
  'wallet-ledger-spec.md': 'wallet-ledger.md',
  'protocol-consistency-spec.md': 'protocol-consistency.md',
  'config-publishing-spec.md': 'config-publishing.md',
  'audit-events-spec.md': 'audit-events.md',
  'agent-audit-spec.md': 'agent-audit.md',
  'multitenant-data-model.md': 'multitenant-data-model.md',
  'architecture.md': 'architecture.md',
  'security-model.md': 'security-model.md',
  'gas-cost-and-settlement-models.md': 'gas-models.md',
  // 'threat-model-v1.md': 'threat-model.md',
}

// ADR 自动识别：adr-001-xxx.md -> adr/001.md
const srcFiles = existsSync(SRC) ? await readdir(SRC) : []
for (const f of srcFiles) {
  const m = f.match(/^adr-(\d+).*\.md$/)
  if (m) MAP[f] = `adr/${m[1]}.md`
}

// 链接重写映射表（包含简写别名）
const LINK_MAP = { ...MAP }
for (const f of srcFiles) {
  const m = f.match(/^adr-(\d+).*\.md$/)
  if (m) {
    LINK_MAP[`adr-${m[1]}.md`] = `adr/${m[1]}.md`
    LINK_MAP[`adr-${m[1]}`] = `adr/${m[1]}.md`
  }
}

const warnings = []
const rel = (p) => path.relative(ROOT, p).split(path.sep).join('/')

function rewriteLinks(text, srcName, outRel, outDirBase) {
  const outDir = path.dirname(path.join(outDirBase, outRel))
  return text.replace(/(!?\[[^\]]*\]\()([^)\s]+)(\))/g, (all, pre, target, post) => {
    if (/^(https?:|mailto:|#)/.test(target)) return all
    let [pathPart, hash = ''] = target.split('#')
    hash = hash ? '#' + hash : ''

    // file:///Users/xxx/.../AgentPay/specs/foo.md -> 仓库相对路径
    pathPart = pathPart.replace(/^file:\/\/\/.*?\/AgentPay\//, '')
    const base = path.basename(pathPart)

    // 1) 指向另一份已同步的规范 -> 站内相对链接
    if (LINK_MAP[base] && !/\.json$/.test(base)) {
      const targetAbs = path.join(outDirBase, LINK_MAP[base])
      let r = path.relative(outDir, targetAbs).split(path.sep).join('/')
      if (!r.startsWith('.')) r = './' + r
      return `${pre}${r}${hash}${post}`
    }

    // 2) 其它仓库文件 -> GitHub 链接（依次尝试不同基准目录）
    const candidates = [
      path.resolve(SRC, pathPart),
      path.resolve(outDirBase, pathPart),
      path.resolve(ROOT, pathPart),
    ]
    const hit = candidates.find((c) => c.startsWith(ROOT) && existsSync(c))
    if (hit) {
      const kind = !path.extname(hit) ? 'tree' : 'blob'
      return `${pre}${REPO}/${kind}/${BRANCH}/${rel(hit)}${hash}${post}`
    }
    warnings.push(`${srcName}: 无法解析链接 ${target}`)
    return all
  })
}

for (const outDir of OUT_DIRS) {
  // 清理上次同步产物（只删 MAP 里的输出，不碰 index.md / reports/）
  for (const out of Object.values(MAP)) await rm(path.join(outDir, out), { force: true })

  let ok = 0
  for (const [srcName, outRel] of Object.entries(MAP)) {
    const srcPath = path.join(SRC, srcName)
    if (!existsSync(srcPath)) {
      warnings.push(`缺少源文件 specs/${srcName}（跳过）`)
      continue
    }
    let text = await readFile(srcPath, 'utf8')
    text = rewriteLinks(text, srcName, outRel, outDir)
    text = `<!-- 本文件由 scripts/sync-specs.mjs 从 specs/${srcName} 自动生成，请勿直接编辑 -->\n\n${text}`
    const outPath = path.join(outDir, outRel)
    await mkdir(path.dirname(outPath), { recursive: true })
    await writeFile(outPath, text)
    ok++
  }
  const relOut = path.relative(ROOT, outDir).split(path.sep).join('/')
  console.log(`同步完成：${ok}/${Object.keys(MAP).length} 个规范 -> ${relOut}/`)
}

if (warnings.length) {
  console.warn('\n需要处理的问题：')
  for (const w of warnings) console.warn('  - ' + w)
}

// 若仍残留本机路径，直接失败，避免发布
const leaked = []
for (const outDir of OUT_DIRS) {
  for (const out of Object.values(MAP)) {
    const p = path.join(outDir, out)
    if (existsSync(p) && /file:\/\/\/Users|\/Users\/[a-z]+\//i.test(await readFile(p, 'utf8'))) {
      leaked.push(path.relative(ROOT, p))
    }
  }
}
if (leaked.length) {
  console.error('\n以下文件仍含本机路径：\n  ' + leaked.join('\n  '))
  process.exit(1)
}
