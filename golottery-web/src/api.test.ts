import { afterEach, describe, expect, it } from 'vitest'

import { ApiError, apiErrorFrom, expiredScope, getToken, scopeOf, setToken, TOKEN_KEYS, unwrap } from './api'

afterEach(() => {
  localStorage.clear()
})

describe('apiErrorFrom', () => {
  it('uses {code, message} from the body', () => {
    const error = apiErrorFrom(401, { code: 'E_UNAUTHORIZED', message: '登录已失效，请重新登录' }, 'fallback')
    expect(error).toMatchObject({ name: 'ApiError', status: 401, code: 'E_UNAUTHORIZED', message: '登录已失效，请重新登录' })
  })

  it('falls back when the body is not an error object', () => {
    const error = apiErrorFrom(500, 'nope', 'oops')
    expect(error.code).toBe('E_INTERNAL')
    expect(error.message).toBe('oops')
  })
})

describe('unwrap', () => {
  it('returns data for a successful response', async () => {
    await expect(unwrap(Promise.resolve({ data: { ok: true }, response: new Response(null, { status: 200 }) }))).resolves.toEqual({ ok: true })
  })

  it('throws ApiError with the server message', async () => {
    const result = { error: { code: 'E_INVALID_CREDENTIALS', message: '账号或口令错误' }, response: new Response(null, { status: 401 }) }
    await expect(unwrap(Promise.resolve(result))).rejects.toMatchObject({ status: 401, code: 'E_INVALID_CREDENTIALS', message: '账号或口令错误' })
  })

  it('turns a rejected request into a network error', async () => {
    const error = await unwrap(Promise.reject(new TypeError('fetch failed'))).catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ status: 0, code: 'E_NETWORK' })
  })
})

describe('token scopes', () => {
  it('maps API prefixes to token scopes', () => {
    expect(scopeOf('/api/platform/me')).toBe('platform')
    expect(scopeOf('http://localhost:3000/api/organization/events?page=2')).toBe('console')
    expect(scopeOf('/api/host/abc/draws')).toBe('host')
    expect(scopeOf('/healthz')).toBeNull()
    expect(scopeOf('/api/platformx')).toBeNull()
  })

  it('drops the scope token on 401 except for the login call itself', () => {
    expect(expiredScope('/api/platform/me', 401)).toBe('platform')
    expect(expiredScope('/api/platform/login', 401)).toBeNull()
    expect(expiredScope('/api/platform/me', 400)).toBeNull()
    expect(expiredScope('/healthz', 401)).toBeNull()
  })

  it('stores each scope under its own key', () => {
    setToken('platform', 'p')
    setToken('console', 'c')
    expect(localStorage.getItem(TOKEN_KEYS.platform)).toBe('p')
    expect(getToken('console')).toBe('c')
    setToken('platform', null)
    expect(getToken('platform')).toBeNull()
    expect(getToken('console')).toBe('c')
  })
})
