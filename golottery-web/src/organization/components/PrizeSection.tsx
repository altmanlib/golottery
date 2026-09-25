import { Alert, Button, Group, Modal, NumberInput, Stack, Table, Text, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import { IconPlus } from '@tabler/icons-react'
import { useState } from 'react'
import type { Event, Prize } from '#/api-gen/types.gen'
import { TableSkeleton } from '#/components/TableSkeleton'
import { showToast } from '#/lib/message'
import { type PrizeValues, prizeErrorMessage, toPrizeInput, validatePrize } from '#/organization/events'
import { useAddPrize, useDeletePrize, usePrizes, useUpdatePrize } from '#/organization/hooks/useEvents'

function PrizeModal({ event, prize, nextSort, onClose }: { event: Event; prize: Prize | null; nextSort: number; onClose: () => void }) {
  const add = useAddPrize(event.id)
  const update = useUpdatePrize(event.id)
  const form = useForm<PrizeValues>({
    initialValues: prize
      ? { name: prize.name, gift: prize.gift, quota: prize.quota, sortNo: prize.sort_no }
      : { name: '', gift: '', quota: 1, sortNo: nextSort },
    validate: validatePrize,
  })
  const failure = prizeErrorMessage(add.error ?? update.error)
  const submit = (values: PrizeValues) => {
    const body = toPrizeInput(values)
    const done = {
      onSuccess: () => {
        onClose()
        showToast('奖项已保存', 'success')
      },
    }
    if (prize) update.mutate({ prizeId: prize.id, body }, done)
    else add.mutate(body, done)
  }

  return (
    <Modal opened onClose={onClose} title={prize ? '编辑奖项' : '添加奖项'} centered>
      <form onSubmit={form.onSubmit(submit)}>
        <Stack gap={12}>
          {failure && (
            <Alert color="red" variant="light">
              {failure}
            </Alert>
          )}
          <TextInput label="奖项名称" withAsterisk maxLength={100} data-autofocus {...form.getInputProps('name')} />
          <TextInput label="奖品" maxLength={200} {...form.getInputProps('gift')} />
          <Group grow align="flex-start">
            <NumberInput label="名额" min={1} allowDecimal={false} {...form.getInputProps('quota')} />
            <NumberInput label="抽奖顺序" allowDecimal={false} description="数字小的先抽" {...form.getInputProps('sortNo')} />
          </Group>
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" loading={add.isPending || update.isPending}>
              保存
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}

export function PrizeSection({ event }: { event: Event }) {
  const prizes = usePrizes(event.id)
  const remove = useDeletePrize(event.id)
  const [editing, setEditing] = useState<Prize | 'new' | null>(null)
  const closed = event.status === 'closed'
  const nextSort = Math.max(0, ...(prizes.data ?? []).map((p) => p.sort_no)) + 1

  return (
    <Stack gap={8}>
      <Group justify="space-between">
        <Text fw={500} size="sm">
          奖项
        </Text>
        <Button size="xs" variant="light" leftSection={<IconPlus size={16} />} disabled={closed} onClick={() => setEditing('new')}>
          添加奖项
        </Button>
      </Group>
      {prizes.isPending ? (
        <TableSkeleton rows={2} />
      ) : prizes.data?.length ? (
        <Table verticalSpacing={8} horizontalSpacing={12}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th ta="right">顺序</Table.Th>
              <Table.Th>奖项</Table.Th>
              <Table.Th>奖品</Table.Th>
              <Table.Th ta="right">名额</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {prizes.data.map((prize) => (
              <Table.Tr key={prize.id}>
                <Table.Td ta="right" ff="monospace">
                  {prize.sort_no}
                </Table.Td>
                <Table.Td>{prize.name}</Table.Td>
                <Table.Td>{prize.gift || '—'}</Table.Td>
                <Table.Td ta="right" ff="monospace">
                  {prize.quota}
                </Table.Td>
                <Table.Td>
                  <Group gap={4} justify="flex-end">
                    <Button size="compact-xs" variant="subtle" disabled={closed} onClick={() => setEditing(prize)}>
                      编辑
                    </Button>
                    <Button
                      size="compact-xs"
                      variant="subtle"
                      color="red"
                      disabled={closed}
                      onClick={() => remove.mutate(prize.id, { onError: (error) => showToast(error.message, 'error') })}>
                      删除
                    </Button>
                  </Group>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      ) : (
        <Text size="sm" c="dimmed">
          还没有奖项
        </Text>
      )}
      {editing && <PrizeModal event={event} prize={editing === 'new' ? null : editing} nextSort={nextSort} onClose={() => setEditing(null)} />}
    </Stack>
  )
}
