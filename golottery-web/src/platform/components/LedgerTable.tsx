import { Table, Text } from '@mantine/core'
import type { LedgerEntry } from '#/api-gen/types.gen'
import { formatDateTime, formatDelta } from '#/lib/format'

export function LedgerTable({ entries }: { entries: LedgerEntry[] }) {
  if (entries.length === 0) {
    return (
      <Text size="sm" c="dimmed" p={12}>
        还没有场次流水
      </Text>
    )
  }
  return (
    <Table verticalSpacing={8} horizontalSpacing={12}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>时间</Table.Th>
          <Table.Th ta="right">变动</Table.Th>
          <Table.Th ta="right">余额</Table.Th>
          <Table.Th>原因</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {entries.map((entry) => (
          <Table.Tr key={entry.id}>
            <Table.Td>{formatDateTime(entry.created_at)}</Table.Td>
            <Table.Td ta="right" ff="monospace" c={entry.delta > 0 ? 'brand' : 'red'}>
              {formatDelta(entry.delta)}
            </Table.Td>
            <Table.Td ta="right" ff="monospace">
              {entry.balance_after}
            </Table.Td>
            <Table.Td>{entry.reason}</Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  )
}
