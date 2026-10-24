import type { UpstreamAuthConfig } from './auth_config';

export type RouteStatus = 'active' | 'disabled';
export type PaymentMethod = 'wallet' | 'onchain' | 'both';

export interface Route {
  route_id: string;
  tenant_id: string;
  project_id: string;
  slug?: string;
  title?: string;
  is_public?: boolean;
  category?: string;
  tags?: string[];
  description_md?: string;
  network?: string;
  path_pattern: string;
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
  status: RouteStatus;
  created_at: number;
  updated_at: number;
}

export interface CreateRoutePayload {
  project_id: string;
  slug?: string;
  title?: string;
  is_public?: boolean;
  category?: string;
  tags?: string[];
  description_md?: string;
  network?: string;
  path_pattern: string;
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
  status?: RouteStatus;
}

export interface UpdateRoutePayload {
  project_id?: string;
  slug?: string;
  title?: string;
  is_public?: boolean;
  category?: string;
  tags?: string[];
  description_md?: string;
  network?: string;
  path_pattern?: string;
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
  status?: RouteStatus;
}
