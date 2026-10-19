import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import type { UserProfile, UserRole, LoginRequest, RegisterRequest } from '@/types/auth';
import type { Tenant } from '@/types/tenant';
import { authApi } from '@/api/client';
import { useTenantStore } from './tenant';

export const useAuthStore = defineStore('auth', () => {
  const initialToken = localStorage.getItem('agentpay_token');
  const initialUserStr = localStorage.getItem('agentpay_user');
  const initialTenantStr = localStorage.getItem('agentpay_tenant');
  const initialPermsStr = localStorage.getItem('agentpay_permissions');

  const token = ref<string | null>(initialToken);
  const currentUser = ref<UserProfile | null>(initialUserStr ? JSON.parse(initialUserStr) : null);
  const currentTenant = ref<Tenant | null>(initialTenantStr ? JSON.parse(initialTenantStr) : null);
  const permissions = ref<string[]>(initialPermsStr ? JSON.parse(initialPermsStr) : []);

  const isAuthenticated = computed(() => !!token.value);
  const currentRole = computed<UserRole | null>(() => currentUser.value?.role || null);

  function setSession(newToken: string, user: UserProfile, tenant?: Tenant, perms?: string[]) {
    token.value = newToken;
    currentUser.value = user;
    localStorage.setItem('agentpay_token', newToken);
    localStorage.setItem('agentpay_user', JSON.stringify(user));

    if (tenant) {
      currentTenant.value = tenant;
      localStorage.setItem('agentpay_tenant', JSON.stringify(tenant));
      localStorage.setItem('agentpay_tenant_id', tenant.tenant_id);

      // Inform tenantStore
      const tenantStore = useTenantStore();
      if (!tenantStore.tenants.some((t) => t.tenant_id === tenant.tenant_id)) {
        tenantStore.tenants.unshift(tenant);
      }
      tenantStore.currentTenantId = tenant.tenant_id;
    } else if (user.tenant_id) {
      localStorage.setItem('agentpay_tenant_id', user.tenant_id);
    }

    if (perms) {
      permissions.value = perms;
      localStorage.setItem('agentpay_permissions', JSON.stringify(perms));
    }
  }

  function clearSession() {
    token.value = null;
    currentUser.value = null;
    currentTenant.value = null;
    permissions.value = [];
    localStorage.removeItem('agentpay_token');
    localStorage.removeItem('agentpay_user');
    localStorage.removeItem('agentpay_tenant');
    localStorage.removeItem('agentpay_permissions');
  }

  async function login(payload: LoginRequest) {
    const res = await authApi.login(payload);
    setSession(res.token, res.user, res.tenant);

    // After login, try to fetch fresh permissions & profile
    try {
      const me = await authApi.getCurrentUser();
      if (me) {
        setSession(res.token, me.user, me.tenant, me.permissions);
      }
    } catch {
      // ignore
    }

    return res.user;
  }

  async function register(payload: RegisterRequest) {
    const res = await authApi.register(payload);
    setSession(res.token, res.user, res.tenant);

    try {
      const me = await authApi.getCurrentUser();
      if (me) {
        setSession(res.token, me.user, me.tenant, me.permissions);
      }
    } catch {
      // ignore
    }

    return res;
  }

  function logout() {
    clearSession();
  }

  function hasRole(allowedRoles?: UserRole[]): boolean {
    if (!allowedRoles || allowedRoles.length === 0) return true;
    if (!currentUser.value) return false;
    return allowedRoles.includes(currentUser.value.role);
  }

  function hasPermission(permission: string): boolean {
    if (currentRole.value === 'owner') return true;
    return permissions.value.includes(permission);
  }

  return {
    token,
    currentUser,
    currentTenant,
    permissions,
    isAuthenticated,
    currentRole,
    login,
    register,
    logout,
    hasRole,
    hasPermission,
    setSession,
    clearSession
  };
});
