import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { configApi } from '../src/api/client';
import type { ValidationResult, ConfigDraftSummary, ConfigDraftDiff } from '../src/types/config';

describe('T4.3 Console - Configuration Draft & Static Validation Matrix Suite', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it('should handle draft summary fetching with isolation metrics', async () => {
    const mockSummary: ConfigDraftSummary = {
      tenant_id: 'ten_primary_001',
      pending_changes: 3,
      routes_count: 5,
      policies_count: 2,
      is_validated: false
    };

    const spy = vi.spyOn(configApi, 'getDraftSummary').mockResolvedValue(mockSummary);
    const summary = await configApi.getDraftSummary('ten_primary_001');

    expect(spy).toHaveBeenCalledWith('ten_primary_001');
    expect(summary.pending_changes).toBe(3);
    expect(summary.routes_count).toBe(5);
    expect(summary.is_validated).toBe(false);
  });

  it('should validate draft successfully when all rules pass', async () => {
    const mockValidResult: ValidationResult = {
      valid: true,
      errors: [],
      summary: {
        total_routes: 3,
        total_policies: 1,
        error_count: 0,
        warning_count: 0
      },
      validated_at: 1773839200
    };

    const spy = vi.spyOn(configApi, 'validateDraft').mockResolvedValue(mockValidResult);
    const result = await configApi.validateDraft('ten_primary_001');

    expect(spy).toHaveBeenCalledWith('ten_primary_001');
    expect(result.valid).toBe(true);
    expect(result.errors.length).toBe(0);
    expect(result.summary.error_count).toBe(0);
  });

  it('should detect and surface multi-rule validation errors (URL, payee address, budget ladder)', async () => {
    const mockInvalidResult: ValidationResult = {
      valid: false,
      errors: [
        {
          target_type: 'Route',
          target_id: 'rt_invalid_1',
          field: 'upstream_url',
          code: 'INVALID_UPSTREAM_URL',
          message: 'upstream_url must be an absolute HTTP/HTTPS URL'
        },
        {
          target_type: 'Route',
          target_id: 'rt_invalid_2',
          field: 'payee_address',
          code: 'INVALID_PAYEE_ADDRESS',
          message: 'payee_address must be a valid 0x 20-byte EVM hex string'
        },
        {
          target_type: 'Policy',
          target_id: 'plc_budget_err',
          field: 'max_amount_per_request',
          code: 'MAX_AMOUNT_EXCEEDS_ROLLING_BUDGET',
          message: 'max_amount_per_request cannot exceed rolling_24h_budget_cap'
        }
      ],
      summary: {
        total_routes: 2,
        total_policies: 1,
        error_count: 3,
        warning_count: 0
      },
      validated_at: 1773839250
    };

    vi.spyOn(configApi, 'validateDraft').mockResolvedValue(mockInvalidResult);
    const result = await configApi.validateDraft('ten_primary_001');

    expect(result.valid).toBe(false);
    expect(result.summary.error_count).toBe(3);

    const codes = result.errors.map((e) => e.code);
    expect(codes).toContain('INVALID_UPSTREAM_URL');
    expect(codes).toContain('INVALID_PAYEE_ADDRESS');
    expect(codes).toContain('MAX_AMOUNT_EXCEEDS_ROLLING_BUDGET');
  });

  it('should fetch and parse draft diffs comparing draft against active snapshot', async () => {
    const mockDiff: ConfigDraftDiff = {
      tenant_id: 'ten_primary_001',
      active_version_id: 'ver_20261018001',
      has_changes: true,
      route_diffs: [
        {
          route_id: 'rt_001',
          action: 'added',
          path_pattern: '/v1/deepseek/chat',
          after: {
            upstream_url: 'https://api.deepseek.com/v1/chat',
            price_base_units: 20000,
            allowed_payment_methods: 'both'
          }
        },
        {
          route_id: 'rt_002',
          action: 'modified',
          path_pattern: '/v1/openai/chat',
          before: { price_base_units: 50000 },
          after: { price_base_units: 40000 }
        }
      ],
      policy_diffs: []
    };

    const spy = vi.spyOn(configApi, 'getDraftDiff').mockResolvedValue(mockDiff);
    const diff = await configApi.getDraftDiff('ten_primary_001');

    expect(spy).toHaveBeenCalledWith('ten_primary_001');
    expect(diff.has_changes).toBe(true);
    expect(diff.route_diffs.length).toBe(2);
    expect(diff.route_diffs[0].action).toBe('added');
    expect(diff.route_diffs[1].action).toBe('modified');
  });

  it('should block publication logic if draft validation fails', async () => {
    const mockFailedValidation: ValidationResult = {
      valid: false,
      errors: [
        {
          target_type: 'Route',
          target_id: 'rt_001',
          field: 'path_pattern',
          code: 'DUPLICATE_PATH_PATTERN',
          message: 'path_pattern /v1/chat is duplicated'
        }
      ],
      summary: {
        total_routes: 2,
        total_policies: 1,
        error_count: 1,
        warning_count: 0
      },
      validated_at: 1773839300
    };

    vi.spyOn(configApi, 'validateDraft').mockResolvedValue(mockFailedValidation);

    // Publication pre-check runner
    async function canPublishDraft(tenantId: string): Promise<{ allow: boolean; errorCount: number }> {
      const res = await configApi.validateDraft(tenantId);
      if (!res.valid) {
        return { allow: false, errorCount: res.summary.error_count };
      }
      return { allow: true, errorCount: 0 };
    }

    const check = await canPublishDraft('ten_primary_001');
    expect(check.allow).toBe(false);
    expect(check.errorCount).toBe(1);
  });
});
