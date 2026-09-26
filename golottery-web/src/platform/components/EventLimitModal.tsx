import { Alert, Button, Group, Modal, NumberInput, Stack, Text } from '@mantine/core'
import { useState } from 'react'
import type { Event } from '#/api-gen/types.gen'
import { showToast } from '#/lib/message'
import { eventMaxAttendeesErrorMessage } from '#/platform/events'
import { useSetEventMaxAttendees } from '#/platform/hooks/useOrgEvents'

type Props = {
  orgId: string
  event: Event | null
  onClose: () => void
}

export function EventLimitModal({ orgId, event, onClose }: Props) {
  const [value, setValue] = useState<number | string>(event?.max_attendees ?? '')
  const save = useSetEventMaxAttendees(orgId)
  const failure = eventMaxAttendeesErrorMessage(save.error)
  const next = Number(value)
  const floor = event?.attendee_count ?? 1
  const valid = event !== null && Number.isInteger(next) && next >= floor

  // Remount via key={event?.id} so the initial value tracks the selected event.
  const close = () => {
    save.reset()
    onClose()
  }

  return (
    <Modal opened={event !== null} onClose={close} title="调整活动人数上限" centered>
      {event && (
        <Stack gap={12}>
          {failure && (
            <Alert color="red" variant="light">
              {failure}
            </Alert>
          )}
          <Text size="sm">
            {event.name} · 当前名单 {event.attendee_count} 人 · 上限不能低于名单人数
          </Text>
          <NumberInput label="人数上限" min={floor} allowDecimal={false} allowNegative={false} value={value} onChange={setValue} data-autofocus />
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={close}>
              取消
            </Button>
            <Button
              disabled={!valid || next === event.max_attendees}
              loading={save.isPending}
              onClick={() =>
                save.mutate(
                  { eventId: event.id, maxAttendees: next },
                  {
                    onSuccess: () => {
                      close()
                      showToast('活动人数上限已更新', 'success')
                    },
                  },
                )
              }>
              保存
            </Button>
          </Group>
        </Stack>
      )}
    </Modal>
  )
}
