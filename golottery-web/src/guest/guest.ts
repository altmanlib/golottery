import { ApiError } from '#/api'
import type { CheckinMethod, GuestStatus } from '#/api-gen/types.gen'
import { formatShanghai } from '#/organization/events'

export const METHOD_LABELS: Record<CheckinMethod, string> = {
  geo: '定位签到',
  direct: '直接签到',
  manual: '工作人员确认',
  proxy: '工作人员代签',
}

export type GuestStep = 'bind' | 'checkin' | 'done' | 'closed'

/** Which card the guest page shows. */
export function guestStep(status: GuestStatus): GuestStep {
  if (status.attendee?.checked_in) return 'done'
  if (status.event.status === 'closed') return 'closed'
  return status.attendee ? 'checkin' : 'bind'
}

export type BindValues = { name: string; phoneLast4: string }

export function validateBind(values: BindValues): Partial<Record<keyof BindValues, string>> {
  return {
    name: values.name.trim() ? undefined : '请输入姓名',
    phoneLast4: /^\d{4}$/.test(values.phoneLast4) ? undefined : '请输入 4 位数字',
  }
}

/** Codes after which the guest should be offered on-site help. */
const HELP_CODES = new Set(['E_ATTENDEE_NOT_MATCHED', 'E_ATTENDEE_TAKEN', 'E_TOO_MANY_ATTEMPTS', 'E_LOW_ACCURACY', 'E_OUT_OF_RANGE'])

export function suggestsHelp(error: unknown): boolean {
  return error instanceof ApiError && HELP_CODES.has(error.code)
}

export function errorMessage(error: unknown): string | null {
  if (!error) return null
  if (error instanceof ApiError) return error.message
  if (error instanceof GeolocationDenied) return '无法获取位置。请允许浏览器使用定位，或联系现场工作人员'
  return '网络异常，请稍后重试'
}

export class GeolocationDenied extends Error {}

export type Fix = { lat: number; lng: number; accuracy: number }

/** Browsers report WGS-84; the server converts it to the fence's GCJ-02. */
export function checkinBody(mode: GuestStatus['event']['checkin_mode'], fix: Fix | null) {
  if (mode === 'direct' || !fix) return {}
  return { lat: fix.lat, lng: fix.lng, accuracy: fix.accuracy, coord_type: 'wgs84' as const }
}

export function currentFix(): Promise<Fix> {
  return new Promise((resolve, reject) => {
    if (!('geolocation' in navigator)) {
      reject(new GeolocationDenied())
      return
    }
    navigator.geolocation.getCurrentPosition(
      (p) => resolve({ lat: p.coords.latitude, lng: p.coords.longitude, accuracy: p.coords.accuracy }),
      () => reject(new GeolocationDenied()),
      { enableHighAccuracy: true, timeout: 15000, maximumAge: 0 },
    )
  })
}

/** What a staff member does with a request from someone not yet bound. */
export type Resolution = { kind: 'link'; attendeeId: string } | { kind: 'create'; name: string; dept: string; phone: string }

export function approvalBody(resolution: Resolution | null) {
  if (!resolution) return {}
  if (resolution.kind === 'link') return { attendee_id: resolution.attendeeId }
  return { create: { name: resolution.name.trim(), dept: resolution.dept.trim(), phone: resolution.phone } }
}

/** The check-in window as the header shows it, in Asia/Shanghai. */
export function checkinWindow(event: GuestStatus['event']): string {
  if (!event.checkin_start || !event.checkin_end) return ''
  return `签到时间 ${formatShanghai(event.checkin_start)} 至 ${formatShanghai(event.checkin_end)}`
}
