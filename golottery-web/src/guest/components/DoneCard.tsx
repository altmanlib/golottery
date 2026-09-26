import { Paper, Stack, Text, ThemeIcon } from '@mantine/core'
import { IconCheck } from '@tabler/icons-react'
import type { GuestAttendee } from '#/api-gen/types.gen'
import { METHOD_LABELS } from '#/guest/guest'
import { formatShanghai } from '#/organization/events'

export function DoneCard({ attendee }: { attendee: GuestAttendee }) {
  return (
    <Paper withBorder radius="md" p={24}>
      <Stack gap={8} align="center">
        <ThemeIcon size={48} radius="xl" color="green">
          <IconCheck size={28} />
        </ThemeIcon>
        <Text fw={700} fz={20}>
          签到成功
        </Text>
        <Text>
          {attendee.name}
          {attendee.dept ? ` · ${attendee.dept}` : ''}
        </Text>
        <Text size="sm" c="dimmed">
          {formatShanghai(attendee.checkin_at)}
          {attendee.checkin_method ? ` · ${METHOD_LABELS[attendee.checkin_method]}` : ''}
        </Text>
        <Text size="sm" c="dimmed">
          请留意现场抽奖
        </Text>
      </Stack>
    </Paper>
  )
}
