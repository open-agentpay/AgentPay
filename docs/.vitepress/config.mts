import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

// GitHub Pages 项目站点需要 DOCS_BASE=/AgentPay/，自定义域名则留空
const base = process.env.DOCS_BASE ?? '/'

export default withMermaid(
  defineConfig({
    title: 'AgentPay',
    base,
    cleanUrls: true,
    lastUpdated: true,
    sitemap: { hostname: 'https://open-agentpay.github.io/AgentPay' },
    head: [
      ['meta', { name: 'theme-color', content: '#f97316' }],
      ['link', { rel: 'icon', type: 'image/png', href: `${base}favicon.png` }],
      ['link', { rel: 'icon', type: 'image/x-icon', href: `${base}favicon.ico` }],
      ['meta', { property: 'og:image', content: 'https://open-agentpay.github.io/AgentPay/og-image.png' }],
      ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
      ['meta', { name: 'twitter:image', content: 'https://open-agentpay.github.io/AgentPay/og-image.png' }],
    ],
    vite: {
      optimizeDeps: {
        include: ['mermaid', 'fastdom'],
      },
    },

    locales: {
      root: {
        label: 'English',
        lang: 'en-US',
        description: 'Financial Firewall & Autonomous Payment Gateway for AI Agents (Experimental Open-Source)',
        themeConfig: {
          nav: [
            { text: 'Guide', link: '/guide/quickstart', activeMatch: '/guide/' },
            { text: 'Specs', link: '/specs/', activeMatch: '/specs/' },
            { text: 'Learn', link: '/learn/', activeMatch: '/learn/' },
            { text: 'Contributing', link: '/contributing/good-first-issues', activeMatch: '/contributing/' },
          ],

          sidebar: {
            '/guide/': [
              {
                text: 'Getting Started',
                items: [
                  { text: 'Overview', link: '/guide/overview' },
                  { text: '5-Minute Quickstart', link: '/guide/quickstart' },
                ],
              },
              {
                text: 'Usage & Integration',
                items: [
                  { text: 'Python SDK', link: '/guide/python-sdk' },
                  { text: 'Gateway Configuration', link: '/guide/gateway' },
                  { text: 'Policy Configuration', link: '/guide/policy' },
                  { text: 'Management Console', link: '/guide/console' },
                ],
              },
              {
                text: 'Reference',
                items: [
                  { text: 'API Specs (x402 / Facilitator)', link: '/guide/api-specs' },
                  { text: 'Deployment', link: '/guide/deployment' },
                ],
              },
            ],

            '/specs/': [
              { text: 'Specifications Overview', link: '/specs/' },
              {
                text: 'Core Contracts',
                items: [
                  { text: 'Payment State Machine', link: '/specs/state-machine' },
                  { text: 'Wallet & Ledger', link: '/specs/wallet-ledger' },
                  { text: 'Protocol Consistency', link: '/specs/protocol-consistency' },
                  { text: 'Config Publishing', link: '/specs/config-publishing' },
                  { text: 'Audit Events', link: '/specs/audit-events' },
                  { text: 'Agent Identity & Behavior Audit', link: '/specs/agent-audit' },
                  { text: 'Multitenant Data Model', link: '/specs/multitenant-data-model' },
                ],
              },
              {
                text: 'Architecture & Security',
                items: [
                  { text: 'Architecture Specification', link: '/specs/architecture' },
                  { text: 'Gas & Settlement Models', link: '/specs/gas-models' },
                  { text: 'Security Model', link: '/specs/security-model' },
                ],
              },
              {
                text: 'Architecture Decisions (ADR)',
                items: [
                  { text: 'ADR-001 x402 Reuse Strategy', link: '/specs/adr/001' },
                  { text: 'ADR-002 PostgreSQL Selection', link: '/specs/adr/002' },
                ],
              },
              {
                text: 'Conformance Reports',
                items: [
                  { text: 'Python/Go Conformance Report', link: '/specs/reports/protocol-conformance-report' },
                  { text: 'Tenant Isolation Report', link: '/specs/reports/tenant-isolation-test-report' },
                ],
              },
            ],

            '/learn/': [
              { text: 'Knowledge Base Home', link: '/learn/' },
              {
                text: 'Fundamentals',
                collapsed: false,
                items: [
                  { text: 'Why Agents Need to Pay', link: '/learn/fundamentals/why-agents-pay' },
                  { text: 'Tool Calling & MCP', link: '/learn/fundamentals/tools-and-mcp' },
                ],
              },
              {
                text: 'Agent Payments',
                collapsed: false,
                items: [
                  { text: 'HTTP 402 & x402', link: '/learn/payments/x402' },
                  { text: 'On-Chain vs Virtual Wallet', link: '/learn/payments/onchain-vs-wallet' },
                  { text: 'Gasless Micro-Payments', link: '/learn/payments/gasless' },
                  { text: 'Timeout Is Not Failure', link: '/learn/payments/timeout-is-not-failure' },
                ],
              },
              {
                text: 'Agent Safety',
                collapsed: false,
                items: [
                  { text: 'Why LLMs Must Never Hold Keys', link: '/learn/safety/llm-and-keys' },
                  { text: 'Financial Firewall Mechanism', link: '/learn/safety/financial-firewall' },
                  { text: 'Runaway Loops & Circuit Breakers', link: '/learn/safety/runaway-loop' },
                  { text: 'Agent Identity & Behavior Audit', link: '/learn/safety/agent-identity-audit' },
                ],
              },
              { text: 'Glossary', link: '/learn/glossary' },
            ],
          },

          outline: { label: 'On this page', level: [2, 3] },
          docFooter: { prev: 'Previous Page', next: 'Next Page' },
          lastUpdated: { text: 'Last Updated' },
          footer: {
            message: 'Experimental open-source project. Not audited for production fund handling.',
            copyright: 'Apache-2.0 License',
          },
        },
      },

      zh: {
        label: '简体中文',
        lang: 'zh-CN',
        link: '/zh/',
        description: 'AI Agent 的金融防火墙与自主支付网关（实验性开源项目）',
        themeConfig: {
          nav: [
            { text: '文档', link: '/zh/guide/quickstart', activeMatch: '/zh/guide/' },
            { text: '规范', link: '/zh/specs/', activeMatch: '/zh/specs/' },
            { text: '知识库', link: '/zh/learn/', activeMatch: '/zh/learn/' },
            { text: '参与贡献', link: '/zh/contributing/good-first-issues', activeMatch: '/zh/contributing/' },
          ],

          sidebar: {
            '/zh/guide/': [
              {
                text: '开始',
                items: [
                  { text: '概览', link: '/zh/guide/overview' },
                  { text: '5 分钟快速开始', link: '/zh/guide/quickstart' },
                ],
              },
              {
                text: '使用',
                items: [
                  { text: 'Python SDK', link: '/zh/guide/python-sdk' },
                  { text: '网关配置', link: '/zh/guide/gateway' },
                  { text: '策略配置', link: '/zh/guide/policy' },
                  { text: '管理控制台', link: '/zh/guide/console' },
                ],
              },
              {
                text: '参考',
                items: [
                  { text: '接口规格 (x402 / Facilitator)', link: '/zh/guide/api-specs' },
                  { text: '部署', link: '/zh/guide/deployment' },
                ],
              },
            ],

            '/zh/specs/': [
              { text: '规范总览', link: '/zh/specs/' },
              {
                text: '核心契约',
                items: [
                  { text: '支付状态机', link: '/zh/specs/state-machine' },
                  { text: '钱包与账本', link: '/zh/specs/wallet-ledger' },
                  { text: '协议一致性', link: '/zh/specs/protocol-consistency' },
                  { text: '配置发布', link: '/zh/specs/config-publishing' },
                  { text: '审计事件', link: '/zh/specs/audit-events' },
                  { text: 'Agent 身份与行为审计', link: '/zh/specs/agent-audit' },
                  { text: '多租户数据模型', link: '/zh/specs/multitenant-data-model' },
                ],
              },
              {
                text: '架构与安全',
                items: [
                  { text: '架构', link: '/zh/specs/architecture' },
                  { text: 'Gas 与结算模型', link: '/zh/specs/gas-models' },
                  { text: '安全模型', link: '/zh/specs/security-model' },
                ],
              },
              {
                text: '架构决策 (ADR)',
                items: [
                  { text: 'ADR-001 x402 复用策略', link: '/zh/specs/adr/001' },
                  { text: 'ADR-002 PostgreSQL 选型', link: '/zh/specs/adr/002' },
                ],
              },
              {
                text: '测试报告',
                items: [
                  { text: 'Python/Go 协议一致性', link: '/zh/specs/reports/protocol-conformance-report' },
                  { text: '多租户隔离与越权', link: '/zh/specs/reports/tenant-isolation-test-report' },
                ],
              },
            ],

            '/zh/learn/': [
              { text: '知识库首页', link: '/zh/learn/' },
              {
                text: '基础概念',
                collapsed: false,
                items: [
                  { text: 'Agent 为什么需要花钱', link: '/zh/learn/fundamentals/why-agents-pay' },
                  { text: 'Tool Calling 与 MCP', link: '/zh/learn/fundamentals/tools-and-mcp' },
                ],
              },
              {
                text: 'Agent 支付',
                collapsed: false,
                items: [
                  { text: 'HTTP 402 与 x402', link: '/zh/learn/payments/x402' },
                  { text: '链上支付 vs 预付余额', link: '/zh/learn/payments/onchain-vs-wallet' },
                  { text: 'Gasless 微支付与 Gas 模型', link: '/zh/learn/payments/gasless' },
                  { text: '超时不等于失败', link: '/zh/learn/payments/timeout-is-not-failure' },
                ],
              },
              {
                text: 'Agent 资金安全',
                collapsed: false,
                items: [
                  { text: '为什么 LLM 不能持有私钥', link: '/zh/learn/safety/llm-and-keys' },
                  { text: '金融防火墙', link: '/zh/learn/safety/financial-firewall' },
                  { text: '失控循环与熔断', link: '/zh/learn/safety/runaway-loop' },
                  { text: 'Agent 身份与行为审计', link: '/zh/learn/safety/agent-identity-audit' },
                ],
              },
              { text: '术语表', link: '/zh/learn/glossary' },
            ],
          },

          outline: { label: '本页目录', level: [2, 3] },
          docFooter: { prev: '上一页', next: '下一页' },
          lastUpdated: { text: '最后更新' },
          footer: {
            message: '实验性项目，未经独立安全审计，请勿用于生产资金。',
            copyright: 'Apache-2.0 License',
          },
        },
      },
    },

    themeConfig: {
      logo: '/logo.png',
      socialLinks: [
        { icon: 'github', link: 'https://github.com/open-agentpay/AgentPay' },
        { icon: 'x', link: 'https://x.com/OpenAgentPay' },
      ],
      search: { provider: 'local' },
    },

    mermaid: {},
  })
)
