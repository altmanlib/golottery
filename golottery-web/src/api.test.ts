import { afterEach, describe, expect, it } from 'vitest'

import { ApiError, apiErrorFrom, expiredScope, getToken, loginRouteFor, redirectAfterExpiry, scopeOf, setToken, TOKEN_KEYS, unwrap } from './api'

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
    expect(scopeOf('/api/guest/checkin')).toBe('guest')
    expect(scopeOf('/healthz')).toBeNull()
    expect(scopeOf('/api/platformx')).toBeNull()
  })

  it('drops the scope token on 401 except for the login calls themselves', () => {
    expect(expiredScope('/api/platform/me', 401)).toBe('platform')
    expect(expiredScope('/api/organization/me', 401)).toBe('console')
    expect(expiredScope('/api/platform/login', 401)).toBeNull()
    expect(expiredScope('/api/organization/login', 401)).toBeNull()
    expect(expiredScope('/api/platform/me', 400)).toBeNull()
    expect(expiredScope('/healthz', 401)).toBeNull()
  })

  it('sends each rejected scope back to its own login page', () => {
    expect(loginRouteFor('platform')).toBe('/platform/login')
    expect(loginRouteFor('console')).toBe('/organization/login')
    expect(loginRouteFor('host')).toBeNull()
    expect(loginRouteFor('guest')).toBeNull()
    expect(redirectAfterExpiry('guest', '/m/abc')).toBeNull()
    expect(expiredScope('/api/guest/checkin', 401)).toBe('guest')
    expect(expiredScope('/api/guest/session', 401)).toBeNull()
  })

  it('redirects only when the user is still inside the expired scope', () => {
    expect(redirectAfterExpiry('console', '/organization')).toBe('/organization/login')
    expect(redirectAfterExpiry('console', '/organization/events/1')).toBe('/organization/login')
    expect(redirectAfterExpiry('console', '/organization/login')).toBeNull()
    expect(redirectAfterExpiry('console', '/platform/orgs')).toBeNull()
    expect(redirectAfterExpiry('platform', '/platform/orgs')).toBe('/platform/login')
    expect(redirectAfterExpiry('platform', '/platformx')).toBeNull()
    expect(redirectAfterExpiry('host', '/host/abc')).toBeNull()
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
