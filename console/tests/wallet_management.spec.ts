import { describe, it, expect, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useAuthStore } from '../src/stores/auth';
import { useTenantStore } from '../src/stores/tenant';
import { walletApi } from '../src/api/client';

// Mock localStorage for Vitest environment
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value.toString();
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    }
  };
})();

Object.defineProperty(globalThis, 'localStorage', {
  value: localStorageMock,
  writable: true
});

describe('T4.6 Console - Virtual Wallet & Ledger Console Test Suite', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
  });

  describe('1. Tenant Isolation & Wallet Data Loading', () => {
    it('should retrieve the primary wallet for the current tenant', async () => {
      const tenantStore = useTenantStore();
      const tenantId = tenantStore.currentTenantId; // ten_9a8b7c6d5e
      expect(tenantId).toBe('ten_9a8b7c6d5e');

      const wallet = await walletApi.getTenantWallet(tenantId);
      expect(wallet).toBeDefined();
      expect(wallet.tenant_id).toBe('ten_9a8b7c6d5e');
      expect(wallet.wallet_id).toBe('wal_ten_9a8b7c6d5e');
      expect(wallet.asset).toBe('USDC');
      expect(wallet.status).toBe('active');
      expect(wallet.available_cache).toBe(4950000); // 4.95 USDC
      expect(wallet.held_cache).toBe(50000);        // 0.05 USDC
    });

    it('should strictly isolate wallets when switching between tenants', async () => {
      const tenantStore = useTenantStore();

      // Tenant 1: Acme AI
      await tenantStore.switchTenant('ten_9a8b7c6d5e');
      const w1 = await walletApi.getTenantWallet(tenantStore.currentTenantId);
      expect(w1.tenant_id).toBe('ten_9a8b7c6d5e');
      expect(w1.status).toBe('active');
      expect(w1.available_cache).toBeGreaterThan(0);

      // Tenant 2: DevLab
      await tenantStore.switchTenant('ten_3f4e5d6c7b');
      const w2 = await walletApi.getTenantWallet(tenantStore.currentTenantId);
      expect(w2.tenant_id).toBe('ten_3f4e5d6c7b');
      expect(w2.wallet_id).toBe('wal_ten_3f4e5d6c7b');
      expect(w2.status).toBe('frozen_arrears'); // Arrears state
      expect(w2.available_cache).toBe(-250000); // Negative balance
    });
  });

  describe('2. Ledger Entries & Financial Immutability', () => {
    it('should fetch immutable ledger entries for a wallet', async () => {
      const entries = await walletApi.getLedgerEntries('wal_ten_9a8b7c6d5e');
      expect(entries.length).toBeGreaterThan(0);

      const captureEntry = entries.find((e) => e.type === 'capture');
      expect(captureEntry).toBeDefined();
      expect(captureEntry?.amount).toBe(-50000);
      expect(captureEntry?.ref_type).toBe('PaymentIntent');
      expect(captureEntry?.ref_id).toBe('pi_7a8b9c0d1e');

      const topupEntry = entries.find((e) => e.type === 'topup_credit');
      expect(topupEntry).toBeDefined();
      expect(topupEntry?.amount).toBe(5000000);
      expect(topupEntry?.ref_type).toBe('TopUpOrder');
    });

    it('should track reversals and chargebacks leading to arrears', async () => {
      const entries = await walletApi.getLedgerEntries('wal_ten_3f4e5d6c7b');
      const reversal = entries.find((e) => e.type === 'reversal');
      expect(reversal).toBeDefined();
      expect(reversal?.amount).toBe(-250000);
      expect(reversal?.reason).toContain('W6 欠费冻结');
    });
  });

  describe('3. Top-Up Orders & Multi-Channel Records', () => {
    it('should list top-up orders with channel and confirmation details', async () => {
      const orders = await walletApi.getTopUpOrders('ten_9a8b7c6d5e', 'wal_ten_9a8b7c6d5e');
      expect(orders.length).toBeGreaterThanOrEqual(2);

      const onchain = orders.find((o) => o.channel === 'onchain_deposit');
      expect(onchain).toBeDefined();
      expect(onchain?.amount).toBe(10000000); // 10 USDC
      expect(onchain?.status).toBe('confirmed');
      expect(onchain?.external_ref).toMatch(/^0x/);

      const adminGrant = orders.find((o) => o.channel === 'admin_grant');
      expect(adminGrant).toBeDefined();
      expect(adminGrant?.amount).toBe(5000000); // 5 USDC
      expect(adminGrant?.status).toBe('confirmed');
    });
  });

  describe('4. Admin Grant Operations & Mandatory Reason Rule', () => {
    it('should reject admin grant if reason is missing or empty', async () => {
      await expect(
        walletApi.adminGrant('ten_9a8b7c6d5e', {
          amount: 10000000,
          reason: '   '
        })
      ).rejects.toThrow('必须填写发放原因与依据');
    });

    it('should reject admin grant if amount is non-positive', async () => {
      await expect(
        walletApi.adminGrant('ten_9a8b7c6d5e', {
          amount: 0,
          reason: '发放零额度测试'
        })
      ).rejects.toThrow('发放金额必须为正数');
    });

    it('should successfully execute admin grant and update balance and topup list', async () => {
      const initialWallet = await walletApi.getTenantWallet('ten_9a8b7c6d5e');
      const prevBalance = initialWallet.available_cache;

      const updated = await walletApi.adminGrant('ten_9a8b7c6d5e', {
        amount: 2000000, // 2 USDC
        reason: '单元测试额度发放 (T4.6 test grant)',
        idempotency_key: 'idem_test_grant_01'
      });

      expect(updated.available_cache).toBe(prevBalance + 2000000);

      const topups = await walletApi.getTopUpOrders('ten_9a8b7c6d5e', 'wal_ten_9a8b7c6d5e');
      const latest = topups[0];
      expect(latest.amount).toBe(2000000);
      expect(latest.channel).toBe('admin_grant');
      expect(latest.reason).toContain('单元测试额度发放');
    });
  });

  describe('5. Manual Adjustment & Large Amount Approval Control', () => {
    it('should reject manual adjustment if reason is empty', async () => {
      await expect(
        walletApi.adjustBalance('wal_ten_9a8b7c6d5e', {
          amount: 1000000,
          reason: ''
        })
      ).rejects.toThrow('必须填写调账原因与审计说明');
    });

    it('should successfully perform balance adjustment and append adjustment ledger', async () => {
      const initialWallet = await walletApi.getTenantWallet('ten_9a8b7c6d5e');
      const prevBalance = initialWallet.available_cache;

      const updated = await walletApi.adjustBalance('wal_ten_9a8b7c6d5e', {
        amount: -100000, // 扣减 0.1 USDC
        reason: '差错扣减平账 (Manual reconciliation)',
        idempotency_key: 'idem_test_adj_01'
      });

      expect(updated.available_cache).toBe(prevBalance - 100000);

      const ledgers = await walletApi.getLedgerEntries('wal_ten_9a8b7c6d5e');
      const latest = ledgers[0];
      expect(latest.type).toBe('adjustment');
      expect(latest.amount).toBe(-100000);
      expect(latest.reason).toContain('差错扣减平账');
    });
  });

  describe('6. Freeze, Unfreeze & W6 Arrears Circuit Breaker', () => {
    it('should require reason when freezing a wallet', async () => {
      await expect(
        walletApi.freezeWallet('wal_ten_9a8b7c6d5e', {
          reason: '  '
        })
      ).rejects.toThrow('冻结必须填写原因与说明');
    });

    it('should freeze an active wallet and transition status to frozen', async () => {
      const frozen = await walletApi.freezeWallet('wal_ten_9a8b7c6d5e', {
        reason: '租户安全合规例行冻结审查'
      });
      expect(frozen.status).toBe('frozen');

      // Unfreeze to restore
      const restored = await walletApi.unfreezeWallet('wal_ten_9a8b7c6d5e', {
        reason: '审查完毕恢复正常'
      });
      expect(restored.status).toBe('active');
    });

    it('should block unfreezing an arrears account when balance is negative (W6 Rule)', async () => {
      // DevLab wallet has negative balance (-250000 base units)
      await expect(
        walletApi.unfreezeWallet('wal_ten_3f4e5d6c7b', {
          reason: '尝试强行解冻欠费账户'
        })
      ).rejects.toThrow(/欠费状态.*必须先充值平账/);
    });
  });

  describe('7. Double-Entry Reconciliation & Audit Report (T1.7 / T5.7)', () => {
    it('should perform balance reconciliation and check consistency', async () => {
      const report = await walletApi.reconcileWallet('wal_ten_9a8b7c6d5e');
      expect(report).toBeDefined();
      expect(report.wallet_id).toBe('wal_ten_9a8b7c6d5e');
      expect(report.available_cache).toBeDefined();
      expect(report.held_cache).toBeDefined();
      expect(report.total_in_wallet).toBe(report.available_cache + report.held_cache);
      expect(typeof report.is_balanced).toBe('boolean');
    });
  });

  describe('8. RBAC Role-Based Authorization Checks', () => {
    it('should verify Owner and Admin have wallet write permissions', () => {
      const authStore = useAuthStore();
      authStore.setSession('token_owner', {
        member_id: 'usr_owner',
        email: 'owner@test.io',
        name: 'Owner',
        role: 'owner',
        status: 'active',
        tenant_id: 'ten_9a8b7c6d5e'
      });
      expect(authStore.currentRole).toBe('owner');
      expect(authStore.hasRole(['owner', 'admin'])).toBe(true);

      authStore.setSession('token_admin', {
        member_id: 'usr_admin',
        email: 'admin@test.io',
        name: 'Admin',
        role: 'admin',
        status: 'active',
        tenant_id: 'ten_9a8b7c6d5e'
      });
      expect(authStore.currentRole).toBe('admin');
      expect(authStore.hasRole(['owner', 'admin'])).toBe(true);
    });

    it('should verify Operator and Viewer cannot perform write operations', () => {
      const authStore = useAuthStore();

      authStore.setSession('token_op', {
        member_id: 'usr_op',
        email: 'op@test.io',
        name: 'Operator',
        role: 'operator',
        status: 'active',
        tenant_id: 'ten_9a8b7c6d5e'
      });
      expect(authStore.currentRole).toBe('operator');
      expect(authStore.hasRole(['owner', 'admin'])).toBe(false);
      expect(authStore.hasRole(['owner', 'admin', 'operator'])).toBe(true); // Can run reconciliation

      authStore.setSession('token_view', {
        member_id: 'usr_view',
        email: 'viewer@test.io',
        name: 'Viewer',
        role: 'viewer',
        status: 'active',
        tenant_id: 'ten_9a8b7c6d5e'
      });
      expect(authStore.currentRole).toBe('viewer');
      expect(authStore.hasRole(['owner', 'admin'])).toBe(false);
      expect(authStore.hasRole(['owner', 'admin', 'operator'])).toBe(false); // Viewer read-only
    });
  });
});
