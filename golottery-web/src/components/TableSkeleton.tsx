import { Skeleton, Stack } from '@mantine/core'

type Props = { rows?: number }

export function TableSkeleton({ rows = 8 }: Props) {
  return (
    <Stack gap={8} p={12}>
      {Array.from({ length: rows }, (_, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: placeholder rows have no identity
        <Skeleton key={i} h={24} radius="sm" />
      ))}
    </Stack>
  )
}
