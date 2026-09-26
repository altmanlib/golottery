import { IconBuildingSkyscraper, IconKey, IconLogout } from '@tabler/icons-react'
import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { getToken } from '#/api'
import { ConsoleShell } from '#/components/ConsoleShell'
import { ChangePasswordModal } from '#/platform/components/ChangePasswordModal'
import { usePlatformLogout } from '#/platform/hooks/usePlatformLogout'
import { usePlatformMe } from '#/platform/hooks/usePlatformMe'

export function PlatformShell() {
  const navigate = useNavigate()
  const me = usePlatformMe()
  const logout = usePlatformLogout(() => navigate('/platform/login', { replace: true }))
  const [changingPassword, setChangingPassword] = useState(false)

  if (!getToken('platform')) {
    return <Navigate to="/platform/login" replace />
  }

  return (
    <>
      <ConsoleShell
        mark="运"
        title="golottery"
        subtitle="运营后台"
        nav={[{ to: '/platform/orgs', label: '组织', icon: <IconBuildingSkyscraper size={16} /> }]}
        user={me.data?.username}
        actions={[
          { label: '修改口令', icon: <IconKey size={16} />, onClick: () => setChangingPassword(true) },
          { label: '退出登录', icon: <IconLogout size={16} />, loading: logout.isPending, onClick: () => logout.mutate() },
        ]}
      />
      <ChangePasswordModal opened={changingPassword} onClose={() => setChangingPassword(false)} />
    </>
  )
}
