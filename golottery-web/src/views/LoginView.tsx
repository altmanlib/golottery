import { Button, Card, PasswordInput, Stack, TextInput, Title } from '@mantine/core'
import { useNavigate } from 'react-router-dom'

export function LoginView() {
  const navigate = useNavigate()

  return (
    <Stack maw={420} mx="auto" mt={80} gap="lg">
      <Title order={2}>组织登录</Title>
      <Card padding="lg" radius="md">
        <Stack gap="md">
          <TextInput label="邮箱" placeholder="admin@example.com" />
          <PasswordInput label="口令" placeholder="••••••••" />
          <Button
            onClick={() => {
              navigate('/console')
            }}>
            进入控制台（骨架）
          </Button>
        </Stack>
      </Card>
    </Stack>
  )
}
