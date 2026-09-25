import { Anchor, Table } from '@mantine/core'
import { Link } from 'react-router-dom'
import type { OrgSummary } from '#/api-gen/types.gen'
import { formatDateTime } from '#/lib/format'
import { OrgStatusBadge } from '#/platform/components/OrgStatusBadge'

export function OrgTable({ orgs }: { orgs: OrgSummary[] }) {
  return (
    <Table stickyHeader highlightOnHover verticalSpacing={8} horizontalSpacing={12}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>名称</Table.Th>
          <Table.Th>联系人</Table.Th>
          <Table.Th>状态</Table.Th>
          <Table.Th ta="right">剩余场次</Table.Th>
          <Table.Th ta="right">人数上限</Table.Th>
          <Table.Th>开通时间</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {orgs.map((org) => (
          <Table.Tr key={org.id}>
            <Table.Td>
              <Anchor component={Link} to={`/platform/orgs/${org.id}`} size="sm">
                {org.name}
              </Anchor>
            </Table.Td>
            <Table.Td>{org.contact || '—'}</Table.Td>
            <Table.Td>
              <OrgStatusBadge status={org.status} />
            </Table.Td>
            <Table.Td ta="right" ff="monospace">
              {org.event_credits}
            </Table.Td>
            <Table.Td ta="right" ff="monospace">
              {org.max_attendees}
            </Table.Td>
            <Table.Td>{formatDateTime(org.created_at)}</Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  )
}
