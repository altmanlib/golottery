import { Alert, Button, Group, NumberInput, SegmentedControl, SimpleGrid, Stack, Switch, Text, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import type { CheckinMode, Event } from '#/api-gen/types.gen'
import { showToast } from '#/lib/message'
import { DIRECT_MODE_WARNING, MODE_LABELS, type SettingsValues, settingsFromEvent, settingsPatch, validateSettings } from '#/organization/events'
import { useUpdateEvent } from '#/organization/hooks/useEvents'

/** Settings form; remount it with key={updated_at} so it shows the saved values. */
export function CheckinSettingsForm({ event }: { event: Event }) {
  const form = useForm<SettingsValues>({ initialValues: settingsFromEvent(event), validate: validateSettings })
  const update = useUpdateEvent(event.id)
  const closed = event.status === 'closed'
  const patch = settingsPatch(event, form.values)
  const dirty = Object.keys(patch).length > 0

  return (
    <form onSubmit={form.onSubmit((values) => update.mutate(settingsPatch(event, values), { onSuccess: () => showToast('签到设置已保存', 'success') }))}>
      <Stack gap={12}>
        {update.error && (
          <Alert color="red" variant="light">
            {update.error.message}
          </Alert>
        )}
        <TextInput label="活动名称" maxLength={100} disabled={closed} {...form.getInputProps('name')} />
        <Stack gap={4}>
          <Text size="sm" fw={500}>
            签到方式
          </Text>
          <SegmentedControl
            disabled={closed}
            data={(['geo', 'direct'] as CheckinMode[]).map((mode) => ({ value: mode, label: MODE_LABELS[mode] }))}
            {...form.getInputProps('checkinMode')}
          />
        </Stack>
        {form.values.checkinMode === 'direct' ? (
          <Alert color="yellow" variant="light">
            {DIRECT_MODE_WARNING}
          </Alert>
        ) : (
          <SimpleGrid cols={{ base: 1, sm: 3 }} spacing={12} verticalSpacing={12}>
            <NumberInput label="中心纬度（GCJ-02）" decimalScale={6} disabled={closed} {...form.getInputProps('centerLat')} />
            <NumberInput label="中心经度（GCJ-02）" decimalScale={6} disabled={closed} {...form.getInputProps('centerLng')} />
            <NumberInput label="半径（米）" min={100} max={1000} allowDecimal={false} disabled={closed} {...form.getInputProps('radiusM')} />
          </SimpleGrid>
        )}
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing={12} verticalSpacing={12}>
          <TextInput type="datetime-local" label="签到开始（北京时间）" disabled={closed} {...form.getInputProps('checkinStart')} />
          <TextInput type="datetime-local" label="签到结束（北京时间）" disabled={closed} {...form.getInputProps('checkinEnd')} />
        </SimpleGrid>
        <Switch label="允许同一人多次中奖" disabled={closed} {...form.getInputProps('allowMultiWin', { type: 'checkbox' })} />
        <Group justify="flex-end">
          <Button type="submit" size="xs" disabled={closed || !dirty} loading={update.isPending}>
            保存设置
          </Button>
        </Group>
      </Stack>
    </form>
  )
}
