import { Paper, SimpleGrid, Stack, Text } from '@mantine/core'
import { useStaffSummary } from '#/guest/hooks/useStaff'

function Figure({ label, value }: { label: string; value: number | undefined }) {
  return (
    <Stack gap={0} align="center">
      <Text ff="monospace" fw={700} fz={22}>
        {value ?? '—'}
      </Text>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
    </Stack>
  )
}

export function StaffSummary({ publicId }: { publicId: string }) {
  const summary = useStaffSummary(publicId, true)
  return (
    <Paper withBorder radius="md" p={12}>
      <SimpleGrid cols={3} spacing={8}>
        <Figure label="名单" value={summary.data?.total} />
        <Figure label="已签到" value={summary.data?.checked_in} />
        <Figure label="待处理" value={summary.data?.pending_requests} />
      </SimpleGrid>
    </Paper>
  )
}
