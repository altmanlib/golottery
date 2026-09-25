import { Alert, Button, PasswordInput, Stack, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import type { PlatformLoginRequest } from '#/api-gen/types.gen'
import { emptyLogin, type LoginValues, loginFailureMessage, toLoginBody, validateLogin } from '#/platform/login'

type Props = {
  pending: boolean
  error: unknown
  onSubmit: (body: PlatformLoginRequest) => void
}

export function LoginForm({ pending, error, onSubmit }: Props) {
  const form = useForm<LoginValues>({ initialValues: emptyLogin, validate: validateLogin })
  const failure = loginFailureMessage(error)

  return (
    <form onSubmit={form.onSubmit((values) => onSubmit(toLoginBody(values)))}>
      <Stack gap={12}>
        {failure && (
          <Alert color="red" variant="light">
            {failure}
          </Alert>
        )}
        <TextInput label="账号" autoComplete="username" autoFocus {...form.getInputProps('username')} />
        <PasswordInput label="口令" autoComplete="current-password" {...form.getInputProps('password')} />
        <Button type="submit" loading={pending} fullWidth>
          登录
        </Button>
      </Stack>
    </form>
  )
}
