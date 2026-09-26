import { Alert, Button, Group, NumberInput, Paper, SegmentedControl, Stack, Text } from '@mantine/core'
import { IconCurrentLocation } from '@tabler/icons-react'
import { useState } from 'react'
import type { CheckinMode, GuestStatus } from '#/api-gen/types.gen'
import { currentFix, errorMessage } from '#/guest/guest'
import { useStaffSettings } from '#/guest/hooks/useStaff'
import { showToast } from '#/lib/message'
import { DIRECT_MODE_WARNING, MODE_LABELS } from '#/organization/events'

/** On-site admins switch the mode or move the fence to where they stand. */
export function SettingsPanel({ publicId, status }: { publicId: string; status: GuestStatus }) {
  const settings = useStaffSettings(publicId)
  const [radius, setRadius] = useState<number | string>('')
  const [locating, setLocating] = useState(false)
  const [fixError, setFixError] = useState<unknown>(null)
  const saved = { onSuccess: () => showToast('已保存', 'success') }

  const moveHere = async () => {
    setFixError(null)
    setLocating(true)
    try {
      const fix = await currentFix()
      settings.mutate({ center_lat: fix.lat, center_lng: fix.lng, coord_type: 'wgs84' }, saved)
    } catch (error) {
      setFixError(error)
    } finally {
      setLocating(false)
    }
  }

  return (
    <Stack gap={12}>
      {Boolean(settings.error || fixError) && (
        <Alert color="red" variant="light">
          {errorMessage(settings.error ?? fixError)}
        </Alert>
      )}
      <Paper withBorder radius="md" p={12}>
        <Stack gap={8}>
          <Text fw={500} size="sm">
            签到方式
          </Text>
          <SegmentedControl
            data={(['geo', 'direct'] as CheckinMode[]).map((m) => ({ value: m, label: MODE_LABELS[m] }))}
            value={status.event.checkin_mode}
            disabled={settings.isPending}
            onChange={(mode) => settings.mutate({ checkin_mode: mode as CheckinMode }, saved)}
          />
          {status.event.checkin_mode === 'direct' && (
            <Text size="xs" c="dimmed">
              {DIRECT_MODE_WARNING}
            </Text>
          )}
        </Stack>
      </Paper>
      <Paper withBorder radius="md" p={12}>
        <Stack gap={8}>
          <Text fw={500} size="sm">
            签到范围
          </Text>
          <Text size="xs" c="dimmed">
            定位偏差大时，可站在会场中心重新设定圆心，或放大半径
          </Text>
          <Button variant="light" leftSection={<IconCurrentLocation size={16} />} loading={locating} onClick={moveHere}>
            以我当前位置为圆心
          </Button>
          <Group gap={8} align="flex-end" wrap="nowrap">
            <NumberInput label="半径（100～1000 米）" min={100} max={1000} allowDecimal={false} value={radius} onChange={setRadius} flex={1} />
            <Button
              variant="default"
              disabled={radius === ''}
              loading={settings.isPending && settings.variables?.radius_m !== undefined}
              onClick={() => settings.mutate({ radius_m: Number(radius) }, saved)}>
              保存半径
            </Button>
          </Group>
        </Stack>
      </Paper>
    </Stack>
  )
}
