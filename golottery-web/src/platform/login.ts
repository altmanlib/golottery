import { ApiError } from '#/api'
import type { PlatformLoginRequest } from '#/api-gen/types.gen'

export type LoginValues = { username: string; password: string }

export const emptyLogin: LoginValues = { username: '', password: '' }

/** The username is sent without surrounding spaces; the password is sent as typed. */
export function toLoginBody(values: LoginValues): PlatformLoginRequest {
  return { username: values.username.trim(), password: values.password }
}

export function validateLogin(values: LoginValues): Partial<Record<keyof LoginValues, string>> {
  return {
    username: values.username.trim() ? undefined : '请输入账号',
    password: values.password ? undefined : '请输入口令',
  }
}

/** Shows the API message for business errors and a generic line for anything else. */
export function loginFailureMessage(error: unknown): string | null {
  if (!error) {
    return null
  }
  return error instanceof ApiError ? error.message : '登录失败，请稍后重试'
}
