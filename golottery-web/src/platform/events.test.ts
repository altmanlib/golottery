import { describe, expect, it } from 'vitest'
import { ApiError } from '#/api'
import { eventMaxAttendeesErrorMessage } from '#/platform/events'

describe('event max attendees', () => {
  it('explains a conflict as a roster floor', () => {
    expect(eventMaxAttendeesErrorMessage(new ApiError(409, 'E_CONFLICT', '操作与当前状态冲突'))).toBe('人数上限不能低于当前名单人数')
    expect(eventMaxAttendeesErrorMessage(new ApiError(400, 'E_BAD_REQUEST', '请求格式不正确'))).toBe('请求格式不正确')
    expect(eventMaxAttendeesErrorMessage(undefined)).toBeNull()
  })
})
