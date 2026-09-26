import { Box, Group } from '@mantine/core'
import type { EventStatus } from '#/api-gen/types.gen'
import { STATUS_LABELS } from '#/organization/events'
import classes from './EventStatusBadge.module.css'

export function EventStatusBadge({ status }: { status: EventStatus }) {
  return (
    <Group component="span" className={classes.badge} data-status={status} gap={6} wrap="nowrap" display="inline-flex">
      <Box component="span" className={classes.dot} />
      {STATUS_LABELS[status]}
    </Group>
  )
}
