import { Alert, Button, Group, Modal, PasswordInput, Stack } from '@mantine/core'
import { useForm } from '@mantine/form'
import { useEffect } from 'react'
import { showToast } from '#/lib/message'
import { useChangePlatformPassword } from '#/platform/hooks/useChangePlatformPassword'
import { emptyPassword, type PasswordValues, passwordErrors, toPasswordBody, validatePassword } from '#/platform/password'

type Props = {
  opened: boolean
  onClose: () => void
}

export function ChangePasswordModal({ opened, onClose }: Props) {
  const form = useForm<PasswordValues>({ initialValues: emptyPassword, validate: validatePassword })
  const change = useChangePlatformPassword()
  const errors = passwordErrors(change.error)

  useEffect(() => {
    if (errors.currentPassword) {
      form.setFieldError('currentPassword', errors.currentPassword)
    }
    if (errors.newPassword) {
      form.setFieldError('newPassword', errors.newPassword)
    }
  }, [errors.currentPassword, errors.newPassword, form.setFieldError])

  const close = () => {
    form.reset()
    change.reset()
    onClose()
  }

  return (
    <Modal opened={opened} onClose={close} title="修改口令" centered>
      <form
        onSubmit={form.onSubmit((values) =>
          change.mutate(toPasswordBody(values), {
            onSuccess: () => {
              close()
              showToast('口令已更新', 'success')
            },
          }),
        )}>
        <Stack gap={12}>
          {errors.form && (
            <Alert color="red" variant="light">
              {errors.form}
            </Alert>
          )}
          <PasswordInput label="当前口令" autoComplete="current-password" data-autofocus {...form.getInputProps('currentPassword')} />
          <PasswordInput label="新口令" description="至少 8 位" autoComplete="new-password" {...form.getInputProps('newPassword')} />
          <PasswordInput label="确认新口令" autoComplete="new-password" {...form.getInputProps('confirmPassword')} />
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={close}>
              取消
            </Button>
            <Button type="submit" loading={change.isPending}>
              保存
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}
