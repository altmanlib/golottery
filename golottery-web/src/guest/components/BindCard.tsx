import { Alert, Button, Paper, Stack, Text, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import { type BindValues, errorMessage, validateBind } from '#/guest/guest'
import { useBind } from '#/guest/hooks/useGuest'

export function BindCard({ publicId, onHelp }: { publicId: string; onHelp: (error: unknown) => void }) {
  const bind = useBind(publicId)
  const form = useForm<BindValues>({ initialValues: { name: '', phoneLast4: '' }, validate: validateBind })
  const submit = (values: BindValues) => bind.mutate(values, { onError: onHelp })

  return (
    <Paper withBorder radius="md" p={16}>
      <form onSubmit={form.onSubmit(submit)}>
        <Stack gap={12}>
          <Stack gap={4}>
            <Text fw={500}>确认身份</Text>
            <Text size="sm" c="dimmed">
              填写名单上的姓名和手机号后四位，只需确认一次
            </Text>
          </Stack>
          {bind.error && (
            <Alert color="red" variant="light">
              {errorMessage(bind.error)}
            </Alert>
          )}
          <TextInput label="姓名" maxLength={50} autoComplete="name" {...form.getInputProps('name')} />
          <TextInput label="手机号后四位" inputMode="numeric" maxLength={4} {...form.getInputProps('phoneLast4')} />
          <Button type="submit" size="md" loading={bind.isPending}>
            确认
          </Button>
        </Stack>
      </form>
    </Paper>
  )
}
