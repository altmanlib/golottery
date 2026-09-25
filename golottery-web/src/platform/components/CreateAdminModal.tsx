import { Alert, Button, Group, Modal, Stack, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import { type CreateAdminValues, createAdminErrors, emptyCreateAdmin, type Reveal, toCreateAdminBody, validateCreateAdmin } from '#/platform/admins'
import { useCreateOrgUser } from '#/platform/hooks/useOrgUsers'

type Props = {
  orgId: string
  opened: boolean
  onClose: () => void
  onCreated: (reveal: Reveal) => void
}

export function CreateAdminModal({ orgId, opened, onClose, onCreated }: Props) {
  const form = useForm<CreateAdminValues>({ initialValues: emptyCreateAdmin, validate: validateCreateAdmin })
  const create = useCreateOrgUser(orgId)
  const errors = createAdminErrors(create.error)

  const close = () => {
    form.reset()
    create.reset()
    onClose()
  }

  return (
    <Modal opened={opened} onClose={close} title="添加管理员" centered>
      <form
        onSubmit={form.onSubmit((values) =>
          create.mutate(toCreateAdminBody(values), {
            onSuccess: (created) => {
              // Hand the password to the reveal state and drop it from the mutation result.
              close()
              onCreated({ title: '管理员已创建', email: created.user.email, password: created.password })
            },
          }),
        )}>
        <Stack gap={12}>
          {errors.form && (
            <Alert color="red" variant="light">
              {errors.form}
            </Alert>
          )}
          <TextInput label="姓名" withAsterisk maxLength={100} data-autofocus {...form.getInputProps('name')} />
          <TextInput
            label="登录邮箱"
            withAsterisk
            type="email"
            maxLength={254}
            {...form.getInputProps('email')}
            error={form.errors.email ?? errors.email}
            onChange={(event) => {
              create.reset()
              form.setFieldValue('email', event.currentTarget.value)
            }}
          />
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={close}>
              取消
            </Button>
            <Button type="submit" loading={create.isPending}>
              创建
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}
