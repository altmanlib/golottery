import { Badge, Card, Group, Stack, Text, Title } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import '#/api'
import { getHealthz } from '#/api-gen/sdk.gen'
import type { Healthz } from '#/api-gen/types.gen'

export function ConsoleHomeView() {
  const health = useQuery({
    queryKey: ['healthz'],
    queryFn: async () => {
      const { data, error } = await getHealthz()
      if (error) {
        throw error
      }
      return data as Healthz
    },
  })

  return (
    <Stack gap="lg" maw={720}>
      <div>
        <Title order={2}>控制台</Title>
        <Text c="dimmed">组织登录、活动配置、名单导入导出将在 M1/M2 实现</Text>
      </div>

      <Card padding="lg" radius="md">
        <Group justify="space-between" mb="sm">
          <Text fw={600}>API 健康检查</Text>
          {health.isLoading && <Badge color="gray">checking</Badge>}
          {health.isError && <Badge color="red">offline</Badge>}
          {health.data?.ok && <Badge color="green">ok</Badge>}
          {health.data && !health.data.ok && <Badge color="yellow">degraded</Badge>}
        </Group>
        <Text size="sm" c="dimmed" style={{ whiteSpace: 'pre-wrap' }}>
          {health.isError ? '无法连接 API。请先启动 golottery-api。' : JSON.stringify(health.data ?? {}, null, 2)}
        </Text>
      </Card>
    </Stack>
  )
}
