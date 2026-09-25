import { Button, Group, Pagination, Paper, ScrollArea, Stack, Text, Title } from '@mantine/core'
import { IconPlus } from '@tabler/icons-react'
import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { EmptyState } from '#/components/EmptyState'
import { TableSkeleton } from '#/components/TableSkeleton'
import { CreateOrgModal } from '#/platform/components/CreateOrgModal'
import { OrgTable } from '#/platform/components/OrgTable'
import { useOrgPage } from '#/platform/hooks/useOrgs'
import { pageCount, parsePage } from '#/platform/orgs'

/** Header 48 + main padding 16×2: the table area takes the rest and scrolls on its own. */
const PAGE_HEIGHT = 'calc(100dvh - 80px)'

export function OrgListPage() {
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const page = parsePage(params.get('page'))
  const orgs = useOrgPage(page)
  const [creating, setCreating] = useState(false)
  const total = orgs.data?.total ?? 0

  const createButton = (
    <Button size="xs" leftSection={<IconPlus size={16} />} onClick={() => setCreating(true)}>
      开通组织
    </Button>
  )

  return (
    <Stack h={PAGE_HEIGHT} gap={12}>
      <Group justify="space-between">
        <Title order={1} fz={18}>
          组织
        </Title>
        {createButton}
      </Group>
      <Paper withBorder radius="md" flex={1} mih={0}>
        {orgs.isPending ? (
          <TableSkeleton />
        ) : orgs.isError ? (
          <EmptyState
            title="组织列表加载失败"
            description={orgs.error.message}
            action={
              <Button size="xs" variant="default" onClick={() => orgs.refetch()}>
                重试
              </Button>
            }
          />
        ) : orgs.data.items.length === 0 ? (
          <EmptyState title="还没有组织" description="开通第一个组织后，它的管理员才能创建活动" action={createButton} />
        ) : (
          <ScrollArea h="100%">
            <OrgTable orgs={orgs.data.items} />
          </ScrollArea>
        )}
      </Paper>
      <Group justify="space-between">
        <Text size="sm" c="dimmed">
          共 {total} 个
        </Text>
        <Pagination size="sm" total={pageCount(total)} value={page} onChange={(next) => setParams(next === 1 ? {} : { page: String(next) })} />
      </Group>
      <CreateOrgModal opened={creating} onClose={() => setCreating(false)} onCreated={(org) => navigate(`/platform/orgs/${org.id}`)} />
    </Stack>
  )
}
