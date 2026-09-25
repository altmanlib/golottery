import { AppShell, Box, Button, Group, Text } from '@mantine/core'
import { IconLogout } from '@tabler/icons-react'
import { Navigate, Outlet, useNavigate } from 'react-router-dom'
import { getToken } from '#/api'
import { useOrganizationLogout, useOrganizationMe } from '#/organization/hooks/useOrganizationSession'

export function OrganizationShell() {
  const navigate = useNavigate()
  const me = useOrganizationMe()
  const logout = useOrganizationLogout(() => navigate('/organization/login', { replace: true }))

  if (!getToken('console')) {
    return <Navigate to="/organization/login" replace />
  }

  return (
    <AppShell header={{ height: 48 }}>
      <AppShell.Header>
        <Group h="100%" px={24} justify="space-between">
          <Text fw={700}>{me.data?.org_name ?? 'golottery 控制台'}</Text>
          <Group gap={12}>
            {me.data && (
              <Text size="sm" c="dimmed">
                {me.data.name}
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
