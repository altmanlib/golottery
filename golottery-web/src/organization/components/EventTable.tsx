import { Anchor, Table } from '@mantine/core'
import { Link } from 'react-router-dom'
import type { Event } from '#/api-gen/types.gen'
import { EventStatusBadge } from '#/organization/components/EventStatusBadge'
import { formatShanghai, MODE_LABELS } from '#/organization/events'

export function EventTable({ events }: { events: Event[] }) {
  return (
    <Table stickyHeader highlightOnHover verticalSpacing={8} horizontalSpacing={12}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>活动</Table.Th>
          <Table.Th>状态</Table.Th>
          <Table.Th>签到方式</Table.Th>
          <Table.Th>签到开始</Table.Th>
          <Table.Th ta="right">名单 / 上限</Table.Th>
          <Table.Th ta="right">奖项</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {events.map((event) => (
          <Table.Tr key={event.id}>
            <Table.Td>
              <Anchor component={Link} to={`/organization/events/${event.id}`} size="sm">
                {event.name}
              </Anchor>
            </Table.Td>
            <Table.Td>
              <EventStatusBadge status={event.status} />
            </Table.Td>
            <Table.Td>{MODE_LABELS[event.checkin_mode]}</Table.Td>
            <Table.Td>{formatShanghai(event.checkin_start)}</Table.Td>
            <Table.Td ta="right" ff="monospace">
              {event.attendee_count} / {event.max_attendees}
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
