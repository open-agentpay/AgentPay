import { describe, it, expect, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useAuthStore } from '../src/stores/auth';
import { useTenantStore } from '../src/stores/tenant';
import { agentApi } from '../src/api/client';
import type { CreateAgentPayload, UpdateAgentPayload } from '../src/types/agent';

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

describe('T4.7 Console - Agent Management, KYA & Audit Timeline Test Suite', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
  });

  describe('1. Tenant Isolation & Agent Listing', () => {
    it('should list agents belonging to the current tenant', async () => {
      const tenantStore = useTenantStore();
      const tenantId = tenantStore.currentTenantId; // ten_9a8b7c6d5e

      const list = await agentApi.listAgents(tenantId);
      expect(list.length).toBeGreaterThanOrEqual(3);
      expect(list.every((a) => a.tenant_id === tenantId)).toBe(true);

      const spider = list.find((a) => a.agent_id === 'agt_spider_01');
      expect(spider).toBeDefined();
      expect(spider?.name).toBe('Autonomous Research Spider');
      expect(spider?.status).toBe('active');
    });

    it('should strictly isolate agents when switching tenants', async () => {
      const tenantStore = useTenantStore();

      // Tenant 1
      await tenantStore.switchTenant('ten_9a8b7c6d5e');
      const list1 = await agentApi.listAgents(tenantStore.currentTenantId);
      expect(list1.some((a) => a.agent_id === 'agt_spider_01')).toBe(true);
      expect(list1.some((a) => a.agent_id === 'agt_dev_bot_01')).toBe(false);

      // Tenant 2
      await tenantStore.switchTenant('ten_3f4e5d6c7b');
      const list2 = await agentApi.listAgents(tenantStore.currentTenantId);
      expect(list2.some((a) => a.agent_id === 'agt_dev_bot_01')).toBe(true);
      expect(list2.some((a) => a.agent_id === 'agt_spider_01')).toBe(false);
    });

    it('should support filtering agents by project and status', async () => {
      const tenantId = 'ten_9a8b7c6d5e';
      const activeList = await agentApi.listAgents(tenantId, undefined, 'active');
      expect(activeList.every((a) => a.status === 'active')).toBe(true);

      const suspendedList = await agentApi.listAgents(tenantId, undefined, 'suspended');
      expect(suspendedList.some((a) => a.agent_id === 'agt_scraper_03')).toBe(true);
    });
  });

  describe('2. KYA (Know Your Agent) Profile & Verification', () => {
    it('should retrieve full KYA profile with model provenance and accountability', async () => {
      const agent = await agentApi.getAgent('ten_9a8b7c6d5e', 'agt_spider_01');
      expect(agent.kya_profile).toBeDefined();

      const kya = agent.kya_profile!;
      expect(kya.model_provider).toBe('DeepSeek');
      expect(kya.model_version).toBe('deepseek-v3');
      expect(kya.framework).toBe('LangChain v0.3');
      expect(kya.hosting_env).toContain('TEE Confidential Cloud');
      expect(kya.owner_name).toBe('Alice Wang');
      expect(kya.owner_email).toBe('alice@agentpay.io');
      expect(kya.autonomy_level).toBe('L3_fully_autonomous');
      expect(kya.status).toBe('verified');
      expect(kya.attestation_hash).toMatch(/^0x/);
      expect(kya.allowed_tools).toContain('web_search');
    });

    it('should support creating a new Agent with KYA registration', async () => {
      const tenantId = 'ten_9a8b7c6d5e';
      const payload: CreateAgentPayload = {
        name: 'Financial Auditor Agent',
        project_id: 'prj_8e7d6c5b4a',
        policy_config: {
          budget_cap_base_units: 300000000,
          rate_limit_rpm: 45
        },
        kya_profile: {
          model_provider: 'Anthropic',
          model_version: 'claude-3-5-sonnet',
          framework: 'CrewAI',
          hosting_env: 'AWS Nitro Enclave',
          owner_name: 'David Smith',
          owner_email: 'david@agentpay.io',
          department: 'Compliance',
          authorized_by: 'Alice Wang',
          autonomy_level: 'L2_semi_autonomous',
          risk_level: 'low',
          status: 'verified',
          max_single_spend: 3000000,
          allowed_tools: ['ledger_read', 'reconcile_check']
        }
      };

      const created = await agentApi.createAgent(tenantId, payload);
      expect(created.agent_id).toMatch(/^agt_/);
      expect(created.name).toBe('Financial Auditor Agent');
      expect(created.kya_profile?.model_provider).toBe('Anthropic');
      expect(created.kya_profile?.autonomy_level).toBe('L2_semi_autonomous');
      expect(created.status).toBe('active');
    });

    it('should support updating Agent KYA profile and policy configuration', async () => {
      const tenantId = 'ten_9a8b7c6d5e';
      const updatePayload: UpdateAgentPayload = {
        name: 'Autonomous Research Spider (Updated v2)',
        policy_config: {
          budget_cap_base_units: 800000000,
          rate_limit_rpm: 100
        },
        kya_profile: {
          model_provider: 'DeepSeek',
          model_version: 'deepseek-v3',
          framework: 'LangChain v0.3',
          hosting_env: 'TEE Confidential Cloud',
          owner_name: 'Alice Wang',
          owner_email: 'alice@agentpay.io',
          department: 'Autonomous Intelligence Lab',
          authorized_by: 'Bob',
          autonomy_level: 'L3_fully_autonomous',
          risk_level: 'low',
          status: 'verified',
          notes: 'KYA 信用等级提升至低风险'
        }
      };

      const updated = await agentApi.updateAgent(tenantId, 'agt_spider_01', updatePayload);
      expect(updated.name).toBe('Autonomous Research Spider (Updated v2)');
      expect(updated.kya_profile?.notes).toContain('KYA 信用等级提升');
    });
  });

  describe('3. Kill Switch (One-Click Emergency Suspension)', () => {
    it('should require a non-empty reason when executing kill switch', async () => {
      await expect(
        agentApi.killAgent('ten_9a8b7c6d5e', 'agt_spider_01', {
          reason: '  '
        })
      ).rejects.toThrow('必须填写停用原因');
    });

    it('should immediately suspend Agent and cascade revoke keys if requested', async () => {
      const res = await agentApi.killAgent('ten_9a8b7c6d5e', 'agt_spider_01', {
        reason: '突发高频死循环异常调控紧急熔断',
        revoke: true
      });

      expect(res.killed).toBe(true);
      expect(res.agent.status).toBe('revoked');
      expect(res.revoked_keys?.length).toBeGreaterThan(0);

      // Verify keys were revoked
      const keys = await agentApi.listAgentKeys('ten_9a8b7c6d5e', 'agt_spider_01');
      expect(keys.every((k) => k.status === 'revoked')).toBe(true);
    });

    it('should support resuming an Agent from suspended status', async () => {
      const resumed = await agentApi.updateAgentStatus('ten_9a8b7c6d5e', 'agt_scraper_03', 'active');
      expect(resumed.status).toBe('active');
    });
  });

  describe('4. API Key Lifecycle Binding to Agent', () => {
    it('should list and create API keys bound to a specific Agent', async () => {
      const tenantId = 'ten_9a8b7c6d5e';
      const keys = await agentApi.listAgentKeys(tenantId, 'agt_analyst_02');
      expect(keys.length).toBeGreaterThan(0);

      const created = await agentApi.createAgentKey(tenantId, 'agt_analyst_02', {
        project_id: 'prj_8e7d6c5b4a',
        name: 'Secondary Worker Token',
        scopes: ['route:invoke', 'wallet:spend']
      });

      expect(created.api_key.name).toBe('Secondary Worker Token');
      expect(created.api_key.agent_id).toBe('agt_analyst_02');
      expect(created.raw_key).toBeDefined();

      const updatedKeys = await agentApi.listAgentKeys(tenantId, 'agt_analyst_02');
      expect(updatedKeys.some((k) => k.name === 'Secondary Worker Token')).toBe(true);
    });

    it('should revoke an API key', async () => {
      const tenantId = 'ten_9a8b7c6d5e';
      const keys = await agentApi.listAgentKeys(tenantId, 'agt_analyst_02');
      const target = keys[0];

      const revoked = await agentApi.revokeAgentKey(tenantId, target.key_id);
      expect(revoked.status).toBe('revoked');
    });
  });

  describe('5. Agent Audit Timeline (T1.8 Spec & T5.8 Reserved)', () => {
    it('should retrieve canonical timeline events adhering to observed vs declared separation', async () => {
      const tenantId = 'ten_9a8b7c6d5e';
      const events = await agentApi.getAgentTimeline(tenantId, 'agt_spider_01');
      expect(events.length).toBeGreaterThan(0);

      const requestEvt = events.find((e) => e.event_type === 'agent.request');
      expect(requestEvt).toBeDefined();

      // Observed fields (trusted gateway facts)
      expect(requestEvt?.observed.status_code).toBe(200);
      expect(requestEvt?.observed.latency_ms).toBeGreaterThan(0);
      expect(requestEvt?.observed.cost_base_units).toBe(50000); // 0.05 USDC
      expect(requestEvt?.observed.payment_intent_id).toBe('pi_7a8b9c0d1e');
      expect(requestEvt?.observed.request_hash).toMatch(/^sha256:/);

      // Declared fields (untrusted agent self-declaration)
      expect(requestEvt?.declared?.session_id).toBe('sess_research_alpha');
      expect(requestEvt?.declared?.task_id).toBe('task_crawl_arxiv_ai');
    });

    it('should support aggregating timeline events by declared session_id', async () => {
      const tenantId = 'ten_9a8b7c6d5e';
      const sessionEvents = await agentApi.getAgentTimeline(tenantId, 'agt_spider_01', {
        session_id: 'sess_research_alpha'
      });

      expect(sessionEvents.length).toBeGreaterThan(0);
      expect(sessionEvents.every((e) => e.declared?.session_id === 'sess_research_alpha')).toBe(true);
    });

    it('should support filtering timeline by canonical event types', async () => {
      const tenantId = 'ten_9a8b7c6d5e';
      const spendEvents = await agentApi.getAgentTimeline(tenantId, 'agt_spider_01', {
        event_type: 'agent.spend'
      });

      expect(spendEvents.length).toBeGreaterThan(0);
      expect(spendEvents.every((e) => e.event_type === 'agent.spend')).toBe(true);
    });
  });

  describe('6. RBAC Role-Based Authorization Checks', () => {
    it('should grant write permissions to Owner and Admin', () => {
      const authStore = useAuthStore();

      authStore.setSession('tok_owner', {
        member_id: 'usr_o',
        email: 'owner@test.io',
        name: 'Owner',
        role: 'owner',
        status: 'active',
        tenant_id: 'ten_9a8b7c6d5e'
      });
      expect(authStore.hasRole(['owner', 'admin'])).toBe(true);

      authStore.setSession('tok_admin', {
        member_id: 'usr_a',
        email: 'admin@test.io',
        name: 'Admin',
        role: 'admin',
        status: 'active',
        tenant_id: 'ten_9a8b7c6d5e'
      });
      expect(authStore.hasRole(['owner', 'admin'])).toBe(true);
    });

    it('should restrict Operator and Viewer from Agent write actions', () => {
      const authStore = useAuthStore();

      authStore.setSession('tok_op', {
        member_id: 'usr_op',
        email: 'op@test.io',
        name: 'Operator',
        role: 'operator',
        status: 'active',
        tenant_id: 'ten_9a8b7c6d5e'
      });
      expect(authStore.hasRole(['owner', 'admin'])).toBe(false);

      authStore.setSession('tok_view', {
        member_id: 'usr_v',
        email: 'viewer@test.io',
        name: 'Viewer',
        role: 'viewer',
        status: 'active',
        tenant_id: 'ten_9a8b7c6d5e'
      });
      expect(authStore.hasRole(['owner', 'admin'])).toBe(false);
    });
  });
});
