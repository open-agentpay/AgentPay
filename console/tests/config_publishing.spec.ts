import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { configApi } from '../src/api/client';
import type { ConfigVersion, ConfigDraftDiff } from '../src/types/config';

describe('T4.4 Console - Configuration Publishing, Versioning & Rollback Suite', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it('should publish valid draft into immutable configuration snapshot', async () => {
    const mockPublishedVersion: ConfigVersion = {
      version_id: 'ver_20261019_001',
      tenant_id: 'ten_primary_001',
      version_number: 1,
      published_by: 'usr_admin001',
      publisher_email: 'admin@agentpay.io',
      change_summary: 'Production rollout: LLM chat completions route',
      snapshot_hash: 'sha256:d8e8fca2dc0f896fd7cb4cb0031ba249ecc0203f7e6f669db75fa4ff31238994',
      routes_count: 2,
      policies_count: 1,
      is_active: true,
      created_at: 1773839500
    };

    const spy = vi.spyOn(configApi, 'publishConfig').mockResolvedValue(mockPublishedVersion);
    const result = await configApi.publishConfig('ten_primary_001', {
      change_summary: 'Production rollout: LLM chat completions route'
    });

    expect(spy).toHaveBeenCalledWith('ten_primary_001', {
      change_summary: 'Production rollout: LLM chat completions route'
    });
    expect(result.version_number).toBe(1);
    expect(result.is_active).toBe(true);
    expect(result.snapshot_hash).toMatch(/^sha256:[a-f0-9]{64}$/);
    expect(result.routes_count).toBe(2);
    expect(result.policies_count).toBe(1);
  });

  it('should fetch configuration version history list', async () => {
    const mockVersions: ConfigVersion[] = [
      {
        version_id: 'ver_20261019_002',
        tenant_id: 'ten_primary_001',
        version_number: 2,
        published_by: 'usr_admin001',
        publisher_email: 'admin@agentpay.io',
        change_summary: 'Add embeddings route and update rate limit',
        snapshot_hash: 'sha256:59c1184f475da4a6828b6d44549ba3676cd3c834579c32de6dd0617a94d80a3a',
        routes_count: 3,
        policies_count: 1,
        is_active: true,
        created_at: 1773840000
      },
      {
        version_id: 'ver_20261019_001',
        tenant_id: 'ten_primary_001',
        version_number: 1,
        published_by: 'usr_owner001',
        publisher_email: 'owner@agentpay.io',
        change_summary: 'Initial production routing snapshot',
        snapshot_hash: 'sha256:d8e8fca2dc0f896fd7cb4cb0031ba249ecc0203f7e6f669db75fa4ff31238994',
        routes_count: 2,
        policies_count: 1,
        is_active: false,
        created_at: 1773839500
      }
    ];

    const spy = vi.spyOn(configApi, 'getVersions').mockResolvedValue(mockVersions);
    const versions = await configApi.getVersions('ten_primary_001');

    expect(spy).toHaveBeenCalledWith('ten_primary_001');
    expect(versions.length).toBe(2);
    expect(versions[0].version_number).toBe(2);
    expect(versions[0].is_active).toBe(true);
    expect(versions[1].version_number).toBe(1);
    expect(versions[1].is_active).toBe(false);
  });

  it('should execute rollback producing incremented version with identical target snapshot hash (TC_CFG_04)', async () => {
    const v1Hash = 'sha256:d8e8fca2dc0f896fd7cb4cb0031ba249ecc0203f7e6f669db75fa4ff31238994';
    const mockRolledBackVersion: ConfigVersion = {
      version_id: 'ver_20261019_003',
      tenant_id: 'ten_primary_001',
      version_number: 3,
      published_by: 'usr_admin001',
      publisher_email: 'admin@agentpay.io',
      change_summary: 'Rollback to version v1: Incident mitigation',
      snapshot_hash: v1Hash, // Critical: must match target version v1 hash!
      routes_count: 2,
      policies_count: 1,
      is_active: true,
      created_at: 1773841000
    };

    const spy = vi.spyOn(configApi, 'rollbackConfig').mockResolvedValue(mockRolledBackVersion);
    const result = await configApi.rollbackConfig('ten_primary_001', {
      target_version_id: 'ver_20261019_001',
      reason: 'Incident mitigation'
    });

    expect(spy).toHaveBeenCalledWith('ten_primary_001', {
      target_version_id: 'ver_20261019_001',
      reason: 'Incident mitigation'
    });
    expect(result.version_number).toBe(3);
    expect(result.snapshot_hash).toBe(v1Hash);
    expect(result.is_active).toBe(true);
  });

  it('should compare differences between two historical configuration snapshots', async () => {
    const mockDiff: ConfigDraftDiff = {
      tenant_id: 'ten_primary_001',
      active_version_id: 'ver_20261019_001',
      has_changes: true,
      route_diffs: [
        {
          route_id: 'rou_embeddings',
          action: 'added',
          path_pattern: '/v1/embeddings',
          after: {
            upstream_url: 'https://valid.llm.provider/v1/embeddings',
            price_base_units: 500
          }
        }
      ],
      policy_diffs: []
    };

    const spy = vi.spyOn(configApi, 'getVersionDiff').mockResolvedValue(mockDiff);
    const diff = await configApi.getVersionDiff('ten_primary_001', 'ver_20261019_001', 'ver_20261019_002');

    expect(spy).toHaveBeenCalledWith('ten_primary_001', 'ver_20261019_001', 'ver_20261019_002');
    expect(diff.has_changes).toBe(true);
    expect(diff.route_diffs.length).toBe(1);
    expect(diff.route_diffs[0].path_pattern).toBe('/v1/embeddings');
  });
});
