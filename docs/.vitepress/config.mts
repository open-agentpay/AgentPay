import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'AgentPay',
  description: 'AI Agent 的金融防火墙与自主支付基础设施 (Financial Firewall & Autonomous Payment Infrastructure)',
  lang: 'zh-CN',
  base: '/',
  lastUpdated: true,
  cleanUrls: true,

  head: [
    ['link', { rel: 'icon', href: '/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#3eaf7c' }],
  ],

  themeConfig: {
    siteTitle: 'AgentPay Docs',
    logo: '/logo.svg',

    nav: [
      { text: '文档首页', link: '/' },
      { text: '架构与指南', link: '/guide/overview' },
      { text: '核心规范 (Specs)', link: '/specs/state-machine' },
      { text: '虚拟钱包', link: '/specs/wallet-ledger' },
      { text: '研发任务与基线', link: '/planning/tasks-v1.1' },
      {
        text: '代码仓库',
        link: 'https://github.com/open-agentpay/AgentPay',
      },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '基础概览',
          items: [
            { text: '项目愿景与产品定位', link: '/guide/overview' },
            { text: 'MVP 系统设计', link: '/guide/mvp-design' },
            { text: 'API 接口规范', link: '/guide/api-specs' },
          ],
        },
      ],
      '/specs/': [
        {
          text: '技术规范 (Specs)',
          items: [
            { text: '双轨支付状态机 (T1.1)', link: '/specs/state-machine' },
            { text: '多租户逻辑数据模型 (T1.2)', link: '/specs/multitenant-data-model' },
            { text: '协议一致性契约 (T1.3)', link: '/specs/protocol-consistency' },
            { text: '配置发布契约 (T1.4)', link: '/specs/config-publishing' },
            { text: '消费审计事件契约 (T1.5)', link: '/specs/audit-events' },
            { text: '全面威胁模型规范 (T1.6)', link: '/specs/threat-model-v1' },
            { text: '虚拟钱包与账本规范 (T1.7)', link: '/specs/wallet-ledger' },
            { text: '系统整体架构设计', link: '/specs/architecture' },
          ],
        },
        {
          text: '架构决策记录 (ADR)',
          items: [
            { text: 'ADR-001: x402 V2 协议选型', link: '/specs/adr-001' },
            { text: 'ADR-002: PostgreSQL 数据库选型', link: '/specs/adr-002' },
          ],
        },
      ],
      '/planning/': [
        {
          text: '研发规划与基线',
          items: [
            { text: '研发任务总清单 (v1.1)', link: '/planning/tasks-v1.1' },
            { text: 'T0.2 行为基线冻结记录', link: '/planning/baseline-freeze' },
            { text: '2026 年底研发计划', link: '/planning/development-plan-2026' },
            { text: '开源新手任务 (Good First Issues)', link: '/planning/good-first-issues' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/open-agentpay/AgentPay' },
    ],

    footer: {
      message: 'Released under the Apache-2.0 License.',
      copyright: 'Copyright © 2026 Open-AgentPay Team',
    },

    search: {
      provider: 'local',
      options: {
        locales: {
          root: {
            translations: {
              button: {
                buttonText: '搜索文档',
                buttonAriaLabel: '搜索文档',
              },
              modal: {
                noResultsText: '无法找到相关结果',
                resetButtonTitle: '清除查询条件',
                footer: {
                  selectText: '选择',
                  navigateText: '切换',
                  closeText: '关闭',
                },
              },
            },
          },
        },
      },
    },
  },
})
