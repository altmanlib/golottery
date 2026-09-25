import { describe, expect, it } from 'vitest'
import { ApiError } from '#/api'
import {
  adjustCreditsErrorMessage,
  createOrgErrors,
  pageCount,
  pageOffset,
  parsePage,
  toAdjustCreditsBody,
  toCreateOrgBody,
  validateAdjustCredits,
  validateCreateOrg,
} from '#/platform/orgs'

describe('paging', () => {
  it('reads the page from the URL and falls back to 1', () => {
    expect(parsePage('3')).toBe(3)
    for (const raw of [null, '', '0', '-2', '1.5', 'abc']) {
      expect(parsePage(raw)).toBe(1)
    }
  })

  it('turns pages into offsets of 40', () => {
    expect(pageOffset(1)).toBe(0)
    expect(pageOffset(3)).toBe(80)
    expect(pageCount(0)).toBe(1)
    expect(pageCount(40)).toBe(1)
    expect(pageCount(41)).toBe(2)
  })
})

describe('create organization form', () => {
  it('submits trimmed text and numbers', () => {
    expect(toCreateOrgBody({ name: ' Acme ', contact: ' 李 ', eventCredits: '3', maxAttendees: 800 })).toEqual({
      name: 'Acme',
      contact: '李',
      event_credits: 3,
      max_attendees: 800,
    })
  })

  it('validates name, credits and attendee limit', () => {
    expect(validateCreateOrg({ name: 'A', contact: '', eventCredits: 0, maxAttendees: 100 })).toEqual({
      name: undefined,
      eventCredits: undefined,
      maxAttendees: undefined,
    })
    const errors = validateCreateOrg({ name: ' ', contact: '', eventCredits: -1, maxAttendees: 0 })
    expect(errors.name).toBeTruthy()
    expect(errors.eventCredits).toBeTruthy()
    expect(errors.maxAttendees).toBeTruthy()
    expect(validateCreateOrg({ name: 'A', contact: '', eventCredits: '', maxAttendees: 1.5 }).eventCredits).toBeTruthy()
  })

  it('routes E_NAME_REQUIRED to the name field', () => {
    expect(createOrgErrors(new ApiError(400, 'E_NAME_REQUIRED', '请填写名称'))).toEqual({ name: '请填写名称' })
    expect(createOrgErrors(new ApiError(400, 'E_BAD_REQUEST', '请求格式不正确'))).toEqual({ form: '请求格式不正确' })
    expect(createOrgErrors(null)).toEqual({})
  })
})

describe('adjust credits form', () => {
  it('submits a signed delta and trimmed reason', () => {
    expect(toAdjustCreditsBody({ delta: '-2', reason: ' 退款 ' })).toEqual({ delta: -2, reason: '退款' })
  })

  it('rejects zero, fractions and empty reasons', () => {
    expect(validateAdjustCredits({ delta: 0, reason: 'x' }).delta).toBeTruthy()
    expect(validateAdjustCredits({ delta: 1.5, reason: 'x' }).delta).toBeTruthy()
    expect(validateAdjustCredits({ delta: '', reason: 'x' }).delta).toBeTruthy()
    expect(validateAdjustCredits({ delta: -1, reason: ' ' }).reason).toBeTruthy()
    expect(validateAdjustCredits({ delta: -1, reason: '退款' })).toEqual({ delta: undefined, reason: undefined })
  })

  it('explains a conflict as an insufficient balance', () => {
    expect(adjustCreditsErrorMessage(new ApiError(409, 'E_CONFLICT', '操作与当前状态冲突'))).toBe('扣减后剩余场次不能小于 0')
    expect(adjustCreditsErrorMessage(new ApiError(404, 'E_NOT_FOUND', '内容不存在或无权访问'))).toBe('内容不存在或无权访问')
    expect(adjustCreditsErrorMessage(undefined)).toBeNull()
  })
})
