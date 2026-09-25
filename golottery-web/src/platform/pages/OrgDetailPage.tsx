import { Anchor, Group, Paper, SimpleGrid, Stack, Text, Title } from '@mantine/core'
import { IconArrowLeft } from '@tabler/icons-react'
import { Link, useParams } from 'react-router-dom'
import { EmptyState } from '#/components/EmptyState'
import { TableSkeleton } from '#/components/TableSkeleton'
import { formatDateTime } from '#/lib/format'
import { AdjustCreditsForm } from '#/platform/components/AdjustCreditsForm'
import { LedgerTable } from '#/platform/components/LedgerTable'
import { MaxAttendeesForm } from '#/platform/components/MaxAttendeesForm'
import { OrgStatusBadge } from '#/platform/components/OrgStatusBadge'
import { OrgStatusControl } from '#/platform/components/OrgStatusControl'
import { useOrg } from '#/platform/hooks/useOrgs'

function Field({ label, value }: { label: string; value: string | number }) {
  return (
    <Stack gap={4}>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Text size="sm">{value}</Text>
    </Stack>
  )
}

export function OrgDetailPage() {
  const { orgId = '' } = useParams()
  const detail = useOrg(orgId)

  const back = (
    <Anchor component={Link} to="/platform/orgs" size="sm">
      <Group gap={4}>
        <IconArrowLeft size={14} />
        组织列表
      </Group>
    </Anchor>
  )

  if (detail.isPending) {
    return (
      <Stack gap={12}>
        {back}
        <TableSkeleton rows={4} />
      </Stack>
    )
  }
  if (detail.isError) {
    return (
      <Stack gap={12}>
        {back}
        <EmptyState title={detail.error.message} />
      </Stack>
    )
  }

  const { org, ledger } = detail.data
  return (
    <Stack gap={16} maw={960}>
      {back}
      <Group justify="space-between">
        <Group gap={8}>
          <Title order={1} fz={18}>
            {org.name}
          </Title>
          <OrgStatusBadge status={org.status} />
        </Group>
        <OrgStatusControl orgId={org.id} status={org.status} />
      </Group>

      <Paper withBorder radius="md" p={12}>
        <SimpleGrid cols={{ base: 2, sm: 4 }} spacing={16} verticalSpacing={12}>
          <Field label="剩余场次" value={org.event_credits} />
          <Field label="人数上限" value={org.max_attendees} />
          <Field label="联系人" value={org.contact || '—'} />
          <Field label="开通时间" value={formatDateTime(org.created_at)} />
        </SimpleGrid>
      </Paper>

      <SimpleGrid cols={{ base: 1, md: 2 }} spacing={16} verticalSpacing={16}>
        <Paper withBorder radius="md" p={12}>
          <AdjustCreditsForm orgId={org.id} />
        </Paper>
        <Paper withBorder radius="md" p={12}>
          <MaxAttendeesForm key={org.max_attendees} orgId={org.id} current={org.max_attendees} />
        </Paper>
      </SimpleGrid>

      <Stack gap={8}>
        <Text fw={500} size="sm">
          最近 20 条场次流水
        </Text>
        <Paper withBorder radius="md">
          <LedgerTable entries={ledger} />
        </Paper>
      </Stack>
    </Stack>
  )
}
