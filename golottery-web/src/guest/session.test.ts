import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getToken, unwrap } from '#/api'
import { getGuestStatus } from '#/api-gen/sdk.gen'
import { asGuest, ensureSession } from '#/guest/session'

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

let logins = 0
let valid = new Set<string>()

function stubServer() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: Request) => {
      const path = new URL(input.url).pathname
      if (path === '/api/guest/session') {
        logins += 1
        const token = `t${logins}`
        valid = new Set([token])
        return json(200, { token, expires_at: '2026-10-01T00:00:00Z' })
      }
      const auth = input.headers.get('Authorization')?.replace('Bearer ', '') ?? ''
      if (!valid.has(auth)) return json(401, { code: 'E_UNAUTHORIZED', message: '请先登录' })
      return json(200, { event: { name: '年会', status: 'ready', checkin_mode: 'direct' } })
    }),
  )
}

describe('guest session', () => {
  beforeEach(() => {
    localStorage.clear()
    logins = 0
    valid = new Set()
    stubServer()
  })
  afterEach(() => vi.unstubAllGlobals())

  it('shares one login between concurrent callers', async () => {
    await Promise.all([ensureSession('pub1'), ensureSession('pub1'), ensureSession('pub1')])
    expect(logins).toBe(1)
    expect(getToken('guest')).toBe('t1')
  })

  it('signs in again for another event', async () => {
    await ensureSession('pub1')
    await ensureSession('pub2')
    expect(logins).toBe(2)
  })

  it('retries once with a fresh login after the token expired', async () => {
    await ensureSession('pub1')
    valid = new Set() // the server forgot the session
    const status = await asGuest('pub1', () => unwrap(getGuestStatus()))
    expect(status.event.name).toBe('年会')
    expect(logins).toBe(2)
    expect(getToken('guest')).toBe('t2')
  })
})
