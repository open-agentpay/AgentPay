import type { MarketplaceRouteItem } from '@/types/marketplace';

export const MARKETPLACE_CATEGORIES = [
  { id: 'all', label: '全部 API (All)', countBadge: true },
  { id: 'AI & LLM', label: 'AI 推理与多模态' },
  { id: 'Financial & Market Intel', label: '金融行情与研报' },
  { id: 'Web Scraping & Search', label: '网页抓取与实时搜索' },
  { id: 'Compute & Developer Tools', label: '沙箱算力与开发工具' },
  { id: 'Enterprise Finance & Compliance', label: '企业财务与合规审计' },
] as const;

export const SEED_MARKETPLACE_ROUTES: MarketplaceRouteItem[] = [
  {
    route_id: 'rte_deepseek_v3_chat_00',
    tenant_id: 'ten_deepseek_official',
    provider_name: 'DeepSeek (深度求索)',
    slug: 'deepseek-v3-chat-api',
    title: 'DeepSeek-V3 高性能大模型对话接口 (OpenAI 兼容)',
    summary:
      '官方直通 DeepSeek-V3 (671B MoE) 生产级对话大模型，全面兼容 OpenAI 协议。毫秒级 SSE 流式返回，按产生 Token 实收（输入 0.10 分/1k，输出 0.20 分/1k），支持流式断流预算保护与即充即用。',
    category: 'AI & LLM',
    tags: ['DeepSeek-V3', 'LLM', 'OpenAI兼容', '流式计费', 'MoE'],
    cluster: 'domestic-v1',
    path_pattern: '/v1/chat/completions',
    methods: ['POST'],
    price_base_units: 0,
    route_type: 'llm',
    model_name: 'deepseek-chat',
    pricing_type: 'per_token',
    prompt_token_price_base_units: 1000, // 0.10 分 / 1k Tokens
    completion_token_price_base_units: 2000, // 0.20 分 / 1k Tokens
    asset: 'Credits',
    allowed_payment_methods: 'wallet',
    rate_limit_rpm: 3000,
    timeout_ms: 60000,
    status: 'active',
    sla: {
      success_rate_24h: 99.98,
      p95_latency_ms: 380,
      paid_calls_24h: 185200,
      total_calls: 1420000,
      health_status: 'healthy',
    },
    parameters: [
      {
        name: 'model',
        type: 'string',
        required: true,
        description: '模型标识名，固定传入 "deepseek-chat"',
        example: '"deepseek-chat"',
      },
      {
        name: 'messages',
        type: 'array of message objects',
        required: true,
        description: '对话上下文列表，每项包含 role ("system" | "user" | "assistant") 与 content',
        example: '[{"role":"user","content":"你好"}]',
      },
      {
        name: 'stream',
        type: 'boolean',
        required: false,
        description: '是否开启 SSE 流式输出，开启后按实际 Token 毫秒级扣费',
        example: 'true',
      },
      {
        name: 'temperature',
        type: 'number (0.0 - 2.0)',
        required: false,
        description: '采样温度，默认 0.7',
        example: '0.7',
      },
      {
        name: 'max_tokens',
        type: 'integer',
        required: false,
        description: '单次回答最大生成 Token 数量',
        example: '4096',
      },
    ],
    sample_params: {
      model: 'deepseek-chat',
      messages: [
        { role: 'system', content: 'You are a helpful assistant.' },
        { role: 'user', content: '你好，请做个简短的自我介绍。' },
      ],
      stream: true,
    },
    sample_response: {
      id: 'chatcmpl-deepseek-9a8b7c',
      object: 'chat.completion.chunk',
      created: 1729500000,
      model: 'deepseek-chat',
      choices: [
        {
          index: 0,
          delta: { content: '你好！我是 DeepSeek-V3...' },
          finish_reason: null,
        },
      ],
      usage: {
        prompt_tokens: 18,
        completion_tokens: 56,
        total_tokens: 74,
      },
    },
    description_md: `### 功能定义 (Overview)
**DeepSeek-V3 高性能大模型对话接口** 专为大语言模型应用与智能体打造，全面遵循官方 OpenAI 格式标准。无论是直接通过 \`openai\` 官方 SDK 调用，还是通过 LangChain、AutoGPT、LlamaIndex 等框架接入，只需修改 \`base_url\` 与 \`api_key\` 即可实现 0 代码成本平滑迁移。

### 为什么选择 AgentFinOps 大模型流式网关？
- **透明精准的千 Token 计费**：输入仅需 **0.10 分 / 1k Tokens**，输出仅需 **0.20 分 / 1k Tokens**。杜绝任何预付巨额年费或账户隐蔽扣费。
- **毫秒级断流预算保护 (Streaming Budget Guard)**：当客户端断开连接或达到配额阈值，网关立刻停止向上游模型请求并瞬间冻结计费，绝不多扣 1 个 Token。
- **免密快速体验**：在商店首页点击“一键申领测试 Key”，立即可在本地直接运行完整的 Python/Node.js 代码！`,
    created_at: 1729510000,
    updated_at: 1729750000,
  },
  {
    route_id: 'rte_smart_money_01',
    tenant_id: 'ten_quant_alpha',
    provider_name: '智融量化 (AlphaQuant Labs)',
    slug: 'smart-money-tracker-api',
    title: '机构资金 (Smart Money) 资金流向追踪 API',
    summary:
      '实时追踪金融市场机构主力资金净流入、高频资金异动及大额建仓信号，支持按次微计费，无需繁琐线下审批流程或预充值长周期 API Key。',
    category: 'Financial & Market Intel',
    tags: ['Smart Money', '资金流追踪', '主力资金', '量化投研'],
    cluster: 'domestic-v1',
    path_pattern: '/v1/market/smart-money',
    methods: ['GET', 'POST'],
    price_base_units: 1000, // 0.001 Credits
    asset: 'Credits',
    allowed_payment_methods: 'wallet',
    rate_limit_rpm: 1200,
    timeout_ms: 10000,
    status: 'active',
    sla: {
      success_rate_24h: 99.94,
      p95_latency_ms: 142,
      paid_calls_24h: 48920,
      total_calls: 312450,
      health_status: 'healthy',
    },
    parameters: [
      {
        name: 'timeframe',
        type: 'string ("1h" | "4h" | "24h" | "7d")',
        required: true,
        description: '聚合统计时间窗口，默认推荐 "24h"',
        example: '"24h"',
      },
      {
        name: 'min_inflow_million',
        type: 'number',
        required: false,
        description: '过滤机构主力净流入门槛（百万元），默认 10',
        example: '10',
      },
      {
        name: 'limit',
        type: 'integer (1 - 100)',
        required: false,
        description: '返回资金净流入排行数量，默认 20',
        example: '10',
      },
    ],
    sample_params: {
      timeframe: '24h',
      min_inflow_million: 10,
      limit: 10,
    },
    sample_response: {
      timeframe: '24h',
      market: 'cn_equities',
      updated_at: '2026-10-24T12:45:00Z',
      signals: [
        {
          symbol: '600519',
          name: '贵州茅台',
          smart_funds_buying: 38,
          net_inflow_cny: 148520000,
          confidence_score: 0.94,
        },
      ],
    },
    description_md: `### 功能定义 (Overview)
**机构资金 (Smart Money) 资金流向追踪 API** 专为量化投研 AI Agent、金融研报助手与市场资金监控机器人设计。通过实时分析市场高胜率席位与机构资金流向，标记重点主力持仓，提供毫秒级净流入与异动信号。

### 为什么适合 AI Agent 直接调用？
- **免注册零摩擦 (Zero-Onboarding)**：无需在第三方平台填写复杂开户信息或申请长生命周期 API Key。
- **支持微计费协议**：单次调用仅需按次扣费，自动完成握手与本地预算核验。
- **结构化 JSON Schema**：输出经过严格类型规整的 JSON，可直接送入 LangChain、CrewAI、AutoGen 或 MCP 工具上下文。`,
    created_at: 1729500000,
    updated_at: 1729740000,
  },
  {
    route_id: 'rte_deepseek_r1_reasoning_02',
    tenant_id: 'ten_acme_ai',
    provider_name: 'Acme Autonomous AI Corp',
    slug: 'deepseek-r1-verifiable-reasoning-api',
    title: 'DeepSeek-R1 Verifiable Reasoning API',
    summary:
      '高吞吐 DeepSeek-R1 深度推理与代码审计接口，附带硬件远程证明与确定性 Token 计费。支持按次结算（0.005 Credits/req），内置 AgentFinOps 财务防火墙防死循环保护。',
    category: 'AI & LLM',
    tags: ['DeepSeek-R1', 'LLM Reasoning', 'TEE Verified', 'Code Audit'],
    cluster: 'domestic-v1',
    path_pattern: '/v1/chat/completions',
    methods: ['POST'],
    price_base_units: 0,
    route_type: 'llm',
    model_name: 'deepseek-reasoner',
    pricing_type: 'per_token',
    prompt_token_price_base_units: 4000, // 0.40 分 / 1k Tokens
    completion_token_price_base_units: 16000, // 1.60 分 / 1k Tokens
    asset: 'Credits',
    allowed_payment_methods: 'wallet',
    rate_limit_rpm: 600,
    timeout_ms: 60000,
    status: 'active',
    sla: {
      success_rate_24h: 99.89,
      p95_latency_ms: 640,
      paid_calls_24h: 82410,
      total_calls: 615900,
      health_status: 'healthy',
    },
    parameters: [
      {
        name: 'model',
        type: 'string',
        required: true,
        description: '推理模型标识，固定支持 "deepseek-r1" 或 "deepseek-v3"',
        example: '"deepseek-r1"',
      },
      {
        name: 'prompt',
        type: 'string',
        required: true,
        description: '需要执行深度逻辑推理、数学证明或代码逻辑审计的输入指令',
        example: '"Audit this function for parameter replay vulnerabilities"',
      },
      {
        name: 'max_tokens',
        type: 'integer',
        required: false,
        description: '最大输出 Token 数量限制，默认 2048',
        example: '2048',
      },
    ],
    sample_params: {
      model: 'deepseek-r1',
      prompt: 'Analyze token metering idempotency and replay protection invariants.',
      max_tokens: 1024,
    },
    sample_response: {
      id: 'chatcmpl_finops_9f8a7b6c',
      model: 'deepseek-r1',
      tee_attestation: 'sha256:9e8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a',
      reasoning_content: '1. Check idempotency key uniqueness per caller...',
      content: 'FinOps-Token-Metering uses random uuid nonces, enabling robust idempotent tracking...',
    },
    description_md: `### 功能定义 (Overview)
**DeepSeek-R1 Verifiable Reasoning API** 为自主智能体提供按需付费的深度思维链（Chain-of-Thought）推理服务。所有推理节点运行于受信执行环境（TEE），并在响应头与响应体中附带哈希证明，确保模型输出确定性。

### 核心优势
- **按次透明计费**：每次调用固定 \`0.005 Credits\`，告别高昂的月度固定订阅与复杂企业合同。
- **网关级幂等重放**：如遇网络抖动超时，买方 SDK 自动携带相同幂等键免费重放，绝不二次扣费。`,
    created_at: 1729480000,
    updated_at: 1729735000,
  },
  {
    route_id: 'rte_headless_markdown_scraper_03',
    tenant_id: 'ten_web_mesh',
    provider_name: 'HyperCrawl Mesh Network',
    slug: 'stealth-browser-markdown-scraper-api',
    title: 'Stealth Browser Clean Markdown Scraper API',
    summary:
      '专为 LLM 与 RAG 打造的反爬无头浏览器渲染抓取接口，自动过防护盾并将任意动态 SPA 网页转换为高信噪比纯净 Markdown。单次调用 0.002 Credits，支持自动计费。',
    category: 'Web Scraping & Search',
    tags: ['Web Scraping', 'Markdown', 'Headless Browser', 'RAG'],
    cluster: 'domestic-v1',
    path_pattern: '/v1/scrape/markdown',
    methods: ['GET', 'POST'],
    price_base_units: 2000, // 0.002 Credits
    asset: 'Credits',
    allowed_payment_methods: 'wallet',
    rate_limit_rpm: 900,
    timeout_ms: 25000,
    status: 'active',
    sla: {
      success_rate_24h: 99.62,
      p95_latency_ms: 820,
      paid_calls_24h: 64150,
      total_calls: 428000,
      health_status: 'healthy',
    },
    parameters: [
      {
        name: 'url',
        type: 'string (URI)',
        required: true,
        description: '需要抓取并清洗为 Markdown 的目标网页完整 URL',
        example: '"https://aipmedia.cn/docs/"',
      },
      {
        name: 'wait_for_selector',
        type: 'string (CSS Selector)',
        required: false,
        description: '可选：等待特定 DOM 选择器渲染完成后再提取正文',
        example: '"main.article-content"',
      },
      {
        name: 'include_links',
        type: 'boolean',
        required: false,
        description: '是否保留正文内联超链接，默认 true',
        example: 'true',
      },
    ],
    sample_params: {
      url: 'https://aipmedia.cn',
      include_links: true,
    },
    sample_response: {
      url: 'https://aipmedia.cn',
      title: 'AgentFinOps — Financial Firewall & AI Gateway',
      word_count: 642,
      markdown: '# AgentFinOps\n\nThe Enterprise Financial Firewall & Quota Settlement Infrastructure for Autonomous AI Agents...',
    },
    description_md: `### 功能定义 (Overview)
**Stealth Browser Clean Markdown Scraper API** 帮助 Research Agent 与 Coding Agent 在运行时实时读取任意网页，自动剥离广告、导航栏与冗余 DOM 脚本，直接输出送入 LLM Context Window 的纯净 Markdown。

### 平台 SLA 护城河保障
- **不成功不扣费**：仅当上游无头浏览器成功渲染并返回 \`200 OK\` 有效正文时才完成捕获结算；若目标站点返回 \`403/502\`，网关自动释放预留额度。`,
    created_at: 1729460000,
    updated_at: 1729730000,
  },
  {
    route_id: 'rte_sec_edgar_filings_04',
    tenant_id: 'ten_quant_intel',
    provider_name: 'QuantSight Financial Feeds',
    slug: 'us-equities-sec-edgar-realtime-api',
    title: 'US Equities & SEC 10-K/10-Q Instant Intel API',
    summary:
      '美股实时盘口报价与 SEC EDGAR 财报（10-K、10-Q、8-K）结构化关键财务指标提取 API。单次调用 0.01 Credits，支持免预充即时查询。',
    category: 'Financial & Market Intel',
    tags: ['US Stocks', 'SEC Filings', 'Quant', 'NASDAQ'],
    cluster: 'domestic-v1',
    path_pattern: '/v1/market/sec-intel',
    methods: ['GET'],
    price_base_units: 10000, // 0.01 Credits
    asset: 'Credits',
    allowed_payment_methods: 'wallet',
    rate_limit_rpm: 600,
    timeout_ms: 12000,
    status: 'active',
    sla: {
      success_rate_24h: 99.98,
      p95_latency_ms: 95,
      paid_calls_24h: 29400,
      total_calls: 195300,
      health_status: 'healthy',
    },
    parameters: [
      {
        name: 'ticker',
        type: 'string',
        required: true,
        description: '美股股票代码，例如 "NVDA", "AAPL", "MSFT"',
        example: '"NVDA"',
      },
      {
        name: 'filing_type',
        type: 'string ("10-K" | "10-Q" | "8-K" | "QUOTE")',
        required: false,
        description: '查询财报类型或实时报价，默认 "QUOTE"',
        example: '"10-Q"',
      },
    ],
    sample_params: {
      ticker: 'NVDA',
      filing_type: '10-Q',
    },
    sample_response: {
      ticker: 'NVDA',
      company_name: 'NVIDIA Corporation',
      latest_price_usd: 142.5,
      revenue_ttm_usd: 96307000000,
      gross_margin_pct: 75.8,
      latest_filing_date: '2026-08-28',
    },
    description_md: `### 功能定义 (Overview)
**US Equities & SEC 10-K/10-Q Instant Intel API** 将原本每年数万美元的机构级美股行情与 SEC 财报结构化数据拆解为 \`0.01 Credits/次\` 的原子级微支付接口，赋能金融分析 Agent 按需调取权威财报数据。`,
    created_at: 1729440000,
    updated_at: 1729720000,
  },
  {
    route_id: 'rte_financial_audit_05',
    tenant_id: 'ten_aegis_sec',
    provider_name: 'Aegis Enterprise Compliance',
    slug: 'financial-statement-auditor-api',
    title: 'Financial Report & Balance Sheet Auditor API',
    summary:
      '面向企业 AI 智能财务审计，对上市公司资产负债表、利润表和现金流量表进行智能解析与合规审计，精准识别异常记账流水与风险科目。单次 0.0015 Credits。',
    category: 'Enterprise Finance & Compliance',
    tags: ['Finance', 'Auditing', 'Compliance', 'Reporting'],
    cluster: 'domestic-v1',
    path_pattern: '/v1/finance/audit-report',
    methods: ['GET', 'POST'],
    price_base_units: 1500, // 0.0015 Credits
    asset: 'Credits',
    allowed_payment_methods: 'wallet',
    rate_limit_rpm: 1500,
    timeout_ms: 8000,
    status: 'active',
    sla: {
      success_rate_24h: 99.91,
      p95_latency_ms: 168,
      paid_calls_24h: 53120,
      total_calls: 389000,
      health_status: 'healthy',
    },
    parameters: [
      {
        name: 'report_id',
        type: 'string',
        required: true,
        description: '待审计财报标识',
        example: '"rep_2026_q3_001"',
      },
      {
        name: 'company_code',
        type: 'string',
        required: true,
        description: '企业统一社会信用代码或证券代码',
        example: '"600519"',
      },
    ],
    sample_params: {
      report_id: 'rep_2026_q3_001',
      company_code: '600519',
    },
    sample_response: {
      is_compliant: true,
      anomaly_detected: false,
      risk_score: 0.02,
      audited_at: '2026-10-24T12:00:00Z',
    },
    description_md: `### 功能定义 (Overview)
**Financial Report & Balance Sheet Auditor API** 是企业智能财务与合规审计 Agent 的必备前置核验接口。每次仅需花费 \`0.0015 Credits\` 即可对企业科目进行规则扫描与合规校验。`,
    created_at: 1729420000,
    updated_at: 1729710000,
  },
  {
    route_id: 'rte_python_sandbox_exec_06',
    tenant_id: 'ten_cloud_microvm',
    provider_name: 'Firecracker Cloud Compute',
    slug: 'ephemeral-python-code-interpreter-api',
    title: 'Ephemeral Firecracker Python Sandbox API',
    summary:
      '毫秒级冷启动的 Firecracker MicroVM 隔离 Python 代码执行沙箱，预装 Pandas、NumPy、SymPy、Matplotlib，让任何 Agent 安全执行数据分析与数学验证。单次调用 0.003 Credits。',
    category: 'Compute & Developer Tools',
    tags: ['Code Interpreter', 'Firecracker VM', 'Python Sandbox'],
    cluster: 'domestic-v1',
    path_pattern: '/v1/compute/python-exec',
    methods: ['POST'],
    price_base_units: 3000, // 0.003 Credits
    asset: 'Credits',
    allowed_payment_methods: 'wallet',
    rate_limit_rpm: 300,
    timeout_ms: 30000,
    status: 'active',
    sla: {
      success_rate_24h: 99.76,
      p95_latency_ms: 310,
      paid_calls_24h: 19840,
      total_calls: 114200,
      health_status: 'healthy',
    },
    parameters: [
      {
        name: 'code',
        type: 'string',
        required: true,
        description: '要在隔离沙箱内执行的 Python 3.12 代码脚本',
        example: '"import numpy as np; print(np.linalg.eigvals([[2, 1], [1, 2]]).tolist())"',
      },
      {
        name: 'timeout_sec',
        type: 'integer (1 - 30)',
        required: false,
        description: '最大执行超时秒数，默认 10 秒',
        example: '10',
      },
    ],
    sample_params: {
      code: 'import math; print({"sharpe_ratio": round(0.18 / math.sqrt(0.0144), 4)})',
      timeout_sec: 10,
    },
    sample_response: {
      exit_code: 0,
      stdout: "{'sharpe_ratio': 1.5}\n",
      stderr: '',
      execution_time_ms: 48,
    },
    description_md: `### 功能定义 (Overview)
**Ephemeral Firecracker Python Sandbox API** 为不具备本地容器环境的轻量级 Agent 提供硬件虚拟化隔离的代码执行能力，单次请求结束后立即销毁内存镜像，兼顾安全与按需低成本调用。`,
    created_at: 1729400000,
    updated_at: 1729700000,
  },
];
