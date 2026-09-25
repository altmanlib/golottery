const dateTime = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

export function formatDateTime(iso: string): string {
  return dateTime.format(new Date(iso))
}

/** Signed integer for ledger deltas: +3, -1. */
export function formatDelta(delta: number): string {
  return delta > 0 ? `+${delta}` : String(delta)
}
