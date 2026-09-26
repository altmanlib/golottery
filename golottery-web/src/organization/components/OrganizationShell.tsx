import { Stack, Text } from '@mantine/core'
import { IconCalendarEvent, IconLogout } from '@tabler/icons-react'
import { Navigate, useNavigate } from 'react-router-dom'
import { getToken } from '#/api'
import { ConsoleShell } from '#/components/ConsoleShell'
import { useOrganizationLogout, useOrganizationMe } from '#/organization/hooks/useOrganizationSession'
import classes from './OrganizationShell.module.css'

export function OrganizationShell() {
  const navigate = useNavigate()
  const me = useOrganizationMe()
  const logout = useOrganizationLogout(() => navigate('/organization/login', { replace: true }))

  if (!getToken('console')) {
    return <Navigate to="/organization/login" replace />
  }

  return (
    <ConsoleShell
      mark="签"
      title={me.data?.org_name ?? 'golottery'}
      subtitle="组织控制台"
      nav={[{ to: '/organization/events', label: '活动', icon: <IconCalendarEvent size={16} /> }]}
      panel={
        <Stack gap={4}>
          <Text size="xs" className={classes.muted}>
            剩余场次
          </Text>
          <Text ff="monospace" fz={24} className={classes.credits}>
            {me.data?.event_credits ?? '—'}
          </Text>
          <Text size="xs" className={classes.muted}>
            首次设为就绪时扣 1 场
          </Text>
        </Stack>
      }
      user={me.data?.name}
      actions={[{ label: '退出登录', icon: <IconLogout size={16} />, loading: logout.isPending, onClick: () => logout.mutate() }]}
    />
  )
}
