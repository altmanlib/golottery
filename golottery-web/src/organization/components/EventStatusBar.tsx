import { Alert, Button, Group, Modal, Stack, Text, Tooltip } from '@mantine/core'
import { useState } from 'react'
import type { Event, EventStatus } from '#/api-gen/types.gen'
import { showToast } from '#/lib/message'
import { readyBlocker, readyCostHint } from '#/organization/events'
import { useUpdateEvent } from '#/organization/hooks/useEvents'

type Action = { to: EventStatus; label: string; title: string; body: string; color?: string }

function actionsFor(event: Event, credits: number | undefined): Action[] {
  switch (event.status) {
    case 'draft':
      return [{ to: 'ready', label: '设为就绪', title: '设为就绪', body: `${readyCostHint(event, credits)}。就绪后宾客即可在签到时间窗内签到。` }]
    case 'ready':
      return [
        { to: 'draft', label: '退回草稿', title: '退回草稿', body: '宾客将无法签到，已扣场次不退还。' },
        { to: 'closed', label: '结束活动', title: '结束活动', body: '结束后配置与名单都不能再修改，此操作不可撤销。', color: 'red' },
      ]
    default:
      return []
  }
}

export function EventStatusBar({ event, credits }: { event: Event; credits: number | undefined }) {
  const update = useUpdateEvent(event.id)
  const [pending, setPending] = useState<Action | null>(null)
  const blocker = readyBlocker(event, credits)

  const dismiss = () => {
    setPending(null)
    update.reset()
  }

  const confirm = () =>
    pending &&
    update.mutate(
      { status: pending.to },
      {
        onSuccess: () => {
          setPending(null)
          showToast(`已${pending.label}`, 'success')
        },
      },
    )

  return (
    <Group gap={8}>
      {actionsFor(event, credits).map((action) => {
        const disabled = action.to === 'ready' && blocker !== null
        const button = (
          <Button
            key={action.to}
            size="xs"
            variant={action.to === 'ready' ? 'filled' : 'light'}
            color={action.color}
            disabled={disabled}
            onClick={() => setPending(action)}>
            {action.label}
          </Button>
        )
        return disabled ? (
          <Tooltip key={action.to} label={blocker}>
            <span>{button}</span>
          </Tooltip>
        ) : (
          button
        )
      })}
      <Modal opened={pending !== null} onClose={dismiss} title={pending?.title} centered>
        <Stack gap={16}>
          <Text size="sm">{pending?.body}</Text>
          {update.error && (
            <Alert color="red" variant="light">
              {update.error.message}
            </Alert>
          )}
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={dismiss}>
              取消
            </Button>
            <Button color={pending?.color} loading={update.isPending} onClick={confirm}>
              确定
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Group>
  )
}
