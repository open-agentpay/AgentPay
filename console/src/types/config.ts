export interface ValidationError {
  target_type: 'Route' | 'Policy' | 'Global';
  target_id: string;
  field: string;
  code: string;
  message: string;
}

export interface ValidationSummary {
  total_routes: number;
  total_policies: number;
  error_count: number;
  warning_count: number;
}

export interface ValidationResult {
  valid: boolean;
  errors: ValidationError[];
  summary: ValidationSummary;
  validated_at: number;
}

export interface ConfigDraftSummary {
  tenant_id: string;
  pending_changes: number;
  routes_count: number;
  policies_count: number;
  last_validated_at?: number;
  is_validated: boolean;
  last_result?: ValidationResult;
}

export interface RouteDiffItem {
  route_id: string;
  action: 'added' | 'modified' | 'deleted';
  path_pattern: string;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
}

export interface PolicyDiffItem {
  policy_id: string;
  action: 'added' | 'modified' | 'deleted';
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
}

export interface ConfigDraftDiff {
  tenant_id: string;
  active_version_id?: string;
  has_changes: boolean;
  route_diffs: RouteDiffItem[];
  policy_diffs: PolicyDiffItem[];
}

export interface ConfigVersion {
  version_id: string;
  tenant_id: string;
  version_number: number;
  published_by: string;
  publisher_email?: string;
  change_summary: string;
  snapshot_hash: string;
  routes_count: number;
  policies_count: number;
  is_active: boolean;
  created_at: string | number;
  snapshot_json?: Record<string, unknown>;
}

export interface PublishConfigPayload {
  change_summary?: string;
}

export interface RollbackConfigPayload {
  target_version_id: string;
  reason?: string;
}
