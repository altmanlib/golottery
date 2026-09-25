import { Stack, Text, Title } from '@mantine/core'

export function HostView() {
  return (
    <Stack p="xl" gap="sm">
      <Title order={2}>大屏</Title>
      <Text c="dimmed">签到人数、抽奖动画与 SSE 将在 M1 实现</Text>
    </Stack>
  )
}
