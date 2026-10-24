import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { SUPPORTED_CURRENCIES, type Tenant, type Project, type CurrencyCode } from '@/types/tenant';
import { tenantApi, projectApi, type CreateProjectPayload, type UpdateProjectPayload } from '@/api/client';
import { MOCK_TENANTS, MOCK_PROJECTS } from '@/api/mockData';

export const useTenantStore = defineStore('tenant', () => {
  const tenants = ref<Tenant[]>([...MOCK_TENANTS]);
  const projects = ref<Project[]>([...MOCK_PROJECTS]);

  const savedTenantId = localStorage.getItem('agentpay_tenant_id') || MOCK_TENANTS[0].tenant_id;
  const currentTenantId = ref<string>(savedTenantId);

  const availableProjects = computed(() => {
    return projects.value.filter((p) => p.tenant_id === currentTenantId.value);
  });

  const savedProjectId = localStorage.getItem('agentpay_project_id');
  const defaultProjectId = availableProjects.value[0]?.project_id || '';
  const currentProjectId = ref<string>(
    availableProjects.value.some((p) => p.project_id === savedProjectId)
      ? (savedProjectId as string)
      : defaultProjectId
  );

  const currentTenant = computed(() => {
    return tenants.value.find((t) => t.tenant_id === currentTenantId.value) || null;
  });

  const currentProject = computed(() => {
    return projects.value.find((p) => p.project_id === currentProjectId.value) || null;
  });

  const currentCurrency = computed<CurrencyCode>(() => {
    const c = currentTenant.value?.currency;
    if (c && c in SUPPORTED_CURRENCIES) {
      return c;
    }
    return 'USD';
  });

  const currencySymbol = computed<string>(() => {
    return SUPPORTED_CURRENCIES[currentCurrency.value]?.symbol || '$';
  });

  function formatAmount(baseUnits: number | undefined, minDecimals = 4): string {
    if (baseUnits === undefined || isNaN(baseUnits)) return '0.0000';
    const val = baseUnits / 1000000;
    return val.toLocaleString('en-US', {
      minimumFractionDigits: minDecimals,
      maximumFractionDigits: 6
    });
  }

  function formatCurrency(baseUnits: number | undefined, showSymbol = false): string {
    const numStr = formatAmount(baseUnits, 4);
    if (showSymbol) {
      return `${currencySymbol.value}${numStr} ${currentCurrency.value}`;
    }
    return `${numStr} ${currentCurrency.value}`;
  }

  async function loadTenants() {
    try {
      const data = await tenantApi.getTenants();
      if (Array.isArray(data) && data.length > 0) {
        tenants.value = data;
        if (!tenants.value.some((t) => t.tenant_id === currentTenantId.value)) {
          await switchTenant(tenants.value[0].tenant_id);
        }
      }
    } catch {
      // fallback to mock
    }
  }

  async function loadProjects() {
    if (!currentTenantId.value) return;
    try {
      const data = await tenantApi.getProjects(currentTenantId.value);
      if (Array.isArray(data)) {
        const otherProjects = projects.value.filter((p) => p.tenant_id !== currentTenantId.value);
        projects.value = [...otherProjects, ...data];

        if (!availableProjects.value.some((p) => p.project_id === currentProjectId.value)) {
          if (availableProjects.value.length > 0) {
            switchProject(availableProjects.value[0].project_id);
          } else {
            currentProjectId.value = '';
            localStorage.removeItem('agentpay_project_id');
          }
        }
      }
    } catch {
      // fallback to mock
    }
  }

  async function switchTenant(tenantId: string) {
    currentTenantId.value = tenantId;
    localStorage.setItem('agentpay_tenant_id', tenantId);

    const tenantProjects = projects.value.filter((p) => p.tenant_id === tenantId);
    if (tenantProjects.length > 0) {
      switchProject(tenantProjects[0].project_id);
    } else {
      currentProjectId.value = '';
      localStorage.removeItem('agentpay_project_id');
    }

    await loadProjects();
  }

  function upsertTenant(tenant: Tenant) {
    const idx = tenants.value.findIndex((t) => t.tenant_id === tenant.tenant_id);
    if (idx !== -1) {
      tenants.value[idx] = { ...tenants.value[idx], ...tenant };
    } else {
      tenants.value.unshift(tenant);
    }
  }

  async function updateTenantCurrency(currency: CurrencyCode): Promise<Tenant> {
    if (!currentTenantId.value) throw new Error('未选择当前租户');
    const updated = await tenantApi.updateTenantCurrency(currentTenantId.value, currency);
    upsertTenant(updated);
    return updated;
  }

  function switchProject(projectId: string) {
    currentProjectId.value = projectId;
    localStorage.setItem('agentpay_project_id', projectId);
  }

  async function createProject(payload: CreateProjectPayload): Promise<Project> {
    if (!currentTenantId.value) throw new Error('未选择当前租户');
    const prj = await projectApi.createProject(currentTenantId.value, payload);
    projects.value.push(prj);
    switchProject(prj.project_id);
    return prj;
  }

  async function updateProject(projectId: string, payload: UpdateProjectPayload): Promise<Project> {
    if (!currentTenantId.value) throw new Error('未选择当前租户');
    const updated = await projectApi.updateProject(currentTenantId.value, projectId, payload);
    const idx = projects.value.findIndex((p) => p.project_id === projectId);
    if (idx !== -1) {
      projects.value[idx] = updated;
    }
    return updated;
  }

  async function deleteProject(projectId: string): Promise<void> {
    if (!currentTenantId.value) throw new Error('未选择当前租户');
    await projectApi.deleteProject(currentTenantId.value, projectId);
    projects.value = projects.value.filter((p) => p.project_id !== projectId);
    if (currentProjectId.value === projectId) {
      if (availableProjects.value.length > 0) {
        switchProject(availableProjects.value[0].project_id);
      } else {
        currentProjectId.value = '';
        localStorage.removeItem('agentpay_project_id');
      }
    }
  }

  return {
    tenants,
    projects,
    currentTenantId,
    currentProjectId,
    currentTenant,
    currentProject,
    currentCurrency,
    currencySymbol,
    availableProjects,
    formatAmount,
    formatCurrency,
    loadTenants,
    loadProjects,
    switchTenant,
    upsertTenant,
    updateTenantCurrency,
    switchProject,
    createProject,
    updateProject,
    deleteProject
  };
});
