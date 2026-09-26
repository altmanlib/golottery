import { Anchor, Group, Stack, Table, Text } from '@mantine/core'
import { Link } from 'react-router-dom'
import type { Event } from '#/api-gen/types.gen'
import { Meter } from '#/components/Meter'
import { EventStatusBadge } from '#/organization/components/EventStatusBadge'
import { formatShanghai, MODE_LABELS } from '#/organization/events'
import classes from './EventTable.module.css'

export function EventTable({ events }: { events: Event[] }) {
  return (
    <Table stickyHeader highlightOnHover verticalSpacing={12} horizontalSpacing={16} classNames={{ th: classes.th, td: classes.td }}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>活动</Table.Th>
          <Table.Th>状态</Table.Th>
          <Table.Th>签到方式</Table.Th>
          <Table.Th>签到开始</Table.Th>
          <Table.Th w={240}>名单 / 上限</Table.Th>
          <Table.Th ta="right">奖项</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {events.map((event) => (
          <Table.Tr key={event.id}>
            <Table.Td>
              <Stack gap={2}>
                <Anchor component={Link} to={`/organization/events/${event.id}`} className={classes.name}>
                  {event.name}
                </Anchor>
                <Text size="xs" className={classes.meta}>
                  创建于 {formatShanghai(event.created_at).slice(0, 10)}
                </Text>
              </Stack>
            </Table.Td>
            <Table.Td>
              <EventStatusBadge status={event.status} />
            </Table.Td>
            <Table.Td>{MODE_LABELS[event.checkin_mode]}</Table.Td>
            <Table.Td>
              {event.checkin_start ? (
                <Text ff="monospace" size="sm">
                  {formatShanghai(event.checkin_start)}
                </Text>
              ) : (
                <Text size="sm" className={classes.unset}>
                  未设置
                </Text>
              )}
            </Table.Td>
            <Table.Td>
              <Group gap={12} wrap="nowrap">
                <Meter value={event.attendee_count} max={event.max_attendees} label="名单人数" />
                <Text ff="monospace" size="sm" w={80}>
                  {event.attendee_count}/{event.max_attendees}
                </Text>
              </Group>
            </Table.Td>
            <Table.Td ta="right" ff="monospace">
              {event.prize_count}
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  )
}
