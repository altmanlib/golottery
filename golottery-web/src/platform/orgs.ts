import { ApiError } from '#/api'
import type { AdjustCreditsRequest, CreateOrgRequest, OrgStatus } from '#/api-gen/types.gen'

export const PAGE_SIZE = 40

/** Attendee tiers from the PRD; the operator may still type any positive number. */
export const ATTENDEE_TIERS = [
  { label: '体验', value: 100 },
  { label: '标准', value: 800 },
  { label: '加量', value: 2000 },
] as const

export const STATUS_LABELS: Record<OrgStatus, string> = {
  active: '启用',
  disabled: '停用',
}

/** Reads `?page=` as a 1-based page number; anything invalid is page 1. */
export function parsePage(raw: string | null): number {
  const page = Number(raw)
  return Number.isInteger(page) && page >= 1 ? page : 1
}

export function pageOffset(page: number): number {
  return (page - 1) * PAGE_SIZE
}

export function pageCount(total: number): number {
  return Math.max(1, Math.ceil(total / PAGE_SIZE))
}

export type CreateOrgValues = {
  name: string
  contact: string
  eventCredits: number | string
  maxAttendees: number | string
}

export const emptyCreateOrg: CreateOrgValues = { name: '', contact: '', eventCredits: 1, maxAttendees: 800 }

export function toCreateOrgBody(values: CreateOrgValues): CreateOrgRequest {
  return {
    name: values.name.trim(),
    contact: values.contact.trim(),
    event_credits: Number(values.eventCredits),
    max_attendees: Number(values.maxAttendees),
  }
}

export function validateCreateOrg(values: CreateOrgValues): Partial<Record<keyof CreateOrgValues, string>> {
  const credits = Number(values.eventCredits)
  const attendees = Number(values.maxAttendees)
  return {
    name: values.name.trim() ? undefined : '请填写组织名称',
    eventCredits: values.eventCredits !== '' && Number.isInteger(credits) && credits >= 0 ? undefined : '场次是不小于 0 的整数',
    maxAttendees: values.maxAttendees !== '' && Number.isInteger(attendees) && attendees > 0 ? undefined : '人数上限是正整数',
  }
}

export type AdjustCreditsValues = { delta: number | string; reason: string }

export const emptyAdjustCredits: AdjustCreditsValues = { delta: '', reason: '' }

export function toAdjustCreditsBody(values: AdjustCreditsValues): AdjustCreditsRequest {
  return { delta: Number(values.delta), reason: values.reason.trim() }
}

export function validateAdjustCredits(values: AdjustCreditsValues): Partial<Record<keyof AdjustCreditsValues, string>> {
  const delta = Number(values.delta)
  return {
    delta: values.delta !== '' && Number.isInteger(delta) && delta !== 0 ? undefined : '填写非零整数，负数表示扣减',
    reason: values.reason.trim() ? undefined : '请填写原因',
  }
}

/** A 409 on credits can only mean the balance would drop below zero. */
export function adjustCreditsErrorMessage(error: unknown): string | null {
  if (!error) {
    return null
  }
  if (error instanceof ApiError) {
    return error.code === 'E_CONFLICT' ? '扣减后剩余场次不能小于 0' : error.message
  }
  return '操作失败，请稍后重试'
}

/** Name errors belong on the name field; everything else is a form-level message. */
export function createOrgErrors(error: unknown): { name?: string; form?: string } {
  if (!error) {
    return {}
  }
  if (error instanceof ApiError) {
    return error.code === 'E_NAME_REQUIRED' ? { name: error.message } : { form: error.message }
  }
  return { form: '操作失败，请稍后重试' }
}
