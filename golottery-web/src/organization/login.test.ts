import { describe, expect, it } from 'vitest'
import { ApiError } from '#/api'
import { orgLoginFailureMessage, toOrgLoginBody, validateOrgLogin } from '#/organization/login'

describe('organization login form', () => {
  it('normalizes the email but not the password', () => {
    expect(toOrgLoginBody({ email: '  Admin@Example.COM ', password: ' Pa55 ' })).toEqual({ email: 'admin@example.com', password: ' Pa55 ' })
  })

  it('requires an email-shaped address and a password', () => {
    expect(validateOrgLogin({ email: 'admin', password: '' })).toEqual({ email: '请输入邮箱', password: '请输入口令' })
    expect(validateOrgLogin({ email: ' a@b.cn ', password: 'x' })).toEqual({ email: undefined, password: undefined })
  })

  it('shows the API message for business errors', () => {
    expect(orgLoginFailureMessage(new ApiError(401, 'E_INVALID_CREDENTIALS', '账号或口令错误'))).toBe('账号或口令错误')
    expect(orgLoginFailureMessage(new TypeError('x'))).toBe('登录失败，请稍后重试')
    expect(orgLoginFailureMessage(null)).toBeNull()
  })
})
