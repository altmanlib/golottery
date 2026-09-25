import { Alert, Button, Group, Modal, NumberInput, Stack, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import { useEffect } from 'react'
import type { OrgSummary } from '#/api-gen/types.gen'
import { useCreateOrg } from '#/platform/hooks/useOrgMutations'
import { ATTENDEE_TIERS, type CreateOrgValues, createOrgErrors, emptyCreateOrg, toCreateOrgBody, validateCreateOrg } from '#/platform/orgs'

type Props = {
  opened: boolean
  onClose: () => void
  onCreated: (org: OrgSummary) => void
}

const tierHint = ATTENDEE_TIERS.map((tier) => `${tier.label} ${tier.value}`).join(' · ')

export function CreateOrgModal({ opened, onClose, onCreated }: Props) {
  const form = useForm<CreateOrgValues>({ initialValues: emptyCreateOrg, validate: validateCreateOrg })
  const create = useCreateOrg()
  const errors = createOrgErrors(create.error)

  useEffect(() => {
    if (errors.name) {
      form.setFieldError('name', errors.name)
    }
  }, [errors.name, form.setFieldError])

  const close = () => {
    form.reset()
    create.reset()
    onClose()
  }

  return (
    <Modal opened={opened} onClose={close} title="开通组织" centered>
      <form
        onSubmit={form.onSubmit((values) =>
          create.mutate(toCreateOrgBody(values), {
            onSuccess: (org) => {
              close()
              onCreated(org)
            },
          }),
        )}>
        <Stack gap={12}>
          {errors.form && (
            <Alert color="red" variant="light">
              {errors.form}
            </Alert>
          )}
          <TextInput label="组织名称" withAsterisk maxLength={100} data-autofocus {...form.getInputProps('name')} />
          <TextInput label="联系人" maxLength={100} {...form.getInputProps('contact')} />
          <Group grow align="flex-start">
            <NumberInput label="初始场次" min={0} allowDecimal={false} allowNegative={false} {...form.getInputProps('eventCredits')} />
            <NumberInput label="人数上限" description={tierHint} min={1} allowDecimal={false} allowNegative={false} {...form.getInputProps('maxAttendees')} />
          </Group>
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={close}>
              取消
            </Button>
            <Button type="submit" loading={create.isPending}>
              开通
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}
