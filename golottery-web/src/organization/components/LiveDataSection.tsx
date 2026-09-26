import { Alert, Button, Group, Modal, Stack, Text, TextInput } from '@mantine/core'
import { IconDownload } from '@tabler/icons-react'
import { useState } from 'react'
import type { Event } from '#/api-gen/types.gen'
import { showToast } from '#/lib/message'
import { useExportAttempts, useResetLiveData } from '#/organization/hooks/useEvents'

/** Clears a trial run; the event name must be typed back to confirm. */
function ResetModal({ event, onClose }: { event: Event; onClose: () => void }) {
  const reset = useResetLiveData(event.id)
  const [name, setName] = useState('')
  const submit = () =>
    reset.mutate(name, {
      onSuccess: (r) => {
        onClose()
        showToast(`已清除：${r.unbound} 人的绑定与签到、${r.attempts} 条签到记录、${r.requests} 条协助请求`, 'success')
      },
    })

  return (
    <Modal opened onClose={onClose} title="重置现场数据" centered>
      <Stack gap={12}>
        <Text size="sm">清除试签到留下的身份绑定、签到结果、签到记录和协助请求，名单、奖项和工作人员保留。只能在签到开始前操作，不可撤销。</Text>
        {reset.error && (
          <Alert color="red" variant="light">
            {reset.error.message}
          </Alert>
        )}
        <TextInput label={`输入活动名称「${event.name}」确认`} value={name} onChange={(e) => setName(e.currentTarget.value)} data-autofocus />
        <Group justify="flex-end" gap={8}>
          <Button variant="default" onClick={onClose}>
            取消
          </Button>
          <Button color="red" disabled={name.trim() !== event.name} loading={reset.isPending} onClick={submit}>
            重置
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}

export function LiveDataSection({ event }: { event: Event }) {
  const exporter = useExportAttempts(event.id, event.name)
  const [resetting, setResetting] = useState(false)
  const started = Boolean(event.checkin_start && new Date(event.checkin_start).getTime() <= Date.now())

  return (
    <Stack gap={8}>
      <Text size="sm" c="dimmed">
        签到明细包含每一次点击签到的时间、坐标和结果，便于核查争议。
      </Text>
      <Group gap={8}>
        <Button
          size="xs"
          variant="light"
          leftSection={<IconDownload size={16} />}
          loading={exporter.isPending}
          onClick={() => exporter.mutate(undefined, { onError: (error) => showToast(error.message, 'error') })}>
          导出签到明细
        </Button>
        <Button size="xs" variant="subtle" color="red" disabled={started || event.status === 'closed'} onClick={() => setResetting(true)}>
          重置现场数据
        </Button>
      </Group>
      {started && event.status !== 'closed' && (
        <Text size="xs" c="dimmed">
          签到已开始，不能再重置
        </Text>
      )}
      {resetting && <ResetModal event={event} onClose={() => setResetting(false)} />}
    </Stack>
  )
}
