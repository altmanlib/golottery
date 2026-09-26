import { Paper, SimpleGrid, Stack, Text } from '@mantine/core'
import { useParams } from 'react-router-dom'
import { EmptyState } from '#/components/EmptyState'
import { Page, PageHeader } from '#/components/Page'
import { TableSkeleton } from '#/components/TableSkeleton'
import { formatDateTime } from '#/lib/format'
import { AdjustCreditsForm } from '#/platform/components/AdjustCreditsForm'
import { AdminSection } from '#/platform/components/AdminSection'
import { LedgerTable } from '#/platform/components/LedgerTable'
import { MaxAttendeesForm } from '#/platform/components/MaxAttendeesForm'
import { OrgEventsSection } from '#/platform/components/OrgEventsSection'
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

  const back = { to: '/platform/orgs', label: '组织' }

  if (detail.isPending || detail.isError) {
    return (
      <Page>
        <PageHeader title="组织" back={back} />
        {detail.isPending ? <TableSkeleton rows={4} /> : <EmptyState title={detail.error.message} />}
      </Page>
    )
  }

  const { org, ledger } = detail.data
  return (
    <Page>
      <PageHeader
        back={back}
        title={org.name}
        badge={<OrgStatusBadge status={org.status} />}
        actions={<OrgStatusControl orgId={org.id} status={org.status} />}
      />
      <Stack gap={16} maw={960}>
        <Paper withBorder p={16}>
          <SimpleGrid cols={{ base: 2, sm: 4 }} spacing={16} verticalSpacing={12}>
            <Field label="剩余场次" value={org.event_credits} />
            <Field label="人数上限" value={org.max_attendees} />
            <Field label="联系人" value={org.contact || '—'} />
            <Field label="开通时间" value={formatDateTime(org.created_at)} />
          </SimpleGrid>
        </Paper>

        <SimpleGrid cols={{ base: 1, md: 2 }} spacing={16} verticalSpacing={16}>
          <Paper withBorder p={16}>
            <AdjustCreditsForm orgId={org.id} />
          </Paper>
          <Paper withBorder p={16}>
            <MaxAttendeesForm key={org.max_attendees} orgId={org.id} current={org.max_attendees} />
          </Paper>
        </SimpleGrid>

        <AdminSection orgId={org.id} />

        <OrgEventsSection orgId={org.id} />

        <Stack gap={8}>
          <Text fw={500} size="sm">
            最近 20 条场次流水
          </Text>
          <Paper withBorder>
            <LedgerTable entries={ledger} />
          </Paper>
        </Stack>
      </Stack>
    </Page>
  )
}
