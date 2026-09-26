import { ApiError } from '#/api'
import type { CheckinMode, Event, EventStatus, PrizeInput, UpdateEventRequest } from '#/api-gen/types.gen'

export const PAGE_SIZE = 40

export const STATUS_LABELS: Record<EventStatus, string> = {
  draft: '草稿',
  ready: '已就绪',
  closed: '已结束',
}

export const MODE_LABELS: Record<CheckinMode, string> = {
  geo: '定位签到',
  direct: '直接签到',
}

export const DIRECT_MODE_WARNING = '宾客不在现场也能签到，奖池可能包含缺席者，需依靠现场缺席重抽'

const SHANGHAI_OFFSET_MS = 8 * 60 * 60 * 1000

/** Shows an instant as the `datetime-local` value it has in Asia/Shanghai, whatever the browser zone. */
export function toShanghaiInput(iso: string | null | undefined): string {
  if (!iso) {
    return ''
  }
  return new Date(new Date(iso).getTime() + SHANGHAI_OFFSET_MS).toISOString().slice(0, 16)
}

/** Reads a `datetime-local` value as Asia/Shanghai time. */
export function fromShanghaiInput(value: string): string | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) {
    return null
  }
  return `${value}:00+08:00`
}

/** Formats an instant in Asia/Shanghai, e.g. 2026-10-01 09:00. */
export function formatShanghai(iso: string | null | undefined): string {
  return iso ? toShanghaiInput(iso).replace('T', ' ') : '—'
}

export type SettingsValues = {
  name: string
  checkinMode: CheckinMode
  centerLat: number | string
  centerLng: number | string
  radiusM: number | string
  checkinStart: string
  checkinEnd: string
  allowMultiWin: boolean
}

export function settingsFromEvent(event: Event): SettingsValues {
  return {
    name: event.name,
    checkinMode: event.checkin_mode,
    centerLat: event.center_lat ?? '',
    centerLng: event.center_lng ?? '',
    radiusM: event.radius_m,
    checkinStart: toShanghaiInput(event.checkin_start),
    checkinEnd: toShanghaiInput(event.checkin_end),
    allowMultiWin: event.allow_multi_win,
  }
}

export function validateSettings(values: SettingsValues): Partial<Record<keyof SettingsValues, string>> {
  const errors: Partial<Record<keyof SettingsValues, string>> = {}
  if (!values.name.trim()) {
    errors.name = '请填写活动名称'
  }
  const lat = values.centerLat === '' ? null : Number(values.centerLat)
  const lng = values.centerLng === '' ? null : Number(values.centerLng)
  if (lat !== null && !(lat >= -90 && lat <= 90)) {
    errors.centerLat = '纬度在 -90 到 90 之间'
  }
  if (lng !== null && !(lng >= -180 && lng <= 180)) {
    errors.centerLng = '经度在 -180 到 180 之间'
  }
  const radius = Number(values.radiusM)
  if (!Number.isInteger(radius) || radius < 100 || radius > 1000) {
    errors.radiusM = '半径为 100～1000 米'
  }
  const start = values.checkinStart ? fromShanghaiInput(values.checkinStart) : null
  const end = values.checkinEnd ? fromShanghaiInput(values.checkinEnd) : null
  if (start && end && new Date(end) <= new Date(start)) {
    errors.checkinEnd = '结束时间要晚于开始时间'
  }
  return errors
}

/** Only the fields that changed are sent; an emptied optional field is left as it was on the server. */
export function settingsPatch(event: Event, values: SettingsValues): UpdateEventRequest {
  const patch: UpdateEventRequest = {}
  const name = values.name.trim()
  if (name !== event.name) patch.name = name
  if (values.checkinMode !== event.checkin_mode) patch.checkin_mode = values.checkinMode
  if (values.centerLat !== '' && Number(values.centerLat) !== event.center_lat) patch.center_lat = Number(values.centerLat)
  if (values.centerLng !== '' && Number(values.centerLng) !== event.center_lng) patch.center_lng = Number(values.centerLng)
  if (Number(values.radiusM) !== event.radius_m) patch.radius_m = Number(values.radiusM)
  const start = fromShanghaiInput(values.checkinStart)
  if (start && (!event.checkin_start || new Date(start).getTime() !== new Date(event.checkin_start).getTime())) patch.checkin_start = start
  const end = fromShanghaiInput(values.checkinEnd)
  if (end && (!event.checkin_end || new Date(end).getTime() !== new Date(event.checkin_end).getTime())) patch.checkin_end = end
  if (values.allowMultiWin !== event.allow_multi_win) patch.allow_multi_win = values.allowMultiWin
  return patch
}

/** Why the event cannot be made ready from the page, or null. The server still checks everything. */
export function readyBlocker(event: Event, credits: number | undefined): string | null {
  if (event.status !== 'draft') {
    return null
  }
  if (!event.credit_consumed && credits === 0) {
    return '剩余场次为 0，请联系运营开通'
  }
  return null
}

export function readyCostHint(event: Event, credits: number | undefined): string {
  if (event.credit_consumed) {
    return '本场已扣过场次，再次就绪不再扣减'
  }
  return credits === undefined ? '首次就绪将消耗 1 场次' : `首次就绪将消耗 1 场次，当前剩余 ${credits} 场`
}

export type ImportRow = { row: number; reason: string }

/** Row errors from a rejected import; other failures come back with no rows. */
export function importFailure(error: unknown): { message: string; rows: ImportRow[] } | null {
  if (!error) {
    return null
  }
  if (!(error instanceof ApiError)) {
    return { message: '导入失败，请稍后重试', rows: [] }
  }
  const body = error.body as { rows?: unknown } | undefined
  const rows = Array.isArray(body?.rows) ? (body.rows as ImportRow[]) : []
  return { message: error.message, rows }
}

export type AttendeeValues = { name: string; dept: string; phone: string }

export const emptyAttendee: AttendeeValues = { name: '', dept: '', phone: '' }

export function validateAttendee(values: AttendeeValues): Partial<Record<keyof AttendeeValues, string>> {
  return {
    name: values.name.trim() ? undefined : '请填写姓名',
    phone: values.phone.replace(/\D/g, '').length >= 4 ? undefined : '至少填写手机号后四位',
  }
}

export type PrizeValues = { name: string; gift: string; quota: number | string; sortNo: number | string }

export function toPrizeInput(values: PrizeValues): PrizeInput {
  return { name: values.name.trim(), gift: values.gift.trim(), quota: Number(values.quota), sort_no: Number(values.sortNo) }
}

export function validatePrize(values: PrizeValues): Partial<Record<keyof PrizeValues, string>> {
  const quota = Number(values.quota)
  const sortNo = Number(values.sortNo)
  return {
    name: values.name.trim() ? undefined : '请填写奖项名称',
    quota: values.quota !== '' && Number.isInteger(quota) && quota > 0 ? undefined : '名额是正整数',
    sortNo: values.sortNo !== '' && Number.isInteger(sortNo) ? undefined : '请填写整数顺序',
  }
}

/** A 409 on prizes means the order number is already used in this event. */
export function prizeErrorMessage(error: unknown): string | null {
  if (!error) {
    return null
  }
  if (error instanceof ApiError) {
    return error.code === 'E_CONFLICT' ? '该顺序已被其他奖项使用' : error.message
  }
  return '操作失败，请稍后重试'
}

/** A 409 on roster rows means the same name and last four digits already exist. */
export function attendeeErrorMessage(error: unknown): string | null {
  if (!error) {
    return null
  }
  if (error instanceof ApiError) {
    return error.code === 'E_CONFLICT' ? '名单里已有同名且后四位相同的人员' : error.message
  }
  return '操作失败，请稍后重试'
}

/** The guest web entry for an event, on the current host. */
export function guestEntryUrl(origin: string, publicId: string): string {
  return `${origin}/#/m/${publicId}`
}

export type DetailTab = 'overview' | 'settings' | 'roster' | 'prizes' | 'onsite' | 'data'

export const DETAIL_TABS: { value: DetailTab; label: string }[] = [
  { value: 'overview', label: '概览' },
  { value: 'settings', label: '签到设置' },
  { value: 'roster', label: '名单' },
  { value: 'prizes', label: '奖项' },
  { value: 'onsite', label: '现场与大屏' },
  { value: 'data', label: '现场数据' },
]

export function parseTab(value: string | null): DetailTab {
  return DETAIL_TABS.some((t) => t.value === value) ? (value as DetailTab) : 'overview'
}

export type ReadinessStep = { label: string; done: boolean; required: boolean; detail: string; tab: DetailTab }

/** The overview checklist; required steps mirror the server's check before an event can go ready. */
export function readinessSteps(event: Event): ReadinessStep[] {
  const geo = event.checkin_mode === 'geo'
  const hasWindow = Boolean(event.checkin_start && event.checkin_end)
  return [
    {
      label: '导入名单',
      done: event.attendee_count > 0,
      required: true,
      detail: event.attendee_count > 0 ? `${event.attendee_count} 人，上限 ${event.max_attendees}` : `上限 ${event.max_attendees} 人`,
      tab: 'roster',
    },
    {
      label: '设定签到时间',
      done: hasWindow,
      required: true,
      detail: hasWindow ? `${formatShanghai(event.checkin_start)} 至 ${formatShanghai(event.checkin_end)}` : '北京时间，窗口外不能签到',
      tab: 'settings',
    },
    {
      label: geo ? '标注会场位置' : '选择签到方式',
      done: !geo || (event.center_lat != null && event.center_lng != null),
      required: true,
      detail: geo ? `定位签到，半径 ${event.radius_m} 米` : '直接签到，不校验位置',
      tab: 'settings',
    },
    {
      label: '设置奖项',
      done: event.prize_count > 0,
      required: false,
      detail: event.prize_count > 0 ? `${event.prize_count} 个奖项` : '抽奖前补上即可',
      tab: 'prizes',
    },
  ]
}
