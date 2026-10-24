export type CurrencyCode = 'CNY' | 'USD' | 'EUR' | 'USDC';

export interface CurrencyMeta {
  code: CurrencyCode;
  symbol: string;
  label: string;
}

export const SUPPORTED_CURRENCIES: Record<CurrencyCode, CurrencyMeta> = {
  CNY: { code: 'CNY', symbol: '¥', label: 'CNY (¥ 人民币)' },
  USD: { code: 'USD', symbol: '$', label: 'USD ($ 美元)' },
  EUR: { code: 'EUR', symbol: '€', label: 'EUR (€ 欧元)' },
  USDC: { code: 'USDC', symbol: '$', label: 'USDC (稳定币)' }
};

export interface Tenant {
  tenant_id: string;
  name: string;
  currency?: CurrencyCode;
  status: 'active' | 'suspended' | 'terminated';
  created_at?: string | number;
}

export interface Project {
  project_id: string;
  tenant_id: string;
  name: string;
  budget_cap_base_units: number;
  status: 'active' | 'archived';
  created_at?: string | number;
}
