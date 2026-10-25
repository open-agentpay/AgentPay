import type { UpstreamAuthConfig } from './auth_config';

export type RouteStatus = 'active' | 'disabled';
export type PaymentMethod = 'wallet' | 'onchain' | 'both';

export interface RouteParameter {
  name: string;
  type: string;
  required: boolean;
  description: string;
  example?: string;
}

export interface Route {
  route_id: string;
  tenant_id: string;
  project_id: string;
  slug?: string;
  title?: string;
  summary?: string;
  provider_name?: string;
  is_public?: boolean;
  category?: string;
  tags?: string[];
  description_md?: string;
  network?: string;
  network_label?: string;
  path_pattern: string;
  methods?: string[];
  upstream_url: string;
  price_base_units: number;
  asset: string;
  onchain_price_base_units?: number;
  onchain_asset?: string;
  payee_address: string;
  allowed_payment_methods: PaymentMethod;
  rate_limit_rpm: number;
  timeout_ms: number;
  upstream_auth?: UpstreamAuthConfig;
  parameters?: RouteParameter[];
  sample_params?: Record<string, unknown>;
  sample_response?: Record<string, unknown>;
  status: RouteStatus;
  created_at: number;
  updated_at: number;
}

export interface CreateRoutePayload {
  project_id: string;
  slug?: string;
  title?: string;
  summary?: string;
  provider_name?: string;
  is_public?: boolean;
  category?: string;
  tags?: string[];
  description_md?: string;
  network?: string;
  network_label?: string;
  path_pattern: string;
  methods?: string[];
  upstream_url: string;
  price_base_units: number;
  asset?: string;
  onchain_price_base_units?: number;
  onchain_asset?: string;
  payee_address: string;
  allowed_payment_methods: PaymentMethod;
  rate_limit_rpm?: number;
  timeout_ms?: number;
  upstream_auth?: UpstreamAuthConfig;
  parameters?: RouteParameter[];
  sample_params?: Record<string, unknown>;
  sample_response?: Record<string, unknown>;
  status?: RouteStatus;
}

export interface UpdateRoutePayload {
  project_id?: string;
  slug?: string;
  title?: string;
  summary?: string;
  provider_name?: string;
  is_public?: boolean;
  category?: string;
  tags?: string[];
  description_md?: string;
  network?: string;
  network_label?: string;
  path_pattern?: string;
  methods?: string[];
  upstream_url?: string;
  price_base_units?: number;
  asset?: string;
  onchain_price_base_units?: number;
  onchain_asset?: string;
  payee_address?: string;
  allowed_payment_methods?: PaymentMethod;
  rate_limit_rpm?: number;
  timeout_ms?: number;
  upstream_auth?: UpstreamAuthConfig;
  parameters?: RouteParameter[];
  sample_params?: Record<string, unknown>;
  sample_response?: Record<string, unknown>;
  status?: RouteStatus;
}
