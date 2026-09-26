const DEVICE_KEY = 'gl.device'

type KeyValue = Pick<Storage, 'getItem' | 'setItem'>

/** A random id that stands for this browser in web mode; it survives reloads. */
export function ensureDeviceId(storage: KeyValue, random: () => string = () => crypto.randomUUID()): string {
  const existing = storage.getItem(DEVICE_KEY)
  if (existing && /^[A-Za-z0-9_-]{16,60}$/.test(existing)) {
    return existing
  }
  const id = random()
  storage.setItem(DEVICE_KEY, id)
  return id
}
