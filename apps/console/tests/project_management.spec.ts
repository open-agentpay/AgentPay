import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useTenantStore } from '../src/stores/tenant';
import { projectApi } from '../src/api/client';
import type { Project } from '../src/types/tenant';

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

describe('T4.1/T4.2 Console - Project Management & Store Sync Test Suite', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
  });

  it('should initialize and filter projects by current tenant', async () => {
    const tenantStore = useTenantStore();
    expect(tenantStore.availableProjects.length).toBeGreaterThan(0);
    const firstPrj = tenantStore.availableProjects[0];
    expect(firstPrj.tenant_id).toBe(tenantStore.currentTenantId);
  });

  it('should support creating a new real project via projectApi and sync store', async () => {
    const tenantStore = useTenantStore();
    const mockCreatedProject: Project = {
      project_id: 'prj_live_market',
      tenant_id: tenantStore.currentTenantId,
      name: '智能高频行情服务',
      budget_cap_base_units: 500000000,
      status: 'active'
    };

    const createSpy = vi.spyOn(projectApi, 'createProject').mockResolvedValue(mockCreatedProject);

    const prj = await tenantStore.createProject({
      name: '智能高频行情服务',
      budget_cap_base_units: 500000000
    });

    expect(createSpy).toHaveBeenCalledWith(tenantStore.currentTenantId, {
      name: '智能高频行情服务',
      budget_cap_base_units: 500000000
    });
    expect(prj.project_id).toBe('prj_live_market');
    expect(tenantStore.currentProjectId).toBe('prj_live_market');
    expect(tenantStore.availableProjects.some((p) => p.project_id === 'prj_live_market')).toBe(true);
  });

  it('should support updating project name and budget cap', async () => {
    const tenantStore = useTenantStore();
    const mockUpdatedProject: Project = {
      project_id: 'prj_live_market',
      tenant_id: tenantStore.currentTenantId,
      name: '智能高频行情服务 (升级版)',
      budget_cap_base_units: 800000000,
      status: 'active'
    };

    const updateSpy = vi.spyOn(projectApi, 'updateProject').mockResolvedValue(mockUpdatedProject);

    const updated = await tenantStore.updateProject('prj_live_market', {
      name: '智能高频行情服务 (升级版)',
      budget_cap_base_units: 800000000
    });

    expect(updateSpy).toHaveBeenCalled();
    expect(updated.name).toBe('智能高频行情服务 (升级版)');
  });

  it('should support deleting a project and falling back to remaining projects', async () => {
    const tenantStore = useTenantStore();
    const deleteSpy = vi.spyOn(projectApi, 'deleteProject').mockResolvedValue();

    // Set current to a test project
    tenantStore.currentProjectId = 'prj_to_delete';
    tenantStore.projects.push({
      project_id: 'prj_to_delete',
      tenant_id: tenantStore.currentTenantId,
      name: '待删除临时项目',
      budget_cap_base_units: 10000,
      status: 'active'
    });

    await tenantStore.deleteProject('prj_to_delete');
    expect(deleteSpy).toHaveBeenCalledWith(tenantStore.currentTenantId, 'prj_to_delete');
    expect(tenantStore.projects.some((p) => p.project_id === 'prj_to_delete')).toBe(false);
  });
});
