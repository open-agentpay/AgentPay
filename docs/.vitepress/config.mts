import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

// GitHub Pages 项目站点需要 DOCS_BASE=/AgentPay/，自定义域名则留空
const base = process.env.DOCS_BASE ?? '/'

export default withMermaid(
  defineConfig({
    lang: 'zh-CN',
    title: 'AgentPay',
    description: 'AI Agent 的金融防火墙与自主支付网关（实验性开源项目）',
    base,
    cleanUrls: true,
    lastUpdated: true,
    sitemap: { hostname: 'https://open-agentpay.github.io/AgentPay' },
    head: [['meta', { name: 'theme-color', content: '#3b82f6' }]],
    vite: {
          optimizeDeps: {
            include: ['mermaid', 'fastdom'],
          },
        },
    themeConfig: {
      nav: [
        { text: '文档', link: '/guide/quickstart', activeMatch: '/guide/' },
        { text: '规范', link: '/specs/', activeMatch: '/specs/' },
        { text: '知识库', link: '/learn/', activeMatch: '/learn/' },
        { text: '参与贡献', link: '/contributing/good-first-issues', activeMatch: '/contributing/' },
      ],

      sidebar: {
        '/guide/': [
          {
            text: '开始',
            items: [
              { text: '概览', link: '/guide/overview' },
              { text: '5 分钟快速开始', link: '/guide/quickstart' },
            ],
          },
          {
            text: '使用',
            items: [
              { text: 'Python SDK', link: '/guide/python-sdk' },
              { text: '网关配置', link: '/guide/gateway' },
              { text: '策略配置', link: '/guide/policy' },
              { text: '管理控制台', link: '/guide/console' },
            ],
          },
          {
            text: '参考',
            items: [
              { text: '接口规格 (x402 / Facilitator)', link: '/guide/api-specs' },
              { text: '部署', link: '/guide/deployment' },
            ],
          },
        ],

        '/specs/': [
          { text: '规范总览', link: '/specs/' },
          {
            text: '核心契约',
            items: [
              { text: '支付状态机', link: '/specs/state-machine' },
              { text: '钱包与账本', link: '/specs/wallet-ledger' },
              { text: '协议一致性', link: '/specs/protocol-consistency' },
              { text: '配置发布', link: '/specs/config-publishing' },
              { text: '审计事件', link: '/specs/audit-events' },
              { text: 'Agent 身份与行为审计', link: '/specs/agent-audit' },
              { text: '多租户数据模型', link: '/specs/multitenant-data-model' },
            ],
          },
          {
            text: '架构与安全',
            items: [
              { text: '架构', link: '/specs/architecture' },
              { text: 'Gas 与结算模型', link: '/specs/gas-models' },
              { text: '安全模型', link: '/specs/security-model' },
            ],
          },
          {
            text: '架构决策 (ADR)',
            items: [
              { text: 'ADR-001 x402 复用策略', link: '/specs/adr/001' },
              { text: 'ADR-002 PostgreSQL 选型', link: '/specs/adr/002' },
            ],
          },
          {
            text: '测试报告',
            items: [
              { text: 'Python/Go 协议一致性', link: '/specs/reports/protocol-conformance-report' },
              { text: '多租户隔离与越权', link: '/specs/reports/tenant-isolation-test-report' },
            ],
          },
        ],

        '/learn/': [
          { text: '知识库首页', link: '/learn/' },
          {
            text: '基础概念',
            collapsed: false,
            items: [
              { text: 'Agent 为什么需要花钱', link: '/learn/fundamentals/why-agents-pay' },
              { text: 'Tool Calling 与 MCP', link: '/learn/fundamentals/tools-and-mcp' },
            ],
          },
          {
            text: 'Agent 支付',
            collapsed: false,
            items: [
              { text: 'HTTP 402 与 x402', link: '/learn/payments/x402' },
              { text: '链上支付 vs 预付余额', link: '/learn/payments/onchain-vs-wallet' },
              { text: 'Gasless 微支付与 Gas 模型', link: '/learn/payments/gasless' },
              { text: '超时不等于失败', link: '/learn/payments/timeout-is-not-failure' },
            ],
          },
          {
            text: 'Agent 资金安全',
            collapsed: false,
            items: [
              { text: '为什么 LLM 不能持有私钥', link: '/learn/safety/llm-and-keys' },
              { text: '金融防火墙', link: '/learn/safety/financial-firewall' },
              { text: '失控循环与熔断', link: '/learn/safety/runaway-loop' },
              { text: 'Agent 身份与行为审计', link: '/learn/safety/agent-identity-audit' },
            ],
          },
          { text: '术语表', link: '/learn/glossary' },
        ],
      },

      socialLinks: [{ icon: 'github', link: 'https://github.com/open-agentpay/AgentPay' }],
      search: { provider: 'local' },
      outline: { label: '本页目录', level: [2, 3] },
      docFooter: { prev: '上一页', next: '下一页' },
      lastUpdated: { text: '最后更新' },
      footer: {
        message: '实验性项目，未经独立安全审计，请勿用于生产资金。',
        copyright: 'Apache-2.0 License',
      },
    },

    mermaid: {},
  })
)
