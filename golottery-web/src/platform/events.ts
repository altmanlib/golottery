import { ApiError } from '#/api'

/** A 409 on an event limit means the new value is below the current roster. */
export function eventMaxAttendeesErrorMessage(error: unknown): string | null {
  if (!error) {
    return null
  }
  if (error instanceof ApiError) {
    return error.code === 'E_CONFLICT' ? '人数上限不能低于当前名单人数' : error.message
  }
  return '操作失败，请稍后重试'
}
