import { describe, expect, it } from 'vitest'
import { ApiError } from '#/api'
import { emptyPassword, passwordErrors, toPasswordBody, validatePassword } from '#/platform/password'

describe('change password form', () => {
  it('sends the passwords as typed', () => {
    expect(toPasswordBody({ currentPassword: ' old ', newPassword: ' newPass1 ', confirmPassword: ' newPass1 ' })).toEqual({
      current_password: ' old ',
      new_password: ' newPass1 ',
    })
  })

  it('requires both passwords, a minimum length, a change, and a matching confirmation', () => {
    expect(validatePassword(emptyPassword)).toEqual({
      currentPassword: '请输入当前口令',
      newPassword: '请输入新口令',
      confirmPassword: '请再次输入新口令',
    })
    expect(validatePassword({ currentPassword: 'old', newPassword: 'short', confirmPassword: 'short' }).newPassword).toBe('新口令至少 8 位')
    expect(validatePassword({ currentPassword: 'samePass1', newPassword: 'samePass1', confirmPassword: 'samePass1' }).newPassword).toBe(
      '新口令不能与当前口令相同',
    )
    expect(validatePassword({ currentPassword: 'oldPass1', newPassword: 'newPass12', confirmPassword: 'other' }).confirmPassword).toBe('两次输入不一致')
    expect(validatePassword({ currentPassword: 'oldPass1', newPassword: 'newPass12', confirmPassword: 'newPass12' })).toEqual({})
  })

  it('routes API codes to the matching field', () => {
    expect(passwordErrors(new ApiError(400, 'E_CURRENT_PASSWORD_WRONG', '当前口令不正确'))).toEqual({
      currentPassword: '当前口令不正确',
    })
    expect(passwordErrors(new ApiError(400, 'E_PASSWORD_TOO_SHORT', '密码至少 8 位'))).toEqual({ newPassword: '密码至少 8 位' })
    expect(passwordErrors(new ApiError(400, 'E_PASSWORD_UNCHANGED', '新口令不能与当前口令相同'))).toEqual({
      newPassword: '新口令不能与当前口令相同',
    })
    expect(passwordErrors(new ApiError(500, 'E_INTERNAL', '服务暂时不可用'))).toEqual({ form: '服务暂时不可用' })
    expect(passwordErrors(null)).toEqual({})
  })
})
