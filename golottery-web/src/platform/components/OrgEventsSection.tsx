import { Button, Group, Pagination, Paper, Stack, Table, Text } from '@mantine/core'
import { useState } from 'react'
import type { Event } from '#/api-gen/types.gen'
import { TableSkeleton } from '#/components/TableSkeleton'
import { formatShanghai, MODE_LABELS, STATUS_LABELS } from '#/organization/events'
import { EventLimitModal } from '#/platform/components/EventLimitModal'
import { useOrgEventPage } from '#/platform/hooks/useOrgEvents'
import { pageCount } from '#/platform/orgs'

export function OrgEventsSection({ orgId }: { orgId: string }) {
  const [page, setPage] = useState(1)
  const events = useOrgEventPage(orgId, page)
  const [editing, setEditing] = useState<Event | null>(null)
  const total = events.data?.total ?? 0

  return (
    <Stack gap={8}>
      <Text fw={500} size="sm">
        活动
      </Text>
      <Paper withBorder radius="md">
        {events.isPending ? (
          <TableSkeleton rows={2} />
        ) : events.isError ? (
          <Text size="sm" c="red" p={12}>
            {events.error.message}
          </Text>
        ) : events.data.items.length === 0 ? (
          <Text size="sm" c="dimmed" p={12}>
            还没有活动。组织管理员在 /organization 创建后会出现在这里
          </Text>
        ) : (
          <Table verticalSpacing={8} horizontalSpacing={12}>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>活动</Table.Th>
                <Table.Th>状态</Table.Th>
                <Table.Th>签到方式</Table.Th>
                <Table.Th>签到开始</Table.Th>
                <Table.Th ta="right">名单 / 上限</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {events.data.items.map((event) => (
                <Table.Tr key={event.id}>
                  <Table.Td>{event.name}</Table.Td>
                  <Table.Td>{STATUS_LABELS[event.status]}</Table.Td>
                  <Table.Td>{MODE_LABELS[event.checkin_mode]}</Table.Td>
                  <Table.Td>{formatShanghai(event.checkin_start)}</Table.Td>
                  <Table.Td ta="right" ff="monospace">
                    {event.attendee_count} / {event.max_attendees}
                  </Table.Td>
                  <Table.Td>
                    <Group justify="flex-end">
                      <Button size="compact-xs" variant="subtle" onClick={() => setEditing(event)}>
                        调整上限
                      </Button>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Paper>
      {total > 0 && (
        <Group justify="space-between">
          <Text size="sm" c="dimmed">
            共 {total} 个
          </Text>
          <Pagination size="sm" total={pageCount(total)} value={page} onChange={setPage} />
        </Group>
      )}
      <EventLimitModal key={editing?.id ?? 'closed'} orgId={orgId} event={editing} onClose={() => setEditing(null)} />
    </Stack>
  )
}
