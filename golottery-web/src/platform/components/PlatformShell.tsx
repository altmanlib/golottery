import { AppShell, Button, Group, Text } from '@mantine/core'
import { IconLogout } from '@tabler/icons-react'
import { Navigate, Outlet, useNavigate } from 'react-router-dom'
import { getToken } from '#/api'
import { usePlatformLogout } from '#/platform/hooks/usePlatformLogout'
import { usePlatformMe } from '#/platform/hooks/usePlatformMe'

export function PlatformShell() {
  const navigate = useNavigate()
  const me = usePlatformMe()
  const logout = usePlatformLogout(() => navigate('/platform/login', { replace: true }))

  if (!getToken('platform')) {
    return <Navigate to="/platform/login" replace />
  }

  return (
    <AppShell header={{ height: 48 }}>
      <AppShell.Header>
        <Group h="100%" px={24} justify="space-between">
          <Text fw={700}>golottery 运营后台</Text>
          <Group gap={12}>
            {me.data && (
              <Text size="sm" c="dimmed">
                {me.data.username}
              </Text>
            )}
            <Button variant="subtle" size="xs" leftSection={<IconLogout size={16} />} loading={logout.isPending} onClick={() => logout.mutate()}>
              退出登录
            </Button>
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Main px={24} py={16}>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
