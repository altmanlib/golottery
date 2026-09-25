import { ApiError } from '#/api'
import type { CreateOrgUserRequest } from '#/api-gen/types.gen'

export type CreateAdminValues = { name: string; email: string }

export const emptyCreateAdmin: CreateAdminValues = { name: '', email: '' }

export function toCreateAdminBody(values: CreateAdminValues): CreateOrgUserRequest {
  return { name: values.name.trim(), email: values.email.trim().toLowerCase() }
}

export function validateCreateAdmin(values: CreateAdminValues): Partial<Record<keyof CreateAdminValues, string>> {
  return {
    name: values.name.trim() ? undefined : '请填写姓名',
    email: /^[^\s@]+@[^\s@]+$/.test(values.email.trim()) ? undefined : '请填写有效邮箱',
  }
}

/** A conflict means the email already belongs to an admin, possibly of another organization. */
export function createAdminErrors(error: unknown): { email?: string; form?: string } {
  if (!error) {
    return {}
  }
  if (error instanceof ApiError) {
    return error.code === 'E_CONFLICT' ? { email: '该邮箱已被使用' } : { form: error.message }
  }
  return { form: '操作失败，请稍后重试' }
}

/** A one-time password on screen. It exists only in this state, never in the query cache. */
export type Reveal = { title: string; email: string; password: string }

export type RevealAction = { type: 'show'; reveal: Reveal } | { type: 'dismiss' }

export function revealReducer(_state: Reveal | null, action: RevealAction): Reveal | null {
  return action.type === 'show' ? action.reveal : null
}
