import { describe, expect, it } from 'vitest'
import { ApiError } from '#/api'
import type { Event } from '#/api-gen/types.gen'
import {
  attendeeErrorMessage,
  formatShanghai,
  fromShanghaiInput,
  guestEntryUrl,
  importFailure,
  parseTab,
  prizeErrorMessage,
  readinessSteps,
  readyBlocker,
  readyCostHint,
  settingsFromEvent,
  settingsPatch,
  toPrizeInput,
  toShanghaiInput,
  validateAttendee,
  validatePrize,
  validateSettings,
} from '#/organization/events'

const event: Event = {
  id: 'e1',
  public_id: 'V1StGXR8_Z5jdHi6B-myT',
  name: '年会',
  status: 'draft',
  checkin_mode: 'geo',
  center_lat: 39.908823,
  center_lng: 116.39747,
  radius_m: 400,
  checkin_start: '2026-10-01T01:00:00Z',
  checkin_end: '2026-10-01T04:00:00Z',
  allow_multi_win: false,
  max_attendees: 800,
  attendee_count: 1,
  prize_count: 0,
  credit_consumed: false,
  created_at: '2026-09-25T00:00:00Z',
}

describe('Asia/Shanghai times', () => {
  it('round-trips datetime-local values through +08:00', () => {
    expect(toShanghaiInput('2026-10-01T01:00:00Z')).toBe('2026-10-01T09:00')
    expect(toShanghaiInput('2026-09-30T17:30:00Z')).toBe('2026-10-01T01:30')
    expect(fromShanghaiInput('2026-10-01T09:00')).toBe('2026-10-01T09:00:00+08:00')
    expect(new Date(fromShanghaiInput('2026-10-01T09:00') as string).toISOString()).toBe('2026-10-01T01:00:00.000Z')
    expect(fromShanghaiInput('')).toBeNull()
    expect(toShanghaiInput(null)).toBe('')
    expect(formatShanghai('2026-10-01T01:00:00Z')).toBe('2026-10-01 09:00')
  })
})

describe('check-in settings', () => {
  it('sends nothing when nothing changed', () => {
    expect(settingsPatch(event, settingsFromEvent(event))).toEqual({})
  })

  it('sends only the changed fields, with times in +08:00', () => {
    const values = { ...settingsFromEvent(event), name: ' 新年会 ', checkinMode: 'direct' as const, checkinEnd: '2026-10-01T13:00', radiusM: 500 }
    expect(settingsPatch(event, values)).toEqual({
      name: '新年会',
      checkin_mode: 'direct',
      radius_m: 500,
      checkin_end: '2026-10-01T13:00:00+08:00',
    })
  })

  it('validates ranges and the window order', () => {
    const errors = validateSettings({ ...settingsFromEvent(event), name: ' ', centerLat: 91, radiusM: 50, checkinEnd: '2026-10-01T08:00' })
    expect(Object.keys(errors).sort()).toEqual(['centerLat', 'checkinEnd', 'name', 'radiusM'])
    expect(validateSettings(settingsFromEvent(event))).toEqual({})
  })
})

describe('ready button', () => {
  it('blocks a first ready without credits only', () => {
    expect(readyBlocker(event, 0)).toBe('剩余场次为 0，请联系运营开通')
    expect(readyBlocker(event, 2)).toBeNull()
    expect(readyBlocker({ ...event, credit_consumed: true }, 0)).toBeNull()
    expect(readyBlocker({ ...event, status: 'ready' }, 0)).toBeNull()
  })

  it('tells the admin what readiness costs', () => {
    expect(readyCostHint(event, 3)).toBe('首次就绪将消耗 1 场次，当前剩余 3 场')
    expect(readyCostHint({ ...event, credit_consumed: true }, 0)).toBe('本场已扣过场次，再次就绪不再扣减')
  })
})

describe('roster and prizes', () => {
  it('lists import rows from the error body', () => {
    const error = new ApiError(400, 'E_IMPORT_INVALID', '有 1 行需要修正，整份文件未导入', {
      code: 'E_IMPORT_INVALID',
      rows: [{ row: 3, reason: '姓名为空' }],
    })
    expect(importFailure(error)).toEqual({ message: '有 1 行需要修正，整份文件未导入', rows: [{ row: 3, reason: '姓名为空' }] })
    expect(importFailure(new ApiError(400, 'E_ROSTER_FULL', '名单不能超过人数上限 3 人', { rows: [] }))?.rows).toEqual([])
    expect(importFailure(null)).toBeNull()
  })

  it('maps conflicts to plain explanations', () => {
    expect(attendeeErrorMessage(new ApiError(409, 'E_CONFLICT', 'x'))).toBe('名单里已有同名且后四位相同的人员')
    expect(prizeErrorMessage(new ApiError(409, 'E_CONFLICT', 'x'))).toBe('该顺序已被其他奖项使用')
    expect(attendeeErrorMessage(new ApiError(400, 'E_ROSTER_FULL', '名单不能超过人数上限 3 人'))).toBe('名单不能超过人数上限 3 人')
  })

  it('validates roster rows and prizes', () => {
    expect(validateAttendee({ name: ' ', dept: '', phone: '12' })).toEqual({ name: '请填写姓名', phone: '至少填写手机号后四位' })
    expect(validateAttendee({ name: '李雷', dept: '', phone: '138-0000-1234' })).toEqual({ name: undefined, phone: undefined })
    expect(validatePrize({ name: '一等奖', gift: '', quota: 0, sortNo: '' })).toEqual({ name: undefined, quota: '名额是正整数', sortNo: '请填写整数顺序' })
    expect(toPrizeInput({ name: ' 一等奖 ', gift: ' 手机 ', quota: '2', sortNo: 1 })).toEqual({ name: '一等奖', gift: '手机', quota: 2, sort_no: 1 })
  })

  it('builds the guest entry url on the current host', () => {
    expect(guestEntryUrl('https://golottery.ioclub.cn', event.public_id)).toBe('https://golottery.ioclub.cn/#/m/V1StGXR8_Z5jdHi6B-myT')
  })
})

describe('detail overview', () => {
  it('marks the steps the server requires before ready', () => {
    const steps = readinessSteps({ ...event, attendee_count: 0, center_lat: null, center_lng: null, checkin_end: null })
    expect(steps.map((s) => [s.label, s.done, s.required])).toEqual([
      ['导入名单', false, true],
      ['设定签到时间', false, true],
      ['标注会场位置', false, true],
      ['设置奖项', false, false],
    ])
    expect(
      readinessSteps(event)
        .filter((s) => s.required)
        .every((s) => s.done),
    ).toBe(true)
  })

  it('needs no fence in direct mode', () => {
    const step = readinessSteps({ ...event, checkin_mode: 'direct', center_lat: null, center_lng: null })[2]
    expect(step).toMatchObject({ label: '选择签到方式', done: true })
  })

  it('falls back to the overview for unknown tabs', () => {
    expect(parseTab('roster')).toBe('roster')
    expect(parseTab('nope')).toBe('overview')
    expect(parseTab(null)).toBe('overview')
  })
})
