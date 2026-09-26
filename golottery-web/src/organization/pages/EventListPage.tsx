import { Box, Button, Group, Pagination, ScrollArea, Stack, Text } from '@mantine/core'
import { IconPlus } from '@tabler/icons-react'
import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { EmptyState } from '#/components/EmptyState'
import { Card, Page, PageHeader } from '#/components/Page'
import { TableSkeleton } from '#/components/TableSkeleton'
import { parsePage } from '#/lib/paging'
import { CreateEventModal } from '#/organization/components/CreateEventModal'
import { EventTable } from '#/organization/components/EventTable'
import { PAGE_SIZE } from '#/organization/events'
import { useEventPage } from '#/organization/hooks/useEvents'
import classes from './EventListPage.module.css'

export function EventListPage() {
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const page = parsePage(params.get('page'))
  const events = useEventPage(page)
  const [creating, setCreating] = useState(false)
  const total = events.data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const createButton = (
    <Button leftSection={<IconPlus size={16} />} onClick={() => setCreating(true)}>
      创建活动
    </Button>
  )

  return (
    <Page fill>
      <PageHeader title="活动" description="从创建到抽奖，一场活动的全部配置都在这里" actions={createButton} />
      <Card p={0} flex={1} mih={0}>
        <Stack h="100%" gap={0}>
          <Box flex={1} mih={0}>
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
          </Box>
          <Group className={classes.footer} justify="space-between" px={16} py={12}>
            <Text size="sm" c="dimmed">
              共 {total} 场
            </Text>
            <Pagination size="sm" total={pages} value={page} onChange={(next) => setParams(next === 1 ? {} : { page: String(next) })} />
          </Group>
        </Stack>
      </Card>
      <CreateEventModal opened={creating} onClose={() => setCreating(false)} onCreated={(event) => navigate(`/organization/events/${event.id}`)} />
    </Page>
  )
}
