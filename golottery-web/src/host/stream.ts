import { getToken } from '#/api'
import type { DrawResult, HostSnapshot } from '#/api-gen/types.gen'

export type HostStreamEvent =
  | { type: 'snapshot'; data: HostSnapshot }
  | { type: 'stats'; data: { checked_in: number; draw_version: number } }
  | { type: 'draw' | 'void'; data: { draw_version: number; results: DrawResult[] } }

/** Reads an SSE response with a Bearer token; EventSource cannot set Authorization. */
export async function readHostStream(signal: AbortSignal, onEvent: (event: HostStreamEvent) => void): Promise<void> {
  const token = getToken('host')
  if (!token) {
    throw new Error('missing host token')
  }
  const response = await fetch('/api/host/stream', {
    headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
    signal,
  })
  if (!response.ok || !response.body) {
    throw new Error(`stream ${response.status}`)
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) {
      return
    }
    buffer += decoder.decode(value, { stream: true })
    for (;;) {
      const sep = buffer.indexOf('\n\n')
      if (sep < 0) {
        break
      }
      const chunk = buffer.slice(0, sep)
      buffer = buffer.slice(sep + 2)
      const parsed = parseSSE(chunk)
      if (parsed) {
        onEvent(parsed)
      }
    }
  }
}

function parseSSE(chunk: string): HostStreamEvent | null {
  let type = 'message'
  const data: string[] = []
  for (const line of chunk.split('\n')) {
    if (line.startsWith('event:')) {
      type = line.slice(6).trim()
    } else if (line.startsWith('data:')) {
      data.push(line.slice(5).trim())
    }
  }
  if (data.length === 0) {
    return null
  }
  const payload = JSON.parse(data.join('\n'))
  if (type === 'snapshot') {
    return { type, data: payload as HostSnapshot }
  }
  if (type === 'stats') {
    return { type, data: payload as { checked_in: number; draw_version: number } }
  }
  if (type === 'draw' || type === 'void') {
    return { type, data: payload as { draw_version: number; results: DrawResult[] } }
  }
  return null
}
