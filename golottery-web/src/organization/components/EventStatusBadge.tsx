import { Badge } from '@mantine/core'
import type { EventStatus } from '#/api-gen/types.gen'
import { STATUS_LABELS } from '#/organization/events'

const COLORS: Record<EventStatus, string> = { draft: 'gray', ready: 'brand', closed: 'dark' }

export function EventStatusBadge({ status }: { status: EventStatus }) {
  return (
    <Badge variant="light" color={COLORS[status]}>
      {STATUS_LABELS[status]}
    </Badge>
  )
}
