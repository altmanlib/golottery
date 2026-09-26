import { Button, Group, Pagination, Paper, ScrollArea, Stack, Text, Title } from '@mantine/core'
import { IconPlus } from '@tabler/icons-react'
import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { EmptyState } from '#/components/EmptyState'
import { TableSkeleton } from '#/components/TableSkeleton'
import { parsePage } from '#/lib/paging'
import { CreateEventModal } from '#/organization/components/CreateEventModal'
import { EventTable } from '#/organization/components/EventTable'
import { PAGE_SIZE } from '#/organization/events'
import { useEventPage } from '#/organization/hooks/useEvents'
import { useOrganizationMe } from '#/organization/hooks/useOrganizationSession'

/** Header 48 + main padding 16×2: the table area takes the rest and scrolls on its own. */
const PAGE_HEIGHT = 'calc(100dvh - 80px)'

export function EventListPage() {
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const page = parsePage(params.get('page'))
  const events = useEventPage(page)
  const me = useOrganizationMe()
  const [creating, setCreating] = useState(false)
  const total = events.data?.total ?? 0

  const createButton = (
    <Button size="xs" leftSection={<IconPlus size={16} />} onClick={() => setCreating(true)}>
      创建活动
    </Button>
  )

  return (
    <Stack h={PAGE_HEIGHT} gap={12}>
      <Group justify="space-between">
        <Group gap={12}>
          <Title order={1} fz={18}>
            活动
          </Title>
          {me.data && (
            <Text size="sm" c="dimmed">
              剩余场次 {me.data.event_credits}
            </Text>
          )}
        </Group>
        {createButton}
      </Group>
      <Paper withBorder radius="md" flex={1} mih={0}>
        {events.isPending ? (
          <TableSkeleton />
        ) : events.isError ? (
          <EmptyState title="活动列表加载失败" description={events.error.message} />
        ) : events.data.items.length === 0 ? (
          <EmptyState title="还没有活动" description="创建草稿不消耗场次，首次设为就绪时扣 1 场" action={createButton} />
        ) : (
          <ScrollArea h="100%">
            <EventTable events={events.data.items} />
          </ScrollArea>
        )}
      </Paper>
      <Group justify="space-between">
        <Text size="sm" c="dimmed">
          共 {total} 场
        </Text>
        <Pagination
          size="sm"
          total={Math.max(1, Math.ceil(total / PAGE_SIZE))}
          value={page}
          onChange={(next) => setParams(next === 1 ? {} : { page: String(next) })}
        />
      </Group>
      <CreateEventModal opened={creating} onClose={() => setCreating(false)} onCreated={(event) => navigate(`/organization/events/${event.id}`)} />
    </Stack>
  )
}
