export interface RouteTelemetrySummary {
  total_calls: number;
  success_rate: number;
  avg_duration_ms: number;
  p95_duration_ms: number;
  total_tokens: number;
  prompt_tokens: number;
  completion_tokens: number;
  failed_calls: number;
}

export interface RouteTelemetryItem {
  route_id: string;
  model_name: string;
  title: string;
  route_type: string;
  calls: number;
  success_rate: number;
  avg_duration_ms: number;
  p95_duration_ms: number;
  total_tokens: number;
  prompt_tokens: number;
  completion_tokens: number;
  cost_amount: number;
  status: string;
}

export interface RouteTelemetryOverviewResponse {
  tenant_id: string;
  time_range: string;
  project_id?: string;
  summary: RouteTelemetrySummary;
  routes: RouteTelemetryItem[];
  cached_at: number;
}
