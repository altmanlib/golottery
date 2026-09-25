import { Center, Paper, Stack, Title } from '@mantine/core'
import { Navigate, useNavigate } from 'react-router-dom'
import { getToken } from '#/api'
import { OrgLoginForm } from '#/organization/components/OrgLoginForm'
import { useOrganizationLogin } from '#/organization/hooks/useOrganizationSession'

export function LoginPage() {
  const navigate = useNavigate()
  const login = useOrganizationLogin(() => navigate('/organization', { replace: true }))

  if (getToken('console')) {
    return <Navigate to="/organization" replace />
  }

  return (
    <Center mih="100vh" p={16}>
      <Paper withBorder radius="md" p={24} w={360}>
        <Stack gap={16}>
          <Title order={1} fz={18}>
            组织登录
          </Title>
          <OrgLoginForm pending={login.isPending} error={login.error} onSubmit={(body) => login.mutate(body)} />
        </Stack>
      </Paper>
    </Center>
  )
}
