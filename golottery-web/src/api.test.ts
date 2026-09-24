import { afterEach, describe, expect, it, vi } from 'vitest';

import { type ApiError, parseApiError, setToken, TOKEN_KEY } from './api';

afterEach(() => {
  setToken(null);
  vi.unstubAllGlobals();
});

describe('ApiError parsing', () => {
  it('parses {code, message}', async () => {
    const response = new Response(JSON.stringify({ code: 'E_UNAUTHORIZED', message: '登录已失效，请重新登录' }), {
      status: 401,
      headers: { 'Content-Type': 'application/json' },
    });
    const error = await parseApiError(response);
    expect(error).toMatchObject({
      name: 'ApiError',
      status: 401,
      code: 'E_UNAUTHORIZED',
      message: '登录已失效，请重新登录',
    } satisfies Partial<ApiError>);
  });

  it('falls back when the body is not JSON', async () => {
    const response = new Response('nope', { status: 500, statusText: 'oops' });
    const error = await parseApiError(response);
    expect(error.code).toBe('E_INTERNAL');
    expect(error.message).toBe('oops');
  });
});

describe('token storage', () => {
  it('writes and clears gl.token', () => {
    setToken('abc');
    expect(localStorage.getItem(TOKEN_KEY)).toBe('abc');
    setToken(null);
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull();
  });
});
