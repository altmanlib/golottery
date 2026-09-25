import { client } from '#/api-gen/client.gen'

export const TOKEN_KEY = 'gl.token'

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

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null): void {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token)
  } else {
    localStorage.removeItem(TOKEN_KEY)
  }
}

let unauthorizedHandler: (() => void) | null = null

/** Register the global 401 handler. The app wires this to the login route. */
export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler
}

export async function parseApiError(response: Response): Promise<ApiError> {
  let code = 'E_INTERNAL'
  let message = response.statusText || 'request failed'
  try {
    const body = (await response.json()) as { code?: string; message?: string }
    code = body?.code ?? code
    message = body?.message ?? message
  } catch {
    // Non-JSON bodies keep the fallback message.
  }
  return new ApiError(response.status, code, message)
}

client.setConfig({ baseUrl: '' })

client.interceptors.request.use((request) => {
  const token = getToken()
  if (token && !request.headers.has('Authorization')) {
    request.headers.set('Authorization', `Bearer ${token}`)
  }
  return request
})

client.interceptors.response.use((response) => {
  if (response.status === 401) {
    setToken(null)
    unauthorizedHandler?.()
  }
  return response
})

export { client }
