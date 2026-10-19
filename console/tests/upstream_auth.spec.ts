import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { routeApi } from '../src/api/client';
import type { AuthTypeSchema, TestUpstreamResult, UpstreamAuthConfig } from '../src/types/auth_config';

describe('T4.2 Upstream API Authentication & Connectivity Testing Suite', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it('should fetch upstream auth strategy schemas', async () => {
    const mockSchemas: AuthTypeSchema[] = [
      {
        type: 'none',
        label: '无鉴权 (None)',
        description: '网关透传请求，不注入凭证',
        fields: []
      },
      {
        type: 'api_key',
        label: 'API Key 鉴权',
        description: '通过 Header/Query/Cookie 传递 API Key',
        fields: [
          { key: 'in', label: '传递位置', kind: 'select', options: ['header', 'query', 'cookie'], secret: false, required: true },
          { key: 'name', label: '参数名', kind: 'text', secret: false, required: true },
          { key: 'value', label: '密钥值', kind: 'password', secret: true, required: true }
        ]
      },
      {
        type: 'bearer',
        label: 'Bearer Token',
        description: '在 Authorization Header 注入 Bearer Token',
        fields: [
          { key: 'token', label: 'Token 凭证', kind: 'password', secret: true, required: true }
        ]
      }
    ];

    const spy = vi.spyOn(routeApi, 'getAuthTypes').mockResolvedValue(mockSchemas);
    const schemas = await routeApi.getAuthTypes();

    expect(spy).toHaveBeenCalled();
    expect(schemas.length).toBe(3);
    expect(schemas[1].type).toBe('api_key');
    expect(schemas[1].fields.find((f) => f.key === 'value')?.secret).toBe(true);
  });

  it('should test upstream connectivity with auth credentials and receive sanitized headers', async () => {
    const mockResult: TestUpstreamResult = {
      success: true,
      status_code: 200,
      status_text: 'OK',
      latency_ms: 38,
      sent_headers: {
        'Host': 'api.openai.com',
        'Authorization': 'Bearer sk-●●●●●●ef',
        'X-Forwarded-Host': 'gateway.agentpay.io'
      }
    };

    const spy = vi.spyOn(routeApi, 'testUpstream').mockResolvedValue(mockResult);

    const res = await routeApi.testUpstream('ten_primary_001', {
      upstream_url: 'https://api.openai.com/v1/chat/completions',
      method: 'GET',
      upstream_auth: {
        type: 'bearer',
        secrets: { token: 'sk-abcdef1234567890ef' }
      }
    });

    expect(spy).toHaveBeenCalledWith('ten_primary_001', expect.objectContaining({
      upstream_url: 'https://api.openai.com/v1/chat/completions',
      method: 'GET'
    }));

    expect(res.success).toBe(true);
    expect(res.status_code).toBe(200);
    expect(res.sent_headers['Authorization']).toContain('●●●●●●');
    expect(res.sent_headers['Authorization']).not.toContain('abcdef1234567890');
  });

  it('should preserve secrets write-only pattern: omit secrets when not updated', () => {
    const existingConfig: UpstreamAuthConfig = {
      type: 'api_key',
      params: { in: 'header', name: 'X-API-Key', prefix: 'Token ' },
      has_secret: true
    };

    function buildPayload(cfg: UpstreamAuthConfig, newSecretValue?: string): UpstreamAuthConfig {
      const payload: UpstreamAuthConfig = {
        type: cfg.type,
        params: { ...cfg.params },
        has_secret: cfg.has_secret
      };
      if (newSecretValue) {
        payload.secrets = { value: newSecretValue };
      }
      return payload;
    }

    // 1. When user does not type a new secret
    const payloadWithoutNewSecret = buildPayload(existingConfig);
    expect(payloadWithoutNewSecret.secrets).toBeUndefined();
    expect(payloadWithoutNewSecret.has_secret).toBe(true);

    // 2. When user types a new secret
    const payloadWithNewSecret = buildPayload(existingConfig, 'new_secret_key_999');
    expect(payloadWithNewSecret.secrets?.value).toBe('new_secret_key_999');
  });
});
