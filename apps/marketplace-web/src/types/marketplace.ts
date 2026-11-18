export type HealthStatus = 'healthy' | 'degraded' | 'maintenance';

export interface MarketplaceSLAMetrics {
  success_rate_24h: number;
  p95_latency_ms: number;
  paid_calls_24h: number;
  total_calls: number;
  health_status: HealthStatus;
}

export interface MarketplaceParameter {
  name: string;
  type: string;
  required: boolean;
  description: string;
  example?: string;
}

export interface MarketplaceRouteItem {
  route_id: string;
  tenant_id: string;
  provider_name: string;
  slug: string;
  title: string;
  summary?: string;
  category: string;
  tags: string[];
  description_md: string;
  cluster?: string;
  network?: string;
  network_label?: string;
  path_pattern: string;
  methods: string[];
  price_base_units: number;
  asset: string;
  onchain_price_base_units?: number;
  onchain_asset?: string;
  payee_address?: string;
  allowed_payment_methods?: 'wallet' | 'onchain' | 'both' | string;
  rate_limit_rpm: number;
  timeout_ms: number;
  status: 'active' | 'disabled';
  sla: MarketplaceSLAMetrics;
  parameters?: MarketplaceParameter[];
  sample_params?: Record<string, unknown>;
  sample_response?: Record<string, unknown>;
  route_type?: 'llm' | 'standard' | string;
  model_name?: string;
  pricing_type?: 'per_token' | 'per_call' | string;
  prompt_token_price_base_units?: number;
  completion_token_price_base_units?: number;
  created_at: number;
  updated_at: number;
}

export interface MarketplaceFilter {
  q?: string;
  category?: string;
  network?: string;
  tag?: string;
}
