import { describe, expect, it } from 'vitest'
import { ApiError } from '#/api'
import type { GuestStatus } from '#/api-gen/types.gen'
import { ensureDeviceId } from '#/guest/device'
import { approvalBody, checkinBody, checkinWindow, errorMessage, GeolocationDenied, guestStep, suggestsHelp, validateBind } from '#/guest/guest'

const base: GuestStatus = { event: { name: '年会', status: 'ready', checkin_mode: 'geo' } }

describe('guest page', () => {
  it('walks bind → check-in → done', () => {
    expect(guestStep(base)).toBe('bind')
    expect(guestStep({ ...base, attendee: { name: '李雷', dept: '', checked_in: false } })).toBe('checkin')
    expect(guestStep({ ...base, attendee: { name: '李雷', dept: '', checked_in: true } })).toBe('done')
    expect(guestStep({ ...base, event: { ...base.event, status: 'closed' } })).toBe('closed')
  })

  it('sends browser coordinates as wgs84, and nothing in direct mode', () => {
    const fix = { lat: 39.9, lng: 116.4, accuracy: 25 }
    expect(checkinBody('geo', fix)).toEqual({ lat: 39.9, lng: 116.4, accuracy: 25, coord_type: 'wgs84' })
    expect(checkinBody('direct', fix)).toEqual({})
  })

  it('validates the binding form', () => {
    expect(validateBind({ name: ' ', phoneLast4: '12a4' })).toEqual({ name: '请输入姓名', phoneLast4: '请输入 4 位数字' })
    expect(validateBind({ name: '李雷', phoneLast4: '1234' })).toEqual({ name: undefined, phoneLast4: undefined })
  })

  it('offers help after errors staff can fix', () => {
    expect(suggestsHelp(new ApiError(400, 'E_OUT_OF_RANGE', '不在签到范围内'))).toBe(true)
    expect(suggestsHelp(new ApiError(409, 'E_WINDOW_CLOSED', '当前不在签到时间内'))).toBe(false)
    expect(errorMessage(new GeolocationDenied())).toContain('允许浏览器使用定位')
  })
})

describe('device id', () => {
  it('is created once and reused', () => {
    const store = new Map<string, string>()
    const storage = { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => void store.set(k, v) }
    const first = ensureDeviceId(storage, () => '11111111-2222-3333-4444-555555555555')
    expect(ensureDeviceId(storage, () => 'other-id-other-id-other')).toBe(first)
    store.set('gl.device', 'bad id!')
    expect(ensureDeviceId(storage, () => 'aaaaaaaaaaaaaaaaaaaa')).toBe('aaaaaaaaaaaaaaaaaaaa')
  })
})

describe('staff approvals', () => {
  it('builds the three request resolutions', () => {
    expect(approvalBody(null)).toEqual({})
    expect(approvalBody({ kind: 'link', attendeeId: 'a1' })).toEqual({ attendee_id: 'a1' })
    expect(approvalBody({ kind: 'create', name: ' 赵六 ', dept: ' 访客 ', phone: '7777' })).toEqual({ create: { name: '赵六', dept: '访客', phone: '7777' } })
  })
})

describe('check-in window', () => {
  it('shows the window in Asia/Shanghai, or nothing when unset', () => {
    expect(checkinWindow({ ...base.event, checkin_start: '2026-10-01T01:00:00Z', checkin_end: '2026-10-01T04:30:00Z' })).toBe(
      '签到时间 2026-10-01 09:00 至 2026-10-01 12:30',
    )
    expect(checkinWindow(base.event)).toBe('')
  })
})
