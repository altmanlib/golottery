import { Alert, Button, Paper, Stack, Text, Textarea, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import type { GuestStatus } from '#/api-gen/types.gen'
import { errorMessage, validateBind } from '#/guest/guest'
import { useHelpRequest } from '#/guest/hooks/useGuest'

type Values = { name: string; phoneLast4: string; reason: string }

/** Asks on-site staff to check the guest in by hand. Unbound guests say who they are. */
export function HelpCard({ publicId, status, onCancel }: { publicId: string; status: GuestStatus; onCancel: () => void }) {
  const help = useHelpRequest(publicId)
  const bound = Boolean(status.attendee)
  const form = useForm<Values>({
    initialValues: { name: '', phoneLast4: '', reason: '' },
    validate: (values) => (bound ? {} : validateBind(values)),
  })

  if (status.request?.status === 'pending') {
    return (
      <Alert color="blue" variant="light" title="已提交协助请求">
        请到签到台找工作人员确认，处理后本页会自动更新
      </Alert>
    )
  }

  const submit = (values: Values) =>
    help.mutate(bound ? { reason: values.reason.trim() } : { name: values.name.trim(), phone_last4: values.phoneLast4, reason: values.reason.trim() })

  return (
    <Paper withBorder radius="md" p={16}>
      <form onSubmit={form.onSubmit(submit)}>
        <Stack gap={12}>
          <Stack gap={4}>
            <Text fw={500}>请工作人员协助</Text>
            <Text size="sm" c="dimmed">
              提交后到签到台找工作人员，由工作人员确认签到
            </Text>
          </Stack>
          {status.request?.status === 'rejected' && (
            <Alert color="yellow" variant="light">
              上一次请求未通过，可以补充说明后重新提交
            </Alert>
          )}
          {help.error && (
            <Alert color="red" variant="light">
              {errorMessage(help.error)}
            </Alert>
          )}
          {!bound && (
            <>
              <TextInput label="姓名" maxLength={50} {...form.getInputProps('name')} />
              <TextInput label="手机号后四位" inputMode="numeric" maxLength={4} {...form.getInputProps('phoneLast4')} />
            </>
          )}
          <Textarea label="情况说明" placeholder="例如：名单上找不到我、定位不准" maxLength={200} autosize minRows={2} {...form.getInputProps('reason')} />
          <Button type="submit" loading={help.isPending}>
            提交
          </Button>
          <Button variant="subtle" onClick={onCancel}>
            取消
          </Button>
        </Stack>
      </form>
    </Paper>
  )
}
