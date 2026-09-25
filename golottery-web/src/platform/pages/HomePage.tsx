import { Stack, Text, Title } from '@mantine/core'

export function HomePage() {
  return (
    <Stack gap={8}>
      <Title order={1} fz={18}>
        运营后台
      </Title>
      <Text size="sm" c="dimmed">
        组织开通与场次配额在阶段 3 接入
      </Text>
    </Stack>
  )
}
