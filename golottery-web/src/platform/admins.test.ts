import { describe, expect, it } from 'vitest'
import { ApiError } from '#/api'
import { createAdminErrors, revealReducer, toCreateAdminBody, validateCreateAdmin } from '#/platform/admins'

describe('create admin form', () => {
  it('submits a trimmed name and a normalized email', () => {
    expect(toCreateAdminBody({ name: ' 李雷 ', email: ' Li@Example.COM ' })).toEqual({ name: '李雷', email: 'li@example.com' })
  })

  it('requires a name and an email', () => {
    expect(validateCreateAdmin({ name: ' ', email: 'x' })).toEqual({ name: '请填写姓名', email: '请填写有效邮箱' })
    expect(validateCreateAdmin({ name: '李雷', email: 'li@example.com' })).toEqual({ name: undefined, email: undefined })
  })

  it('puts a duplicate email on the email field', () => {
    expect(createAdminErrors(new ApiError(409, 'E_CONFLICT', '操作与当前状态冲突'))).toEqual({ email: '该邮箱已被使用' })
    expect(createAdminErrors(new ApiError(404, 'E_NOT_FOUND', '内容不存在或无权访问'))).toEqual({ form: '内容不存在或无权访问' })
    expect(createAdminErrors(null)).toEqual({})
  })
})

describe('one-time password', () => {
  const reveal = { title: '管理员已创建', email: 'li@example.com', password: 'Abc234xyzQ' }

  it('is shown once and gone after dismissing', () => {
    let state = revealReducer(null, { type: 'show', reveal })
    expect(state?.password).toBe('Abc234xyzQ')
    state = revealReducer(state, { type: 'dismiss' })
    expect(state).toBeNull()
  })

  it('replaces an earlier password instead of keeping both', () => {
    const next = revealReducer(reveal, { type: 'show', reveal: { ...reveal, password: 'Zzz234xyzQ' } })
    expect(next?.password).toBe('Zzz234xyzQ')
  })
})
