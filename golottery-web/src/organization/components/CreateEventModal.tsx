import { Alert, Button, Group, Modal, Stack, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import type { Event } from '#/api-gen/types.gen'
import { useCreateEvent } from '#/organization/hooks/useEvents'

type Props = { opened: boolean; onClose: () => void; onCreated: (event: Event) => void }

export function CreateEventModal({ opened, onClose, onCreated }: Props) {
  const form = useForm({ initialValues: { name: '' }, validate: { name: (v) => (v.trim() ? null : '请填写活动名称') } })
  const create = useCreateEvent()
  const close = () => {
    form.reset()
    create.reset()
    onClose()
  }

  return (
    <Modal opened={opened} onClose={close} title="创建活动" centered>
      <form
        onSubmit={form.onSubmit((values) =>
          create.mutate(values.name.trim(), {
            onSuccess: (event) => {
              close()
              onCreated(event)
            },
          }),
        )}>
        <Stack gap={12}>
          {create.error && (
            <Alert color="red" variant="light">
              {create.error.message}
            </Alert>
          )}
          <TextInput label="活动名称" withAsterisk maxLength={100} data-autofocus description="创建草稿不消耗场次" {...form.getInputProps('name')} />
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
