export interface AuditEvent {
  audit_id: string;
  tenant_id: string;
  operator_id: string;
  action: string;
  target_type: string;
  target_id: string;
  before_state?: Record<string, any> | null;
  after_state?: Record<string, any> | null;
  client_ip: string;
  created_at: string | number;
}

export interface AuditEventFilter {
  target_type?: string;
  target_id?: string;
  limit?: number;
}
