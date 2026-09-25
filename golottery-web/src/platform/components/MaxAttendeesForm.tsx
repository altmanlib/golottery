import { Button, Group, NumberInput } from '@mantine/core'
import { useState } from 'react'
import { showToast } from '#/lib/message'
import { useSetMaxAttendees } from '#/platform/hooks/useOrgMutations'
import { ATTENDEE_TIERS } from '#/platform/orgs'

const tierHint = ATTENDEE_TIERS.map((tier) => `${tier.label} ${tier.value}`).join(' · ')

export function MaxAttendeesForm({ orgId, current }: { orgId: string; current: number }) {
  const [value, setValue] = useState<number | string>(current)
  const save = useSetMaxAttendees(orgId)
  const next = Number(value)
  const valid = Number.isInteger(next) && next > 0

  return (
    <Group gap={8} align="flex-end">
      <NumberInput
        w={200}
        label="人数上限"
        description={`只影响之后创建的活动 · ${tierHint}`}
        min={1}
        allowDecimal={false}
        allowNegative={false}
        value={value}
        onChange={setValue}
        error={save.error instanceof Error ? save.error.message : undefined}
      />
      <Button
        size="xs"
        variant="default"
        disabled={!valid || next === current}
        loading={save.isPending}
        onClick={() => save.mutate(next, { onSuccess: () => showToast('人数上限已更新', 'success') })}>
        保存
      </Button>
    </Group>
  )
}
