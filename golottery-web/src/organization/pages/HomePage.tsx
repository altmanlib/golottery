import { Stack, Text, Title } from '@mantine/core'
import { useOrganizationMe } from '#/organization/hooks/useOrganizationSession'

export function HomePage() {
  const me = useOrganizationMe()

  return (
    <Stack gap={8}>
      <Title order={1} fz={18}>
        {me.data?.org_name ?? '控制台'}
      </Title>
      <Text size="sm" c="dimmed">
        活动配置在阶段 5 接入
      </Text>
    </Stack>
  )
}
