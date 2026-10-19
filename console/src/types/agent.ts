export type AgentStatus = 'active' | 'suspended' | 'revoked';

export type KYAAutonomyLevel = 'L1_assisted' | 'L2_semi_autonomous' | 'L3_fully_autonomous';

export type KYARiskLevel = 'low' | 'medium' | 'high' | 'critical';

export type KYAVerificationStatus = 'verified' | 'pending_review' | 'unverified';

export interface KYAProfile {
  model_provider: string; // OpenAI, Anthropic, DeepSeek, Local / Self-hosted
  model_version: string;  // deepseek-v3, gpt-4o, claude-3-5-sonnet
  framework: string;      // LangChain, CrewAI, AutoGen, Custom
  hosting_env: string;    // Cloud Worker, TEE Enclave, Local Machine
  owner_name: string;     // 技术责任人
  owner_email: string;    // 责任人邮箱
  department: string;     // 业务归属部门
  authorized_by: string;  // 业务授权人
  autonomy_level: KYAAutonomyLevel;
  risk_level: KYARiskLevel;
  status: KYAVerificationStatus;
  max_single_spend?: number; // 最大单笔限额 base units
  allowed_tools?: string[];  // 授权调用的工具/能力列表
  attestation_hash?: string; // KYA 证明或哈希指纹
  verified_at?: string;
  notes?: string;
}

export interface AgentPolicyConfig {
  budget_cap_base_units?: number;
  rate_limit_rpm?: number;
  rate_limit_qps?: number;
  route_allowlist?: string[];
  kya?: KYAProfile;
}

export interface Agent {
  agent_id: string;
  tenant_id: string;
  project_id: string;
  name: string;
  status: AgentStatus;
  policy_config?: AgentPolicyConfig;
  kya_profile?: KYAProfile;
  created_at: number | string;
  updated_at: number | string;
  // Metrics & Stats
  requests_count?: number;
  total_spend_base_units?: number;
  reject_rate?: string | number;
  last_active_at?: string;
}

export interface AgentAPIKey {
  key_id: string;
  tenant_id: string;
  project_id: string;
  agent_id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  status: 'active' | 'revoked' | 'expired';
  created_at: number | string;
  expires_at?: number | string;
  last_used_at?: number | string;
}

export type TimelineEventType =
  | 'agent.request'
  | 'agent.policy_decision'
  | 'agent.spend'
  | 'agent.control'
  | 'agent.anomaly'
  | 'agent.export';

export interface AgentTimelineEvent {
  event_id: string;
  event_type: TimelineEventType;
  occurred_at: string;
  actor: 'agent' | 'admin' | 'system';
  summary: string;
  // Observed (客观事实，网关可观测防伪造)
  observed: {
    tenant_id: string;
    project_id: string;
    agent_id: string;
    request_id: string;
    route_id?: string;
    config_version?: string;
    status_code?: number;
    latency_ms?: number;
    cost_base_units?: number;
    policy_decision?: 'allow' | 'deny';
    reject_code?: string;
    payment_intent_id?: string;
    settlement_id?: string;
    hold_id?: string;
    request_hash?: string;
    operator_id?: string;
    control_reason?: string;
  };
  // Declared (Agent 客户端自报声明，仅用于关联分析，不可作为授权或计费依据)
  declared?: {
    session_id?: string;
    task_id?: string;
    tool_name?: string;
    parent_request_id?: string;
    user_prompt_summary?: string;
  };
}

export interface AgentTimelineQuery {
  session_id?: string;
  task_id?: string;
  event_type?: string;
  limit?: number;
}

export interface CreateAgentPayload {
  project_id: string;
  name: string;
  policy_config?: AgentPolicyConfig;
  kya_profile?: KYAProfile;
}

export interface UpdateAgentPayload {
  name?: string;
  policy_config?: AgentPolicyConfig;
  kya_profile?: KYAProfile;
}

export interface KillAgentPayload {
  reason: string;
  revoke?: boolean;
}

export interface CreateAgentKeyPayload {
  project_id: string;
  name: string;
  scopes?: string[];
  expires_at?: number;
}
