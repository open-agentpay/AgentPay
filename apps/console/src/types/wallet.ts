export type WalletStatus = 'active' | 'frozen' | 'frozen_arrears' | 'closed';

export type HoldStatus = 'held' | 'captured' | 'released' | 'expired';

export type LedgerEntryType =
  | 'topup_credit'
  | 'capture'
  | 'hold_release'
  | 'adjustment'
  | 'reversal';

export type TopUpChannel = 'admin_grant' | 'onchain_deposit' | 'fiat';

export type TopUpStatus =
  | 'created'
  | 'pending'
  | 'confirmed'
  | 'failed'
  | 'expired'
  | 'reversed';

export interface Wallet {
  wallet_id: string;
  tenant_id: string;
  asset: string;
  status: WalletStatus;
  version: number;
  available_cache: number;
  held_cache: number;
  created_at: string;
  updated_at: string;
}

export interface LedgerEntry {
  entry_id: string;
  wallet_id: string;
  tenant_id: string;
  type: LedgerEntryType;
  amount: number;
  balance_after?: number;
  idempotency_key: string;
  ref_type: string;
  ref_id: string;
  operator_id?: string;
  reason?: string;
  created_at: string;
}

export interface TopUpOrder {
  topup_id: string;
  wallet_id: string;
  tenant_id: string;
  channel: TopUpChannel;
  amount: number;
  asset: string;
  status: TopUpStatus;
  external_ref?: string;
  operator_id?: string;
  reason?: string;
  created_at: string;
  confirmed_at?: string;
}

export interface ReconciliationReport {
  wallet_id: string;
  available_cache: number;
  held_cache: number;
  total_in_wallet: number;
  sum_ledger_entries: number;
  sum_active_holds: number;
  is_balanced: boolean;
  discrepancy: number;
}

export interface AdminGrantPayload {
  amount: number;
  reason: string;
  idempotency_key?: string;
}

export interface AdjustWalletPayload {
  amount: number;
  reason: string;
  idempotency_key?: string;
  approved_by?: string;
}

export interface FreezeWalletPayload {
  reason: string;
}
