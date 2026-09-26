import { ApiError } from '#/api'
import type { ChangePasswordRequest } from '#/api-gen/types.gen'

export type PasswordValues = {
  currentPassword: string
  newPassword: string
  confirmPassword: string
}

export const emptyPassword: PasswordValues = {
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
}

export function toPasswordBody(values: PasswordValues): ChangePasswordRequest {
  return {
    current_password: values.currentPassword,
    new_password: values.newPassword,
  }
}

export function validatePassword(values: PasswordValues): Partial<Record<keyof PasswordValues, string>> {
  const errors: Partial<Record<keyof PasswordValues, string>> = {}
  if (!values.currentPassword) {
    errors.currentPassword = '请输入当前口令'
  }
  if (!values.newPassword) {
    errors.newPassword = '请输入新口令'
  } else if (values.newPassword.length < 8) {
    errors.newPassword = '新口令至少 8 位'
  } else if (values.newPassword === values.currentPassword) {
    errors.newPassword = '新口令不能与当前口令相同'
  }
  if (!values.confirmPassword) {
    errors.confirmPassword = '请再次输入新口令'
  } else if (values.confirmPassword !== values.newPassword) {
    errors.confirmPassword = '两次输入不一致'
  }
  return errors
}

/** Maps API password errors onto the matching field; anything else is a form-level message. */
export function passwordErrors(error: unknown): { currentPassword?: string; newPassword?: string; form?: string } {
  if (!error) {
    return {}
  }
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'E_CURRENT_PASSWORD_WRONG':
        return { currentPassword: error.message }
      case 'E_PASSWORD_TOO_SHORT':
      case 'E_PASSWORD_UNCHANGED':
        return { newPassword: error.message }
      default:
        return { form: error.message }
    }
  }
  return { form: '操作失败，请稍后重试' }
}
