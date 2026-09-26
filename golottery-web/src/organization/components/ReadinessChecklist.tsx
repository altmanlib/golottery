import { Anchor, Center, Group, Stack, Text } from '@mantine/core'
import { IconCheck } from '@tabler/icons-react'
import type { Event } from '#/api-gen/types.gen'
import { Meter } from '#/components/Meter'
import { type DetailTab, readinessSteps } from '#/organization/events'
import classes from './ReadinessChecklist.module.css'

export function ReadinessChecklist({ event, onOpen }: { event: Event; onOpen: (tab: DetailTab) => void }) {
  const steps = readinessSteps(event)
  const required = steps.filter((s) => s.required)
  const done = required.filter((s) => s.done).length

  return (
    <Stack gap={12}>
      <Group justify="space-between">
        <Text size="sm" c="dimmed">
          {event.status === 'draft' ? `必填 ${done} / ${required.length} 项已完成，全部完成后可设为就绪` : '活动已设为就绪，修改仍会立即生效'}
        </Text>
      </Group>
      <Meter value={done} max={required.length} label="就绪进度" />
      <Stack gap={0}>
        {steps.map((step) => (
          <Group key={step.label} className={classes.row} gap={12} wrap="nowrap">
            <Center className={classes.mark} data-state={step.done ? 'done' : step.required ? 'todo' : 'optional'}>
              {step.done && <IconCheck size={12} stroke={3} />}
            </Center>
            <Text size="md" flex={1}>
              {step.label}
              {!step.required && (
                <Text span size="xs" c="dimmed" ml={8}>
                  可选
                </Text>
              )}
            </Text>
            <Text size="sm" className={classes.detail}>
              {step.detail}
            </Text>
            <Anchor component="button" type="button" size="sm" fw={600} w={56} ta="right" onClick={() => onOpen(step.tab)}>
              {step.done ? '查看' : '去设置'}
            </Anchor>
          </Group>
        ))}
      </Stack>
    </Stack>
  )
}
