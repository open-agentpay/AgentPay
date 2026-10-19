import { describe, it, expect, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useAuthStore } from '../src/stores/auth';
import { useTenantStore } from '../src/stores/tenant';
import { MOCK_USERS, MOCK_TENANTS, MOCK_PROJECTS } from '../src/api/mockData';

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

describe('T4.1 Console - Auth & RBAC Store Test Suite', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
  });

  it('should initialize with no active session by default', () => {
    const authStore = useAuthStore();
    expect(authStore.isAuthenticated).toBe(false);
    expect(authStore.currentUser).toBeNull();
    expect(authStore.currentRole).toBeNull();
  });

  it('should support logging in with role-based session and persistence', async () => {
    const authStore = useAuthStore();
    await authStore.login({ email: 'admin@agentpay.io', role: 'admin' });

    expect(authStore.isAuthenticated).toBe(true);
    expect(authStore.currentRole).toBe('admin');
    expect(authStore.currentUser?.email).toBe('admin@agentpay.io');
    expect(localStorage.getItem('agentpay_token')).toBeTruthy();
  });

  it('should enforce role checks correctly via hasRole()', () => {
    const authStore = useAuthStore();
    authStore.setSession('token_viewer', MOCK_USERS.viewer);

    expect(authStore.currentRole).toBe('viewer');
    expect(authStore.hasRole(['viewer'])).toBe(true);
    expect(authStore.hasRole(['owner', 'admin'])).toBe(false);
    expect(authStore.hasRole(['operator', 'viewer'])).toBe(true);
  });

  it('should clear session upon logout', () => {
    const authStore = useAuthStore();
    authStore.setSession('token_owner', MOCK_USERS.owner);
    expect(authStore.isAuthenticated).toBe(true);

    authStore.logout();
    expect(authStore.isAuthenticated).toBe(false);
    expect(authStore.currentUser).toBeNull();
    expect(localStorage.getItem('agentpay_token')).toBeNull();
  });

  it('should store and evaluate permissions and tenant context correctly', () => {
    const authStore = useAuthStore();
    authStore.setSession(
      'token_new',
      {
        member_id: 'usr_new',
        email: 'new@corp.com',
        role: 'admin',
        status: 'active',
        tenant_id: 'ten_new'
      },
      {
        tenant_id: 'ten_new',
        name: 'New Corp',
        status: 'active'
      },
      ['tenant:read', 'project:create', 'agent:kill']
    );

    expect(authStore.isAuthenticated).toBe(true);
    expect(authStore.currentRole).toBe('admin');
    expect(authStore.currentTenant?.name).toBe('New Corp');
    expect(authStore.hasPermission('agent:kill')).toBe(true);
    expect(authStore.hasPermission('wallet:adjust')).toBe(false);
  });
});

describe('T4.1 Console - Multi-Tenant & Project Switcher Test Suite', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
  });

  it('should initialize with default tenant and correctly filtered projects', () => {
    const tenantStore = useTenantStore();
    expect(tenantStore.currentTenantId).toBe(MOCK_TENANTS[0].tenant_id);

    const expectedProjects = MOCK_PROJECTS.filter(
      (p) => p.tenant_id === MOCK_TENANTS[0].tenant_id
    );
    expect(tenantStore.availableProjects.length).toBe(expectedProjects.length);
    expect(tenantStore.currentProjectId).toBe(expectedProjects[0].project_id);
  });

  it('should filter projects dynamically when switching tenant', () => {
    const tenantStore = useTenantStore();
    const secondTenant = MOCK_TENANTS[1];

    tenantStore.switchTenant(secondTenant.tenant_id);

    expect(tenantStore.currentTenantId).toBe(secondTenant.tenant_id);
    expect(localStorage.getItem('agentpay_tenant_id')).toBe(secondTenant.tenant_id);

    const secondTenantProjects = MOCK_PROJECTS.filter(
      (p) => p.tenant_id === secondTenant.tenant_id
    );
    expect(tenantStore.availableProjects.length).toBe(secondTenantProjects.length);
    expect(tenantStore.currentProjectId).toBe(secondTenantProjects[0].project_id);
  });
});
