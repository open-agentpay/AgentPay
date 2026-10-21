/**
 * Payment, Settlement and Fulfillment Correlation Types
 * Conforms to Specification v2.1 (specs/payment-state-machine-v2.md)
 */

export type PaymentMethod = 'wallet' | 'onchain';

export type IntentStatus =
  | 'created'
  | 'challenged'
  | 'authorized'
  | 'rejected'
  | 'expired'
  | 'completed'
  | 'failed'
  | 'needs_review';

export type SettlementStatus =
  | 'not_started'
  | 'held'
  | 'submitted'
  | 'pending'
  | 'confirmed'
  | 'released'
  | 'failed'
  | 'unknown';

export type FulfillmentStatus =
  | 'not_started'
  | 'in_progress'
  | 'succeeded'
  | 'failed'
  | 'unknown';

export type DisplayStatus =
  | 'WALLET_HOLD_PROCESSING'
  | 'SETTLEMENT_IN_PROGRESS'
  | 'FULFILLMENT_IN_PROGRESS'
  | 'COMPLETED'
  | 'SETTLED_FULFILLMENT_FAILED_REFUND_REQUIRED'
  | 'SETTLED_FULFILLMENT_UNKNOWN_NEEDS_REVIEW'
  | 'RELEASED_UNBILLED'
  | 'PAYMENT_FAILED'
  | 'RECOVERY_REQUIRED'
  | 'REJECTED'
  | 'EXPIRED';

export interface PaymentIntent {
  intent_id: string;
  tenant_id: string;
  project_id: string;
  agent_id?: string;
  route_id: string;
  idempotency_key?: string;
  request_id: string;
  payment_method: PaymentMethod;
  amount_base_units: number;
  asset: string;
  status: IntentStatus;
  config_version_id?: string;
  created_at: string;
  updated_at: string;
}

export interface SettlementRecord {
  settlement_id: string;
  intent_id: string;
  payment_method: PaymentMethod;
  network_id?: string;
  asset_address: string;
  payer_address: string;
  payee_address: string;
  amount_base_units: number;
  nonce?: string;
  tx_hash?: string;
  block_number?: number;
  status: SettlementStatus;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface FulfillmentRecord {
  fulfillment_id: string;
  intent_id: string;
  upstream_url: string;
  upstream_status: number;
  latency_ms: number;
  response_body_hash?: string;
  status: FulfillmentStatus;
  created_at: string;
  updated_at: string;
}

export interface WalletHoldAssociation {
  hold_id: string;
  wallet_id: string;
  amount: number;
  status: 'held' | 'captured' | 'released' | 'expired';
  expires_at: string;
}

export interface LedgerAssociation {
  entry_id: string;
  wallet_id: string;
  type: 'capture' | 'hold_release' | string;
  amount: number;
  created_at: string;
}

export interface PaymentAuditRecord {
  intent_id: string;
  tenant_id: string;
  project_id: string;
  agent_id?: string;
  route_id: string;
  request_id: string;
  idempotency_key?: string;
  payment_method: PaymentMethod;
  amount_base_units: number;
  asset: string;

  // Composite Status & Notes
  display_status: DisplayStatus;
  display_note: string;
  is_recovery_required: boolean;

  // Three Orthogonal Subsystems
  intent_status: IntentStatus;
  settlement_status: SettlementStatus;
  fulfillment_status: FulfillmentStatus;

  // Linked Entities
  intent?: PaymentIntent;
  settlement?: SettlementRecord;
  fulfillment?: FulfillmentRecord;
  hold?: WalletHoldAssociation;
  ledger?: LedgerAssociation;

  created_at: string;
  updated_at: string;
}

export interface PaymentAuditQuery {
  project_id?: string;
  agent_id?: string;
  route_id?: string;
  payment_method?: string;
  display_status?: string;
  intent_status?: string;
  start_time?: string;
  end_time?: string;
  limit?: number;
  offset?: number;
}

export interface PaymentListResponse {
  payments: PaymentAuditRecord[];
  total: number;
  limit: number;
  offset: number;
}
