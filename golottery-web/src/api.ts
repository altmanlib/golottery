import { client } from '#/api-gen/client.gen'

/** Token scopes follow the API path prefix; the three token types are not interchangeable. */
export type TokenScope = 'platform' | 'console' | 'host'

export const TOKEN_KEYS: Record<TokenScope, string> = {
  platform: 'gl.token.platform',
  console: 'gl.token.console',
  host: 'gl.token.host',
}

const SCOPE_PREFIXES: [string, TokenScope][] = [
  ['/api/platform/', 'platform'],
  ['/api/organization/', 'console'],
  ['/api/host/', 'host'],
]

/** A 401 from these paths means wrong credentials, not an expired session. */
const LOGIN_PATHS = new Set(['/api/platform/login'])

const NETWORK_ERROR_MESSAGE = '无法连接服务器，请稍后重试'

export class ApiError extends Error {
  code: string
  status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

function pathOf(url: string): string {
  return new URL(url, 'http://localhost').pathname
}

export function scopeOf(url: string): TokenScope | null {
  const path = pathOf(url)
  return SCOPE_PREFIXES.find(([prefix]) => path.startsWith(prefix))?.[1] ?? null
}

/** Returns the scope whose token must be dropped after this response, if any. */
export function expiredScope(url: string, status: number): TokenScope | null {
  if (status !== 401 || LOGIN_PATHS.has(pathOf(url))) {
    return null
  }
  return scopeOf(url)
}

export function getToken(scope: TokenScope): string | null {
  return localStorage.getItem(TOKEN_KEYS[scope])
}

export function setToken(scope: TokenScope, token: string | null): void {
  if (token) {
    localStorage.setItem(TOKEN_KEYS[scope], token)
  } else {
    localStorage.removeItem(TOKEN_KEYS[scope])
  }
}

let unauthorizedHandler: ((scope: TokenScope) => void) | null = null

/** Register the global 401 handler. The app wires this to each scope's entry route. */
export function setUnauthorizedHandler(handler: ((scope: TokenScope) => void) | null): void {
  unauthorizedHandler = handler
}

/** Builds an ApiError from a `{code, message}` body; other bodies keep the fallback text. */
export function apiErrorFrom(status: number, body: unknown, fallback: string): ApiError {
  const { code, message } = (typeof body === 'object' && body !== null ? body : {}) as { code?: unknown; message?: unknown }
  return new ApiError(status, typeof code === 'string' ? code : 'E_INTERNAL', typeof message === 'string' ? message : fallback)
}

type SdkResult<T> = { data?: T; error?: unknown; response?: Response }

/** Resolves an SDK call to its data, or throws ApiError. */
export async function unwrap<T>(pending: Promise<SdkResult<T>>): Promise<T> {
  let result: SdkResult<T>
  try {
    result = await pending
  } catch {
    throw new ApiError(0, 'E_NETWORK', NETWORK_ERROR_MESSAGE)
  }
  const { data, error, response } = result
  if (error !== undefined || !response?.ok) {
    throw apiErrorFrom(response?.status ?? 0, error, response ? response.statusText || '系统出错了，请稍后重试' : NETWORK_ERROR_MESSAGE)
  }
  return data as T
}

client.setConfig({ baseUrl: '' })

client.interceptors.request.use((request) => {
  const scope = scopeOf(request.url)
  const token = scope ? getToken(scope) : null
  if (token && !request.headers.has('Authorization')) {
    request.headers.set('Authorization', `Bearer ${token}`)
  }
  return request
})

client.interceptors.response.use((response, request) => {
  const scope = expiredScope(request.url, response.status)
  if (scope) {
    setToken(scope, null)
    unauthorizedHandler?.(scope)
  }
  return response
})

export { client }
