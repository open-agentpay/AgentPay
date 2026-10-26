export type AuthFieldKind = 'text' | 'password' | 'select' | 'number' | 'switch' | 'textarea';

export interface AuthFieldSchema {
  key: string;
  label: string;
  kind: AuthFieldKind;
  options?: string[];
  secret: boolean;
  required: boolean;
  default_value?: unknown;
  description?: string;
}

export interface AuthTypeSchema {
  type: string;
  label: string;
  description: string;
  fields: AuthFieldSchema[];
}

export interface UpstreamAuthConfig {
  type: string;
  params?: Record<string, unknown>;
  secrets?: Record<string, string>;
  has_secret?: boolean;
  key_version?: number;
  updated_at?: number;
}

export interface TestUpstreamPayload {
  upstream_url: string;
  method?: string;
  upstream_auth?: UpstreamAuthConfig;
}

export interface TestUpstreamResult {
  success: boolean;
  status_code: number;
  status_text: string;
  latency_ms: number;
  sent_headers: Record<string, string>;
  error?: string;
}
