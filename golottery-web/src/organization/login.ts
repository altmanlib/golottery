import { ApiError } from '#/api'
import type { OrganizationLoginRequest } from '#/api-gen/types.gen'

export type OrgLoginValues = { email: string; password: string }

export const emptyOrgLogin: OrgLoginValues = { email: '', password: '' }

/** Emails are stored lowercased and trimmed; the password is sent as typed. */
export function toOrgLoginBody(values: OrgLoginValues): OrganizationLoginRequest {
  return { email: values.email.trim().toLowerCase(), password: values.password }
}

export function validateOrgLogin(values: OrgLoginValues): Partial<Record<keyof OrgLoginValues, string>> {
  return {
    email: /^[^\s@]+@[^\s@]+$/.test(values.email.trim()) ? undefined : '请输入邮箱',
    password: values.password ? undefined : '请输入口令',
  }
}

export function orgLoginFailureMessage(error: unknown): string | null {
  if (!error) {
    return null
  }
  return error instanceof ApiError ? error.message : '登录失败，请稍后重试'
}
