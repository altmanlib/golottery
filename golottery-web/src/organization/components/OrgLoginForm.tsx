import { Alert, Button, PasswordInput, Stack, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import type { OrganizationLoginRequest } from '#/api-gen/types.gen'
import { emptyOrgLogin, type OrgLoginValues, orgLoginFailureMessage, toOrgLoginBody, validateOrgLogin } from '#/organization/login'

type Props = {
  pending: boolean
  error: unknown
  onSubmit: (body: OrganizationLoginRequest) => void
}

export function OrgLoginForm({ pending, error, onSubmit }: Props) {
  const form = useForm<OrgLoginValues>({ initialValues: emptyOrgLogin, validate: validateOrgLogin })
  const failure = orgLoginFailureMessage(error)

  return (
    <form onSubmit={form.onSubmit((values) => onSubmit(toOrgLoginBody(values)))}>
      <Stack gap={12}>
        {failure && (
          <Alert color="red" variant="light">
            {failure}
          </Alert>
        )}
        <TextInput label="邮箱" type="email" autoComplete="username" autoFocus {...form.getInputProps('email')} />
        <PasswordInput label="口令" autoComplete="current-password" {...form.getInputProps('password')} />
        <Button type="submit" loading={pending} fullWidth>
          登录
        </Button>
      </Stack>
    </form>
  )
}
