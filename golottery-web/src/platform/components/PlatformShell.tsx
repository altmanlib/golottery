import { Anchor, AppShell, Box, Button, Group, Text } from '@mantine/core'
import { IconLogout } from '@tabler/icons-react'
import { Link, Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { getToken } from '#/api'
import { usePlatformLogout } from '#/platform/hooks/usePlatformLogout'
import { usePlatformMe } from '#/platform/hooks/usePlatformMe'

function NavLink({ to, children }: { to: string; children: string }) {
  const { pathname } = useLocation()
  const active = pathname.startsWith(to)
  return (
    <Anchor component={Link} to={to} size="sm" c={active ? 'brand' : 'dimmed'} fw={active ? 500 : 400} underline="never">
      {children}
    </Anchor>
  )
}

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
          <Group gap={24}>
            <Text fw={700}>golottery 运营后台</Text>
            <NavLink to="/platform/orgs">组织</NavLink>
          </Group>
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
      {/* Padding goes on an inner Box: spacing props on AppShell.Main would replace its header offset. */}
      <AppShell.Main>
        <Box px={24} py={16}>
          <Outlet />
        </Box>
      </AppShell.Main>
    </AppShell>
  )
}
