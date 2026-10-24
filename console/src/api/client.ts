import axios from 'axios';
import {
  MOCK_USERS,
  MOCK_TENANTS,
  MOCK_PROJECTS,
  MOCK_WALLETS,
  MOCK_LEDGER_ENTRIES,
  MOCK_TOPUP_ORDERS,
  MOCK_AGENTS,
  MOCK_AGENT_KEYS,
  MOCK_AGENT_TIMELINE,
  MOCK_PAYMENTS
} from './mockData';
import type {
  PaymentAuditRecord,
  PaymentAuditQuery,
  PaymentListResponse
} from '@/types/payment';
import type {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
  MeResponse,
  UserRole,
  UserProfile
} from '@/types/auth';
import type {
  Wallet,
  LedgerEntry,
  TopUpOrder,
  ReconciliationReport,
  AdminGrantPayload,
  AdjustWalletPayload,
  FreezeWalletPayload
} from '@/types/wallet';
import type {
  Agent,
  AgentStatus,
  AgentAPIKey,
  AgentTimelineEvent,
  AgentTimelineQuery,
  CreateAgentPayload,
  UpdateAgentPayload,
  KillAgentPayload,
  CreateAgentKeyPayload
} from '@/types/agent';

const apiClient = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
});

// Request interceptor to attach JWT/Session token
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('agentpay_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  const currentTenantId = localStorage.getItem('agentpay_tenant_id');
  if (currentTenantId) {
    config.headers['X-Tenant-Id'] = currentTenantId;
  }
  const currentProjectId = localStorage.getItem('agentpay_project_id');
  if (currentProjectId) {
    config.headers['X-Project-Id'] = currentProjectId;
  }
  return config;
});

// Response interceptor
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    // Only redirect to login if 401 is NOT from auth endpoints (e.g. invalid credentials)
    const url = error.config?.url || '';
    const isAuthEndpoint = url.includes('/auth/login') || url.includes('/auth/register');

    if (error.response && error.response.status === 401 && !isAuthEndpoint) {
      localStorage.removeItem('agentpay_token');
      localStorage.removeItem('agentpay_user');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export const authApi = {
  async register(payload: RegisterRequest): Promise<RegisterResponse> {
    try {
      const response = await apiClient.post<RegisterResponse>('/auth/register', payload);
      return response.data;
    } catch (err: unknown) {
      // If server returned an API error response, throw that error with its message
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '注册失败');
      }
      throw err;
    }
  },

  async login(payload: LoginRequest): Promise<LoginResponse> {
    // If it's a quick role-switch demo without real password attempt
    if (!payload.password && payload.role) {
      const roleKey = payload.role as UserRole;
      const matchedUser = MOCK_USERS[roleKey];
      return {
        token: `mock_jwt_token_${matchedUser.role}_${Date.now()}`,
        user: matchedUser,
        tenant: MOCK_TENANTS[0]
      };
    }

    try {
      const response = await apiClient.post<LoginResponse>('/auth/login', {
        email: payload.email,
        password: payload.password
      });
      return response.data;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '登录失败，请检查账号密码');
      }
      // If server is unreachable, allow mock fallback if email matches mock user
      if (axios.isAxiosError(err) && !err.response) {
        const roleKey = (payload.role || 'admin') as UserRole;
        const matchedUser = MOCK_USERS[roleKey] || {
          member_id: 'usr_custom_mock',
          email: payload.email,
          name: payload.email.split('@')[0],
          role: roleKey,
          status: 'active',
          tenant_id: 'ten_9a8b7c6d5e'
        };
        return {
          token: `mock_jwt_token_${matchedUser.role}_${Date.now()}`,
          user: matchedUser,
          tenant: MOCK_TENANTS[0]
        };
      }
      throw err;
    }
  },

  async getCurrentUser(): Promise<MeResponse | null> {
    try {
      const response = await apiClient.get<MeResponse>('/auth/me');
      return response.data;
    } catch {
      const userStr = localStorage.getItem('agentpay_user');
      if (!userStr) return null;
      const user = JSON.parse(userStr);
      return {
        user,
        tenant: MOCK_TENANTS[0],
        permissions: ['tenant:read', 'project:read', 'route:read', 'wallet:read']
      };
    }
  },

  async changePassword(oldPassword: string, newPassword: string): Promise<void> {
    try {
      await apiClient.post('/auth/password', {
        old_password: oldPassword,
        new_password: newPassword
      });
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '密码修改失败');
      }
      throw err;
    }
  }
};

export interface AddMemberPayload {
  email: string;
  role: UserRole;
  password?: string;
}

export interface UpdateMemberPayload {
  role?: UserRole;
  status?: 'active' | 'disabled';
}

export const memberApi = {
  async getMembers(tenantId: string): Promise<UserProfile[]> {
    try {
      const response = await apiClient.get<{ members: UserProfile[] }>(`/tenants/${tenantId}/members`);
      return response.data.members;
    } catch {
      // Mock fallback
      return Object.values(MOCK_USERS);
    }
  },

  async addMember(tenantId: string, payload: AddMemberPayload): Promise<UserProfile> {
    try {
      const response = await apiClient.post<{ member: UserProfile }>(`/tenants/${tenantId}/members`, payload);
      return response.data.member;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '添加成员失败');
      }
      throw err;
    }
  },

  async updateMember(tenantId: string, memberId: string, payload: UpdateMemberPayload): Promise<UserProfile> {
    try {
      const response = await apiClient.patch<{ member: UserProfile }>(`/tenants/${tenantId}/members/${memberId}`, payload);
      return response.data.member;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '更新成员失败');
      }
      throw err;
    }
  },

  async removeMember(tenantId: string, memberId: string): Promise<void> {
    try {
      await apiClient.delete(`/tenants/${tenantId}/members/${memberId}`);
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '移除成员失败');
      }
      throw err;
    }
  }
};

export const tenantApi = {
  async getTenants(): Promise<import('@/types/tenant').Tenant[]> {
    try {
      const response = await apiClient.get<{ tenants: import('@/types/tenant').Tenant[] }>('/tenants');
      return response.data.tenants || [];
    } catch {
      return MOCK_TENANTS;
    }
  },

  async updateTenantCurrency(
    tenantId: string,
    currency: import('@/types/tenant').CurrencyCode
  ): Promise<import('@/types/tenant').Tenant> {
    try {
      const response = await apiClient.patch<{ tenant: import('@/types/tenant').Tenant }>(
        `/tenants/${tenantId}/currency`,
        { currency }
      );
      return response.data.tenant;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '更新租户本位币失败');
      }
      const mock = MOCK_TENANTS.find((t) => t.tenant_id === tenantId);
      if (mock) {
        mock.currency = currency;
        return { ...mock };
      }
      throw err;
    }
  },

  async getProjects(tenantId?: string): Promise<import('@/types/tenant').Project[]> {
    if (!tenantId) return [];
    try {
      const response = await apiClient.get<{ projects: import('@/types/tenant').Project[] }>(`/tenants/${tenantId}/projects`);
      return response.data.projects || [];
    } catch {
      return MOCK_PROJECTS.filter((p) => p.tenant_id === tenantId);
    }
  }
};

export interface CreateProjectPayload {
  name: string;
  budget_cap_base_units?: number;
}

export interface UpdateProjectPayload {
  name?: string;
  budget_cap_base_units?: number;
  status?: 'active' | 'archived';
}

export const projectApi = {
  async getProjects(tenantId: string): Promise<import('@/types/tenant').Project[]> {
    return tenantApi.getProjects(tenantId);
  },

  async createProject(tenantId: string, payload: CreateProjectPayload): Promise<import('@/types/tenant').Project> {
    try {
      const response = await apiClient.post<{ project: import('@/types/tenant').Project }>(`/tenants/${tenantId}/projects`, payload);
      return response.data.project;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '创建项目失败');
      }
      throw err;
    }
  },

  async updateProject(tenantId: string, projectId: string, payload: UpdateProjectPayload): Promise<import('@/types/tenant').Project> {
    try {
      const response = await apiClient.patch<{ project: import('@/types/tenant').Project }>(`/tenants/${tenantId}/projects/${projectId}`, payload);
      return response.data.project;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '更新项目失败');
      }
      throw err;
    }
  },

  async deleteProject(tenantId: string, projectId: string): Promise<void> {
    try {
      await apiClient.delete(`/tenants/${tenantId}/projects/${projectId}`);
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '删除项目失败');
      }
      throw err;
    }
  }
};

export const routeApi = {
  async getRoutes(tenantId: string, projectId?: string): Promise<import('@/types/route').Route[]> {
    try {
      const url = projectId ? `/tenants/${tenantId}/routes?project_id=${projectId}` : `/tenants/${tenantId}/routes`;
      const response = await apiClient.get<{ routes: import('@/types/route').Route[] }>(url);
      return response.data.routes || [];
    } catch {
      return [];
    }
  },

  async createRoute(tenantId: string, payload: import('@/types/route').CreateRoutePayload): Promise<import('@/types/route').Route> {
    try {
      const response = await apiClient.post<{ route: import('@/types/route').Route }>(`/tenants/${tenantId}/routes`, payload);
      return response.data.route;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '创建路由失败');
      }
      throw err;
    }
  },

  async updateRoute(tenantId: string, routeId: string, payload: import('@/types/route').UpdateRoutePayload): Promise<import('@/types/route').Route> {
    try {
      const response = await apiClient.patch<{ route: import('@/types/route').Route }>(`/tenants/${tenantId}/routes/${routeId}`, payload);
      return response.data.route;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '更新路由失败');
      }
      throw err;
    }
  },

  async deleteRoute(tenantId: string, routeId: string): Promise<void> {
    try {
      await apiClient.delete(`/tenants/${tenantId}/routes/${routeId}`);
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '删除路由失败');
      }
      throw err;
    }
  },

  async getAuthTypes(): Promise<import('@/types/auth_config').AuthTypeSchema[]> {
    try {
      const response = await apiClient.get<{ types: import('@/types/auth_config').AuthTypeSchema[] }>('/auth-types');
      return response.data.types;
    } catch {
      return [
        { type: 'none', label: '无鉴权 (None)', description: '网关透传请求，不注入凭证', fields: [] },
        {
          type: 'api_key',
          label: 'API Key 鉴权',
          description: '通过 Header/Query/Cookie 注入 API Key',
          fields: [
            { key: 'in', label: '传递位置', kind: 'select', options: ['header', 'query', 'cookie'], secret: false, required: true },
            { key: 'name', label: '参数名', kind: 'text', secret: false, required: true },
            { key: 'prefix', label: '值前缀', kind: 'text', secret: false, required: false },
            { key: 'value', label: '密钥值', kind: 'password', secret: true, required: true }
          ]
        },
        {
          type: 'bearer',
          label: 'Bearer Token',
          description: '注入 Authorization: Bearer <token>',
          fields: [
            { key: 'token', label: 'Token 凭证', kind: 'password', secret: true, required: true }
          ]
        },
        {
          type: 'basic',
          label: 'HTTP Basic 认证',
          description: '注入 Base64 用户名与密码',
          fields: [
            { key: 'username', label: '用户名', kind: 'text', secret: true, required: true },
            { key: 'password', label: '密码', kind: 'password', secret: true, required: true }
          ]
        }
      ];
    }
  },

  async testUpstream(tenantId: string, payload: import('@/types/auth_config').TestUpstreamPayload): Promise<import('@/types/auth_config').TestUpstreamResult> {
    try {
      const response = await apiClient.post<{ result: import('@/types/auth_config').TestUpstreamResult }>(`/tenants/${tenantId}/routes/test-upstream`, payload);
      return response.data.result;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '上游测试请求失败');
      }
      throw err;
    }
  }
};

export const configApi = {
  async validateDraft(tenantId: string): Promise<import('@/types/config').ValidationResult> {
    try {
      const response = await apiClient.post<{ result: import('@/types/config').ValidationResult }>(`/tenants/${tenantId}/config/validate`);
      return response.data.result;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '配置校验失败');
      }
      throw err;
    }
  },

  async getDraftSummary(tenantId: string): Promise<import('@/types/config').ConfigDraftSummary> {
    try {
      const response = await apiClient.get<{ draft: import('@/types/config').ConfigDraftSummary }>(`/tenants/${tenantId}/config/draft`);
      return response.data.draft;
    } catch {
      return {
        tenant_id: tenantId,
        pending_changes: 0,
        routes_count: 0,
        policies_count: 0,
        is_validated: false
      };
    }
  },

  async getDraftDiff(tenantId: string): Promise<import('@/types/config').ConfigDraftDiff> {
    try {
      const response = await apiClient.get<{ diff: import('@/types/config').ConfigDraftDiff }>(`/tenants/${tenantId}/config/draft/diff`);
      return response.data.diff;
    } catch {
      return {
        tenant_id: tenantId,
        has_changes: false,
        route_diffs: [],
        policy_diffs: []
      };
    }
  },

  async publishConfig(tenantId: string, payload: import('@/types/config').PublishConfigPayload = {}): Promise<import('@/types/config').ConfigVersion> {
    try {
      const response = await apiClient.post<{ version: import('@/types/config').ConfigVersion }>(`/tenants/${tenantId}/config/publish`, payload);
      return response.data.version;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '发布配置版本失败');
      }
      throw err;
    }
  },

  async getVersions(tenantId: string): Promise<import('@/types/config').ConfigVersion[]> {
    try {
      const response = await apiClient.get<{ versions: import('@/types/config').ConfigVersion[] }>(`/tenants/${tenantId}/config/versions`);
      return response.data.versions;
    } catch {
      return [];
    }
  },

  async getVersion(tenantId: string, versionId: string): Promise<import('@/types/config').ConfigVersion> {
    try {
      const response = await apiClient.get<{ version: import('@/types/config').ConfigVersion }>(`/tenants/${tenantId}/config/versions/${versionId}`);
      return response.data.version;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '获取配置版本失败');
      }
      throw err;
    }
  },

  async rollbackConfig(tenantId: string, payload: import('@/types/config').RollbackConfigPayload): Promise<import('@/types/config').ConfigVersion> {
    try {
      const response = await apiClient.post<{ version: import('@/types/config').ConfigVersion }>(`/tenants/${tenantId}/config/rollback`, payload);
      return response.data.version;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '版本回滚失败');
      }
      throw err;
    }
  },

  async getVersionDiff(tenantId: string, baseVersionId?: string, targetVersionId?: string): Promise<import('@/types/config').ConfigDraftDiff> {
    try {
      const params: Record<string, string> = {};
      if (baseVersionId) params.base = baseVersionId;
      if (targetVersionId) params.target = targetVersionId;
      const response = await apiClient.get<{ diff: import('@/types/config').ConfigDraftDiff }>(`/tenants/${tenantId}/config/versions/diff`, { params });
      return response.data.diff;
    } catch {
      return {
        tenant_id: tenantId,
        has_changes: false,
        route_diffs: [],
        policy_diffs: []
      };
    }
  }
};
 
export const walletApi = {
  async getTenantWallet(tenantId: string): Promise<Wallet> {
    try {
      const response = await apiClient.get<{ wallet: Wallet }>(`/tenants/${tenantId}/wallets`);
      return response.data.wallet;
    } catch {
      if (MOCK_WALLETS[tenantId]) {
        return { ...MOCK_WALLETS[tenantId] };
      }
      return {
        wallet_id: `wal_${tenantId}`,
        tenant_id: tenantId,
        asset: 'USDC',
        status: 'active',
        version: 1,
        available_cache: 0,
        held_cache: 0,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString()
      };
    }
  },

  async getWalletById(walletId: string): Promise<Wallet> {
    try {
      const response = await apiClient.get<{ wallet: Wallet }>(`/wallets/${walletId}`);
      return response.data.wallet;
    } catch (err: unknown) {
      const found = Object.values(MOCK_WALLETS).find((w) => w.wallet_id === walletId);
      if (found) return { ...found };
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '获取钱包失败');
      }
      throw err;
    }
  },

  async getLedgerEntries(walletId: string): Promise<LedgerEntry[]> {
    try {
      const response = await apiClient.get<{ count: number; items: LedgerEntry[] }>(`/wallets/${walletId}/ledger`);
      return response.data.items || [];
    } catch {
      return MOCK_LEDGER_ENTRIES[walletId] ? [...MOCK_LEDGER_ENTRIES[walletId]] : [];
    }
  },

  async getTopUpOrders(tenantId: string, walletId?: string): Promise<TopUpOrder[]> {
    const targetWalletId = walletId || `wal_${tenantId}`;
    try {
      const response = await apiClient.get<{ count: number; items: TopUpOrder[] }>(`/wallets/${targetWalletId}/topups`);
      return response.data.items || [];
    } catch {
      return MOCK_TOPUP_ORDERS[targetWalletId] ? [...MOCK_TOPUP_ORDERS[targetWalletId]] : [];
    }
  },

  async adminGrant(tenantId: string, payload: AdminGrantPayload): Promise<Wallet> {
    if (!payload.reason || !payload.reason.trim()) {
      throw new Error('必须填写发放原因与依据 (Required reason)');
    }
    if (!payload.amount || payload.amount <= 0) {
      throw new Error('发放金额必须为正数');
    }
    const idempotencyKey = payload.idempotency_key || `idem_grant_${Date.now()}`;

    try {
      const response = await apiClient.post<{ wallet: Wallet }>(`/tenants/${tenantId}/wallets/admin-grant`, {
        amount: payload.amount,
        reason: payload.reason.trim(),
        idempotency_key: idempotencyKey
      });
      return response.data.wallet;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '管理员发放失败');
      }
      // Mock fallback
      const w = MOCK_WALLETS[tenantId];
      if (w) {
        w.available_cache += payload.amount;
        w.version++;
        w.updated_at = new Date().toISOString();
        if (w.status === 'frozen_arrears' && w.available_cache >= 0) {
          w.status = 'active';
        }
        const topupOrder: TopUpOrder = {
          topup_id: `top_mock_${Date.now()}`,
          wallet_id: w.wallet_id,
          tenant_id: tenantId,
          channel: 'admin_grant',
          amount: payload.amount,
          asset: w.asset,
          status: 'confirmed',
          external_ref: idempotencyKey,
          reason: payload.reason.trim(),
          operator_id: 'usr_current',
          created_at: new Date().toISOString(),
          confirmed_at: new Date().toISOString()
        };
        if (!MOCK_TOPUP_ORDERS[w.wallet_id]) {
          MOCK_TOPUP_ORDERS[w.wallet_id] = [];
        }
        MOCK_TOPUP_ORDERS[w.wallet_id].unshift(topupOrder);

        const ledger: LedgerEntry = {
          entry_id: `ent_mock_${Date.now()}`,
          wallet_id: w.wallet_id,
          tenant_id: tenantId,
          type: 'topup_credit',
          amount: payload.amount,
          balance_after: w.available_cache,
          idempotency_key: `top_${topupOrder.topup_id}`,
          ref_type: 'TopUpOrder',
          ref_id: topupOrder.topup_id,
          operator_id: 'usr_current',
          reason: payload.reason.trim(),
          created_at: new Date().toISOString()
        };
        if (!MOCK_LEDGER_ENTRIES[w.wallet_id]) {
          MOCK_LEDGER_ENTRIES[w.wallet_id] = [];
        }
        MOCK_LEDGER_ENTRIES[w.wallet_id].unshift(ledger);
        return { ...w };
      }
      throw err;
    }
  },

  async adjustBalance(walletId: string, payload: AdjustWalletPayload): Promise<Wallet> {
    if (!payload.reason || !payload.reason.trim()) {
      throw new Error('必须填写调账原因与审计说明 (Required reason)');
    }
    if (!payload.amount || payload.amount === 0) {
      throw new Error('调账金额不能为 0');
    }
    const idempotencyKey = payload.idempotency_key || `idem_adj_${Date.now()}`;

    try {
      const response = await apiClient.post<{ wallet: Wallet }>(`/wallets/${walletId}/adjust`, {
        amount: payload.amount,
        reason: payload.reason.trim(),
        idempotency_key: idempotencyKey
      });
      return response.data.wallet;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '调账操作失败');
      }
      // Mock fallback
      const w = Object.values(MOCK_WALLETS).find((item) => item.wallet_id === walletId);
      if (w) {
        w.available_cache += payload.amount;
        w.version++;
        w.updated_at = new Date().toISOString();
        if (w.status === 'frozen_arrears' && w.available_cache >= 0) {
          w.status = 'active';
        }
        const ledger: LedgerEntry = {
          entry_id: `ent_mock_${Date.now()}`,
          wallet_id: w.wallet_id,
          tenant_id: w.tenant_id,
          type: 'adjustment',
          amount: payload.amount,
          balance_after: w.available_cache,
          idempotency_key: idempotencyKey,
          ref_type: 'Adjustment',
          ref_id: idempotencyKey,
          operator_id: 'usr_current',
          reason: payload.reason.trim(),
          created_at: new Date().toISOString()
        };
        if (!MOCK_LEDGER_ENTRIES[w.wallet_id]) {
          MOCK_LEDGER_ENTRIES[w.wallet_id] = [];
        }
        MOCK_LEDGER_ENTRIES[w.wallet_id].unshift(ledger);
        return { ...w };
      }
      throw err;
    }
  },

  async freezeWallet(walletId: string, payload: FreezeWalletPayload): Promise<Wallet> {
    if (!payload.reason || !payload.reason.trim()) {
      throw new Error('冻结必须填写原因与说明');
    }
    try {
      const response = await apiClient.post<{ wallet: Wallet }>(`/wallets/${walletId}/freeze`, {
        reason: payload.reason.trim()
      });
      return response.data.wallet;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '冻结操作失败');
      }
      // Mock fallback
      const w = Object.values(MOCK_WALLETS).find((item) => item.wallet_id === walletId);
      if (w) {
        w.status = 'frozen';
        w.updated_at = new Date().toISOString();
        return { ...w };
      }
      throw err;
    }
  },

  async unfreezeWallet(walletId: string, payload: FreezeWalletPayload): Promise<Wallet> {
    try {
      const response = await apiClient.post<{ wallet: Wallet }>(`/wallets/${walletId}/unfreeze`, {
        reason: payload.reason.trim()
      });
      return response.data.wallet;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '解冻操作失败');
      }
      // Mock fallback
      const w = Object.values(MOCK_WALLETS).find((item) => item.wallet_id === walletId);
      if (w) {
        if (w.status === 'frozen_arrears' && w.available_cache < 0) {
          throw new Error('账户处于欠费状态 (余额小于0)，必须先充值平账补足负余额后方可解冻 (W6)');
        }
        w.status = 'active';
        w.updated_at = new Date().toISOString();
        return { ...w };
      }
      throw err;
    }
  },

  async reconcileWallet(walletId: string): Promise<ReconciliationReport> {
    try {
      const response = await apiClient.post<{ recalculation: ReconciliationReport }>(`/wallets/${walletId}/recalculate`);
      return response.data.recalculation;
    } catch {
      const w = Object.values(MOCK_WALLETS).find((item) => item.wallet_id === walletId);
      const entries = MOCK_LEDGER_ENTRIES[walletId] || [];
      const sumLedger = entries.reduce((acc, curr) => acc + curr.amount, 0);
      const available = w ? w.available_cache : 0;
      const held = w ? w.held_cache : 0;
      const total = available + held;
      const diff = sumLedger - total;
      return {
        wallet_id: walletId,
        available_cache: available,
        held_cache: held,
        total_in_wallet: total,
        sum_ledger_entries: sumLedger,
        sum_active_holds: held,
        is_balanced: diff === 0,
        discrepancy: diff
      };
    }
  }
};

export const agentApi = {
  async listAgents(tenantId: string, projectId?: string, status?: string): Promise<Agent[]> {
    try {
      const params: Record<string, string> = {};
      if (projectId) params.project_id = projectId;
      if (status && status !== 'all') params.status = status;
      const response = await apiClient.get<{ agents: Agent[] }>(`/tenants/${tenantId}/agents`, { params });
      return response.data.agents || [];
    } catch {
      let list = MOCK_AGENTS[tenantId] ? [...MOCK_AGENTS[tenantId]] : [];
      if (projectId) {
        list = list.filter((a) => a.project_id === projectId);
      }
      if (status && status !== 'all') {
        list = list.filter((a) => a.status === status);
      }
      return list;
    }
  },

  async getAgent(tenantId: string, agentId: string): Promise<Agent> {
    try {
      const response = await apiClient.get<{ agent: Agent }>(`/agents/${agentId}`);
      return response.data.agent;
    } catch {
      const list = MOCK_AGENTS[tenantId] || [];
      const found = list.find((a) => a.agent_id === agentId);
      if (found) return { ...found };
      throw new Error('Agent 未找到');
    }
  },

  async createAgent(tenantId: string, payload: CreateAgentPayload): Promise<Agent> {
    if (!payload.name || !payload.name.trim()) {
      throw new Error('Agent 名称不能为空');
    }
    if (!payload.project_id || !payload.project_id.trim()) {
      throw new Error('必须选择归属业务项目');
    }

    try {
      const policyConfig = {
        ...(payload.policy_config || {}),
        kya: payload.kya_profile
      };
      const response = await apiClient.post<{ agent: Agent }>(`/tenants/${tenantId}/agents`, {
        project_id: payload.project_id,
        name: payload.name.trim(),
        policy_config: policyConfig
      });
      return response.data.agent;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '创建 Agent 失败');
      }
      // Mock fallback
      const newAgent: Agent = {
        agent_id: `agt_${Date.now()}`,
        tenant_id: tenantId,
        project_id: payload.project_id,
        name: payload.name.trim(),
        status: 'active',
        created_at: Math.floor(Date.now() / 1000),
        updated_at: Math.floor(Date.now() / 1000),
        requests_count: 0,
        total_spend_base_units: 0,
        reject_rate: '0.0%',
        policy_config: payload.policy_config,
        kya_profile: payload.kya_profile
      };
      if (!MOCK_AGENTS[tenantId]) {
        MOCK_AGENTS[tenantId] = [];
      }
      MOCK_AGENTS[tenantId].unshift(newAgent);
      return { ...newAgent };
    }
  },

  async updateAgent(tenantId: string, agentId: string, payload: UpdateAgentPayload): Promise<Agent> {
    try {
      const policyConfig = {
        ...(payload.policy_config || {}),
        kya: payload.kya_profile
      };
      const response = await apiClient.patch<{ agent: Agent }>(`/agents/${agentId}`, {
        name: payload.name,
        policy_config: policyConfig
      });
      return response.data.agent;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '更新 Agent 失败');
      }
      // Mock fallback
      const list = MOCK_AGENTS[tenantId] || [];
      const item = list.find((a) => a.agent_id === agentId);
      if (item) {
        if (payload.name) item.name = payload.name;
        if (payload.policy_config) item.policy_config = payload.policy_config;
        if (payload.kya_profile) item.kya_profile = payload.kya_profile;
        item.updated_at = Math.floor(Date.now() / 1000);
        return { ...item };
      }
      throw err;
    }
  },

  async killAgent(
    tenantId: string,
    agentId: string,
    payload: KillAgentPayload
  ): Promise<{ agent: Agent; revoked_keys?: string[]; killed: boolean }> {
    if (!payload.reason || !payload.reason.trim()) {
      throw new Error('一键停用必须填写停用原因与说明 (Required reason)');
    }

    try {
      const response = await apiClient.post<{ agent: Agent; revoked_keys?: string[]; killed: boolean }>(
        `/tenants/${tenantId}/agents/${agentId}/kill`,
        {
          reason: payload.reason.trim(),
          revoke: !!payload.revoke
        }
      );
      return response.data;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '一键停用失败');
      }
      // Mock fallback
      const list = MOCK_AGENTS[tenantId] || [];
      const item = list.find((a) => a.agent_id === agentId);
      if (item) {
        item.status = payload.revoke ? 'revoked' : 'suspended';
        item.updated_at = Math.floor(Date.now() / 1000);

        const revokedKeys: string[] = [];
        if (payload.revoke && MOCK_AGENT_KEYS[agentId]) {
          for (const k of MOCK_AGENT_KEYS[agentId]) {
            k.status = 'revoked';
            revokedKeys.push(k.key_id);
          }
        }
        return {
          agent: { ...item },
          revoked_keys: revokedKeys,
          killed: true
        };
      }
      throw err;
    }
  },

  async updateAgentStatus(tenantId: string, agentId: string, status: AgentStatus): Promise<Agent> {
    try {
      const response = await apiClient.post<{ agent: Agent }>(`/tenants/${tenantId}/agents/${agentId}/status`, {
        status
      });
      return response.data.agent;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '更新状态失败');
      }
      const list = MOCK_AGENTS[tenantId] || [];
      const item = list.find((a) => a.agent_id === agentId);
      if (item) {
        item.status = status;
        item.updated_at = Math.floor(Date.now() / 1000);
        return { ...item };
      }
      throw err;
    }
  },

  async listAgentKeys(tenantId: string, agentId: string): Promise<AgentAPIKey[]> {
    try {
      const response = await apiClient.get<{ api_keys: AgentAPIKey[] }>(
        `/tenants/${tenantId}/api-keys?agent_id=${agentId}`
      );
      return response.data.api_keys || [];
    } catch {
      return MOCK_AGENT_KEYS[agentId] ? [...MOCK_AGENT_KEYS[agentId]] : [];
    }
  },

  async createAgentKey(
    tenantId: string,
    agentId: string,
    payload: CreateAgentKeyPayload
  ): Promise<{ api_key: AgentAPIKey; raw_key?: string }> {
    try {
      const response = await apiClient.post<{ api_key: AgentAPIKey; raw_key?: string }>(
        `/tenants/${tenantId}/api-keys`,
        {
          agent_id: agentId,
          project_id: payload.project_id,
          name: payload.name,
          scopes: payload.scopes || ['route:invoke'],
          expires_at: payload.expires_at
        }
      );
      return response.data;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '创建凭证密钥失败');
      }
      // Mock fallback
      const keyId = `key_${Date.now()}`;
      const prefix = `agp_live_${keyId.slice(-6)}`;
      const newKey: AgentAPIKey = {
        key_id: keyId,
        tenant_id: tenantId,
        project_id: payload.project_id,
        agent_id: agentId,
        name: payload.name,
        key_prefix: prefix,
        scopes: payload.scopes || ['route:invoke'],
        status: 'active',
        created_at: Math.floor(Date.now() / 1000)
      };
      if (!MOCK_AGENT_KEYS[agentId]) {
        MOCK_AGENT_KEYS[agentId] = [];
      }
      MOCK_AGENT_KEYS[agentId].unshift(newKey);
      return {
        api_key: newKey,
        raw_key: `${prefix}_mock_secret_key_${Date.now()}`
      };
    }
  },

  async revokeAgentKey(tenantId: string, keyId: string): Promise<AgentAPIKey> {
    try {
      const response = await apiClient.post<{ api_key: AgentAPIKey }>(`/tenants/${tenantId}/api-keys/${keyId}/revoke`);
      return response.data.api_key;
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data?.error) {
        throw new Error(err.response.data.error.message || '撤销密钥失败');
      }
      for (const keys of Object.values(MOCK_AGENT_KEYS)) {
        const found = keys.find((k) => k.key_id === keyId);
        if (found) {
          found.status = 'revoked';
          return { ...found };
        }
      }
      throw err;
    }
  },

  // 时间线读取：如果后端有时间线读模型则直接调用，未就绪时启用预留 Mock 引擎
  async getAgentTimeline(
    tenantId: string,
    agentId: string,
    query?: AgentTimelineQuery
  ): Promise<AgentTimelineEvent[]> {
    try {
      const params: Record<string, string> = {};
      if (query?.session_id) params.session_id = query.session_id;
      if (query?.event_type) params.event_type = query.event_type;
      const response = await apiClient.get<{ count: number; items: AgentTimelineEvent[] }>(
        `/tenants/${tenantId}/agents/${agentId}/timeline`,
        { params }
      );
      if (response.data?.items) {
        return response.data.items;
      }
    } catch {
      // fallback to Mock Timeline
    }

    let events = MOCK_AGENT_TIMELINE[agentId] ? [...MOCK_AGENT_TIMELINE[agentId]] : [];
    if (query?.session_id && query.session_id !== 'all') {
      events = events.filter((e) => e.declared?.session_id === query.session_id);
    }
    if (query?.event_type && query.event_type !== 'all') {
      events = events.filter((e) => e.event_type === query.event_type);
    }
    return events;
  }
};

export const paymentApi = {
  async listPayments(tenantId: string, query?: PaymentAuditQuery): Promise<PaymentListResponse> {
    try {
      const params: Record<string, any> = {};
      if (query?.project_id && query.project_id !== 'all') params.project_id = query.project_id;
      if (query?.agent_id && query.agent_id !== 'all') params.agent_id = query.agent_id;
      if (query?.route_id) params.route_id = query.route_id;
      if (query?.payment_method && query.payment_method !== 'all') params.payment_method = query.payment_method;
      if (query?.display_status && query.display_status !== 'all') params.display_status = query.display_status;
      if (query?.intent_status && query.intent_status !== 'all') params.intent_status = query.intent_status;
      if (query?.start_time) params.start_time = query.start_time;
      if (query?.end_time) params.end_time = query.end_time;
      if (query?.limit !== undefined) params.limit = query.limit;
      if (query?.offset !== undefined) params.offset = query.offset;

      const response = await apiClient.get<PaymentListResponse>(`/tenants/${tenantId}/payments`, { params });
      return response.data;
    } catch {
      // Mock fallback
      let list = MOCK_PAYMENTS[tenantId] ? [...MOCK_PAYMENTS[tenantId]] : [];
      if (query?.project_id && query.project_id !== 'all') {
        list = list.filter((p) => p.project_id === query.project_id);
      }
      if (query?.agent_id && query.agent_id !== 'all') {
        list = list.filter((p) => p.agent_id === query.agent_id);
      }
      if (query?.route_id) {
        const q = query.route_id.toLowerCase();
        list = list.filter((p) => p.route_id.toLowerCase().includes(q) || p.request_id.toLowerCase().includes(q));
      }
      if (query?.payment_method && query.payment_method !== 'all') {
        list = list.filter((p) => p.payment_method === query.payment_method);
      }
      if (query?.display_status && query.display_status !== 'all') {
        list = list.filter((p) => p.display_status === query.display_status);
      }
      if (query?.start_time) {
        const start = new Date(query.start_time).getTime();
        list = list.filter((p) => new Date(p.created_at).getTime() >= start);
      }
      if (query?.end_time) {
        const end = new Date(query.end_time).getTime();
        list = list.filter((p) => new Date(p.created_at).getTime() <= end);
      }
      const total = list.length;
      const offset = query?.offset || 0;
      const limit = query?.limit || 20;
      const paginated = list.slice(offset, offset + limit);
      return {
        payments: paginated,
        total,
        limit,
        offset
      };
    }
  },

  async getPayment(tenantId: string, intentId: string): Promise<PaymentAuditRecord> {
    try {
      const response = await apiClient.get<{ payment: PaymentAuditRecord }>(`/tenants/${tenantId}/payments/${intentId}`);
      return response.data.payment;
    } catch {
      const list = MOCK_PAYMENTS[tenantId] || [];
      const item = list.find((p) => p.intent_id === intentId);
      if (item) return item;
      throw new Error('未找到支付记录');
    }
  },

  async getPaymentByRequestId(tenantId: string, requestId: string): Promise<PaymentAuditRecord> {
    try {
      const response = await apiClient.get<{ payment: PaymentAuditRecord }>(
        `/tenants/${tenantId}/payments/by-request/${requestId}`
      );
      return response.data.payment;
    } catch {
      const list = MOCK_PAYMENTS[tenantId] || [];
      const item = list.find((p) => p.request_id === requestId);
      if (item) return item;
      throw new Error('未找到该请求对应的支付记录');
    }
  }
};

export default apiClient;


