import { Center, Paper, Stack, Title } from '@mantine/core'
import { Navigate, useNavigate } from 'react-router-dom'
import { getToken } from '#/api'
import { LoginForm } from '#/platform/components/LoginForm'
import { usePlatformLogin } from '#/platform/hooks/usePlatformLogin'

export function LoginPage() {
  const navigate = useNavigate()
  const login = usePlatformLogin(() => navigate('/platform', { replace: true }))

  if (getToken('platform')) {
    return <Navigate to="/platform" replace />
  }

  return (
    <Center mih="100vh" p={16}>
      <Paper withBorder radius="md" p={24} w={360}>
        <Stack gap={16}>
          <Title order={1} fz={18}>
            运营登录
          </Title>
          <LoginForm pending={login.isPending} error={login.error} onSubmit={(body) => login.mutate(body)} />
        </Stack>
      </Paper>
    </Center>
  )
}
