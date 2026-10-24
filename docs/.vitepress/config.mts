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
    ignoreDeadLinks: true,
    sitemap: { hostname: 'https://openagentpay.dev' },
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
                text: 'Chapter 1: Agent Basics',
                collapsed: false,
                items: [
                  { text: 'What is an Agent', link: '/learn/01-agent-basics/what-is-an-agent' },
                  { text: 'Tool Calling', link: '/learn/01-agent-basics/tool-calling' },
                  { text: 'MCP Standard', link: '/learn/01-agent-basics/mcp' },
                  { text: 'Agent Frameworks', link: '/learn/01-agent-basics/agent-frameworks' },
                  { text: 'Agent Failure Modes', link: '/learn/01-agent-basics/agent-failure-modes' },
                  { text: 'Autonomy Levels', link: '/learn/01-agent-basics/autonomy-levels' },
                ],
              },
              {
                text: 'Chapter 2: Agent Payments',
                collapsed: false,
                items: [
                  { text: 'Why Agents Pay', link: '/learn/02-agent-payments/why-agents-pay' },
                  { text: 'Payment Options', link: '/learn/02-agent-payments/payment-options' },
                  { text: 'HTTP 402 & x402', link: '/learn/02-agent-payments/http-402-and-x402' },
                  { text: 'Stablecoins & USDC', link: '/learn/02-agent-payments/stablecoins-and-usdc' },
                  { text: 'Gasless Auth (EIP-3009)', link: '/learn/02-agent-payments/gasless-eip-3009' },
                  { text: 'Micropayment Economics', link: '/learn/02-agent-payments/micropayment-economics' },
                  { text: 'Five Trust Problems', link: '/learn/02-agent-payments/trust-problems' },
                  { text: 'Ecosystem Landscape', link: '/learn/02-agent-payments/ecosystem-landscape' },
                ],
              },
              {
                text: 'Chapter 3: Infrastructure Map',
                collapsed: false,
                items: [
                  { text: 'Agent Stack Layers', link: '/learn/03-infra-map/agent-stack' },
                  { text: 'Where AgentPay Fits', link: '/learn/03-infra-map/where-agentpay-fits' },
                  { text: 'AgentPay vs x402', link: '/learn/03-infra-map/agentpay-vs-x402' },
                ],
              },
              {
                text: 'Chapter 4: AgentPay Concepts',
                collapsed: false,
                items: [
                  { text: 'Financial Firewall', link: '/learn/04-agentpay-concepts/financial-firewall' },
                  { text: 'Payment Lifecycle', link: '/learn/04-agentpay-concepts/payment-lifecycle' },
                  { text: 'Timeout Is Not Failure', link: '/learn/04-agentpay-concepts/timeout-is-not-failure' },
                  { text: 'Three-State Model', link: '/learn/04-agentpay-concepts/three-state-model' },
                  { text: 'Budget Reservation', link: '/learn/04-agentpay-concepts/budget-reservation' },
                  { text: 'Dual-Rail Architecture', link: '/learn/04-agentpay-concepts/dual-rail' },
                  { text: 'Agent Identity', link: '/learn/04-agentpay-concepts/agent-identity' },
                  { text: 'Idempotency', link: '/learn/04-agentpay-concepts/idempotency' },
                  { text: 'Multi-Tenant Model', link: '/learn/04-agentpay-concepts/multi-tenant' },
                  { text: 'Threat Model Lite', link: '/learn/04-agentpay-concepts/threat-model-lite' },
                ],
              },
              {
                text: 'Chapter 5: Hands-On',
                collapsed: false,
                items: [
                  { text: '10-Minute Demo', link: '/learn/05-hands-on/ten-minute-demo' },
                  { text: 'First Paid Route', link: '/learn/05-hands-on/first-paid-route' },
                  { text: 'Policy in Action', link: '/learn/05-hands-on/policy-in-action' },
                  { text: 'Read Payment Trace', link: '/learn/05-hands-on/read-payment-trace' },
                  { text: 'Audit Timeline & Kill Switch', link: '/learn/05-hands-on/audit-timeline' },
                  { text: 'Draft, Publish & Rollback', link: '/learn/05-hands-on/draft-publish-rollback' },
                ],
              },
              {
                text: 'Chapter 6: Resources',
                collapsed: false,
                items: [
                  { text: 'Glossary', link: '/learn/06-resources/glossary' },
                  { text: 'ID Prefixes', link: '/learn/06-resources/id-prefixes' },
                  { text: 'Safety Redlines', link: '/learn/06-resources/safety-redlines' },
                  { text: 'FAQ', link: '/learn/06-resources/faq' },
                  { text: 'Further Reading', link: '/learn/06-resources/further-reading' },
                ],
              },
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
              { text: '学习地图', link: '/zh/learn/' },
              {
                text: '第 1 章：Agent 开发基础',
                collapsed: false,
                items: [
                  { text: '从聊天到执行任务：Agent 与 LLM 的区别', link: '/zh/learn/01-agent-basics/what-is-an-agent' },
                  { text: '工具调用：Agent 如何接触外部世界', link: '/zh/learn/01-agent-basics/tool-calling' },
                  { text: 'MCP：工具的标准接口', link: '/zh/learn/01-agent-basics/mcp' },
                  { text: 'LangChain / CrewAI 等框架的位置', link: '/zh/learn/01-agent-basics/agent-frameworks' },
                  { text: '幻觉、提示词注入、无限循环、重试风暴', link: '/zh/learn/01-agent-basics/agent-failure-modes' },
                  { text: '自主度分级：辅助 / 半自主 / 全自主', link: '/zh/learn/01-agent-basics/autonomy-levels' },
                ],
              },
              {
                text: '第 2 章：Agent 支付知识',
                collapsed: false,
                items: [
                  { text: 'Agent 为什么需要花钱', link: '/zh/learn/02-agent-payments/why-agents-pay' },
                  { text: '现有方式对比', link: '/zh/learn/02-agent-payments/payment-options' },
                  { text: 'HTTP 402 与 x402 协议原理', link: '/zh/learn/02-agent-payments/http-402-and-x402' },
                  { text: '稳定币基础，为什么选 USDC', link: '/zh/learn/02-agent-payments/stablecoins-and-usdc' },
                  { text: 'EIP-712 / EIP-3009 免 Gas 授权', link: '/zh/learn/02-agent-payments/gasless-eip-3009' },
                  { text: '微支付的成本问题', link: '/zh/learn/02-agent-payments/micropayment-economics' },
                  { text: 'Agent 支付的五个信任问题', link: '/zh/learn/02-agent-payments/trust-problems' },
                  { text: '其他 Agent 支付方案生态', link: '/zh/learn/02-agent-payments/ecosystem-landscape' },
                ],
              },
              {
                text: '第 3 章：Agent 基础设施全景',
                collapsed: false,
                items: [
                  { text: '分层图：技术栈全景', link: '/zh/learn/03-infra-map/agent-stack' },
                  { text: 'AgentPay 在哪一层，不做什么', link: '/zh/learn/03-infra-map/where-agentpay-fits' },
                  { text: 'AgentPay vs x402', link: '/zh/learn/03-infra-map/agentpay-vs-x402' },
                ],
              },
              {
                text: '第 4 章：AgentPay 核心概念',
                collapsed: false,
                items: [
                  { text: '为什么需要金融防火墙', link: '/zh/learn/04-agentpay-concepts/financial-firewall' },
                  { text: '一次支付的完整生命周期', link: '/zh/learn/04-agentpay-concepts/payment-lifecycle' },
                  { text: '超时不等于失败（双花故事）', link: '/zh/learn/04-agentpay-concepts/timeout-is-not-failure' },
                  { text: '意图 / 结算 / 履约为什么要拆开', link: '/zh/learn/04-agentpay-concepts/three-state-model' },
                  { text: '预算预留 vs 先检查后支付', link: '/zh/learn/04-agentpay-concepts/budget-reservation' },
                  { text: '虚拟钱包 vs 链上（双轨）', link: '/zh/learn/04-agentpay-concepts/dual-rail' },
                  { text: 'Agent 身份：观测 vs 自报', link: '/zh/learn/04-agentpay-concepts/agent-identity' },
                  { text: '幂等与免费重放', link: '/zh/learn/04-agentpay-concepts/idempotency' },
                  { text: '租户 / 项目 / Agent / API Key', link: '/zh/learn/04-agentpay-concepts/multi-tenant' },
                  { text: '通俗版威胁模型', link: '/zh/learn/04-agentpay-concepts/threat-model-lite' },
                ],
              },
              {
                text: '第 5 章：动手实践',
                collapsed: false,
                items: [
                  { text: '10 分钟 Demo', link: '/zh/learn/05-hands-on/ten-minute-demo' },
                  { text: '控制台配置第一个付费路由', link: '/zh/learn/05-hands-on/first-paid-route' },
                  { text: '配置策略并观察拦截', link: '/zh/learn/05-hands-on/policy-in-action' },
                  { text: '读懂一笔支付的全链路', link: '/zh/learn/05-hands-on/read-payment-trace' },
                  { text: '审计时间线与 Kill Switch', link: '/zh/learn/05-hands-on/audit-timeline' },
                  { text: '草稿、发布与回滚', link: '/zh/learn/05-hands-on/draft-publish-rollback' },
                ],
              },
              {
                text: '第 6 章：速查资料',
                collapsed: false,
                items: [
                  { text: '术语表', link: '/zh/learn/06-resources/glossary' },
                  { text: 'ID 前缀速查', link: '/zh/learn/06-resources/id-prefixes' },
                  { text: '安全红线一页纸', link: '/zh/learn/06-resources/safety-redlines' },
                  { text: '常见问题 (FAQ)', link: '/zh/learn/06-resources/faq' },
                  { text: '延伸阅读', link: '/zh/learn/06-resources/further-reading' },
                ],
              },
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
