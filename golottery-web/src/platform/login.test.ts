import { describe, expect, it } from 'vitest'
import { ApiError } from '#/api'
import { loginFailureMessage, toLoginBody, validateLogin } from '#/platform/login'

describe('platform login form', () => {
  it('trims the username but sends the password as typed', () => {
    expect(toLoginBody({ username: '  ops ', password: ' secret ' })).toEqual({ username: 'ops', password: ' secret ' })
  })

  it('requires both fields', () => {
    expect(validateLogin({ username: ' ', password: '' })).toEqual({ username: '请输入账号', password: '请输入口令' })
    expect(validateLogin({ username: 'ops', password: 'x' })).toEqual({ username: undefined, password: undefined })
  })

  it('shows the API message, or a generic line for other errors', () => {
    expect(loginFailureMessage(null)).toBeNull()
    expect(loginFailureMessage(new ApiError(429, 'E_TOO_MANY_ATTEMPTS', '尝试次数过多，请 15 分钟后再试'))).toBe('尝试次数过多，请 15 分钟后再试')
    expect(loginFailureMessage(new Error('boom'))).toBe('登录失败，请稍后重试')
  })
})
