import { ApiError, getToken, setToken, unwrap } from '#/api'
import { guestLogin } from '#/api-gen/sdk.gen'
import { ensureDeviceId } from '#/guest/device'

/** The event the stored guest token belongs to; a token never crosses events. */
const EVENT_KEY = 'gl.guest.event'

let pending: { publicId: string; login: Promise<void> } | null = null

/** Signs in once per event; concurrent callers share one login so they don't rotate each other's token. */
export function ensureSession(publicId: string): Promise<void> {
  if (getToken('guest') && localStorage.getItem(EVENT_KEY) === publicId) {
    return Promise.resolve()
  }
  if (pending?.publicId === publicId) {
    return pending.login
  }
  const login = (async () => {
    const session = await unwrap(guestLogin({ body: { public_id: publicId, device_id: ensureDeviceId(localStorage) } }))
    setToken('guest', session.token)
    localStorage.setItem(EVENT_KEY, publicId)
  })().finally(() => {
    pending = null
  })
  pending = { publicId, login }
  return login
}

/** Runs a guest call, signing in first and once more if the token expired meanwhile. */
export async function asGuest<T>(publicId: string, call: () => Promise<T>): Promise<T> {
  await ensureSession(publicId)
  try {
    return await call()
  } catch (error) {
    if (!(error instanceof ApiError && error.status === 401)) {
      throw error
    }
    // The API client already dropped the rejected token.
    await ensureSession(publicId)
    return call()
  }
}
