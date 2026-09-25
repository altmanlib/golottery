import { Alert, Button, Group, NumberInput, Stack, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import { showToast } from '#/lib/message'
import { useAdjustCredits } from '#/platform/hooks/useOrgMutations'
import { type AdjustCreditsValues, adjustCreditsErrorMessage, emptyAdjustCredits, toAdjustCreditsBody, validateAdjustCredits } from '#/platform/orgs'

export function AdjustCreditsForm({ orgId }: { orgId: string }) {
  const form = useForm<AdjustCreditsValues>({ initialValues: emptyAdjustCredits, validate: validateAdjustCredits })
  const adjust = useAdjustCredits(orgId)
  const failure = adjustCreditsErrorMessage(adjust.error)

  return (
    <form
      onSubmit={form.onSubmit((values) =>
        adjust.mutate(toAdjustCreditsBody(values), {
          onSuccess: (entry) => {
            form.reset()
            showToast(`场次已调整，剩余 ${entry.balance_after}`, 'success')
          },
        }),
      )}>
      <Stack gap={8}>
        {failure && (
          <Alert color="red" variant="light">
            {failure}
          </Alert>
        )}
        <Group gap={8} align="flex-start">
          <NumberInput w={140} label="场次变动" placeholder="如 5 或 -1" allowDecimal={false} {...form.getInputProps('delta')} />
          <TextInput flex={1} label="原因" maxLength={200} {...form.getInputProps('reason')} />
        </Group>
        <Group justify="flex-end">
          <Button type="submit" size="xs" loading={adjust.isPending}>
            调整场次
          </Button>
        </Group>
      </Stack>
    </form>
  )
}
