import { getToken, setToken } from '#/api'

const PUBLIC_ID_KEY = 'gl.host.public_id'

/** Remembers which event the current host token belongs to. */
export function getHostPublicId(): string | null {
  return localStorage.getItem(PUBLIC_ID_KEY)
}

export function setHostSession(publicId: string, token: string): void {
  localStorage.setItem(PUBLIC_ID_KEY, publicId)
  setToken('host', token)
}

export function clearHostSession(): void {
  localStorage.removeItem(PUBLIC_ID_KEY)
  setToken('host', null)
}

export function hasHostSession(publicId: string): boolean {
  return Boolean(getToken('host') && getHostPublicId() === publicId)
}
