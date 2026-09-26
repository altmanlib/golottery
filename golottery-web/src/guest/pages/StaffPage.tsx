import { Alert, Anchor, Badge, Center, Group, Loader, Tabs } from '@mantine/core'
import { useEffect, useRef } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { EmptyState } from '#/components/EmptyState'
import { GuestShell } from '#/guest/components/GuestShell'
import { ProxyPanel } from '#/guest/components/ProxyPanel'
import { RequestList } from '#/guest/components/RequestList'
import { SettingsPanel } from '#/guest/components/SettingsPanel'
import { StaffSummary } from '#/guest/components/StaffSummary'
import { errorMessage } from '#/guest/guest'
import { useGuestStatus, useJoinStaff } from '#/guest/hooks/useGuest'

export function StaffPage() {
  const { publicId = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const invite = params.get('invite')
  const status = useGuestStatus(publicId)
  const join = useJoinStaff(publicId)
  // Invites are single-use: redeem once, even when React runs the effect twice.
  const redeemed = useRef(false)

  useEffect(() => {
    if (!invite || redeemed.current) return
    redeemed.current = true
    join.mutate(invite, { onSettled: () => setParams({}, { replace: true }) })
  }, [invite, join, setParams])

  if (status.isPending || join.isPending) {
    return (
      <Center mih="100dvh">
        <Loader />
      </Center>
    )
  }
  if (status.isError) {
    return (
      <GuestShell title="现场工作台">
        <EmptyState title={status.error.message} />
      </GuestShell>
    )
  }

  const data = status.data
  const role = data.staff_role
  const self = (
    <Anchor component={Link} to={`/m/${publicId}`} size="sm" ta="center">
      我的签到
    </Anchor>
  )
  if (!role) {
    return (
      <GuestShell title={data.event.name}>
        {join.error && (
          <Alert color="red" variant="light">
            {errorMessage(join.error)}
          </Alert>
        )}
        <EmptyState title="你还不是本活动的工作人员" description="请用组织方发来的邀请链接打开本页" />
        {self}
      </GuestShell>
    )
  }

  return (
    <GuestShell title={data.event.name} subtitle={role === 'admin' ? '现场工作台 · 管理员' : '现场工作台'}>
      <StaffSummary publicId={publicId} />
      <Tabs defaultValue="requests" keepMounted={false}>
        <Tabs.List grow>
          <Tabs.Tab value="requests">协助请求</Tabs.Tab>
          <Tabs.Tab value="proxy">代签</Tabs.Tab>
          {role === 'admin' && <Tabs.Tab value="settings">现场设置</Tabs.Tab>}
        </Tabs.List>
        <Tabs.Panel value="requests" pt={12}>
          <RequestList publicId={publicId} />
        </Tabs.Panel>
        <Tabs.Panel value="proxy" pt={12}>
          <ProxyPanel publicId={publicId} />
        </Tabs.Panel>
        {role === 'admin' && (
          <Tabs.Panel value="settings" pt={12}>
            <SettingsPanel publicId={publicId} status={data} />
          </Tabs.Panel>
        )}
      </Tabs>
      <Group justify="center">
        {self}
        <Badge variant="light">{data.event.status === 'ready' ? '签到进行中' : data.event.status === 'closed' ? '已结束' : '未开放'}</Badge>
      </Group>
    </GuestShell>
  )
}
