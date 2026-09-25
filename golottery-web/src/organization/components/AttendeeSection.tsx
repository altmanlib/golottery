import { Alert, Button, FileButton, Group, List, Modal, Pagination, Stack, Table, Text, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import { IconDownload, IconPlus, IconUpload } from '@tabler/icons-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { Attendee, Event } from '#/api-gen/types.gen'
import { TableSkeleton } from '#/components/TableSkeleton'
import { showToast } from '#/lib/message'
import { parsePage } from '#/lib/paging'
import { type AttendeeValues, attendeeErrorMessage, emptyAttendee, importFailure, PAGE_SIZE, validateAttendee } from '#/organization/events'
import { useAddAttendee, useAttendees, useDeleteAttendee, useExportAttendees, useImportAttendees, useUpdateAttendee } from '#/organization/hooks/useEvents'

function AttendeeModal({ event, attendee, onClose }: { event: Event; attendee: Attendee | null; onClose: () => void }) {
  const add = useAddAttendee(event.id)
  const update = useUpdateAttendee(event.id)
  const form = useForm<AttendeeValues>({
    initialValues: attendee ? { name: attendee.name, dept: attendee.dept, phone: attendee.phone_last4 } : emptyAttendee,
    validate: validateAttendee,
  })
  const failure = attendeeErrorMessage(add.error ?? update.error)
  const submit = (values: AttendeeValues) => {
    const body = { name: values.name.trim(), dept: values.dept.trim(), phone: values.phone }
    const done = {
      onSuccess: () => {
        onClose()
        showToast('名单已保存', 'success')
      },
    }
    if (attendee) update.mutate({ attendeeId: attendee.id, body }, done)
    else add.mutate(body, done)
  }

  return (
    <Modal opened onClose={onClose} title={attendee ? '编辑人员' : '添加人员'} centered>
      <form onSubmit={form.onSubmit(submit)}>
        <Stack gap={12}>
          {failure && (
            <Alert color="red" variant="light">
              {failure}
            </Alert>
          )}
          <TextInput label="姓名" withAsterisk maxLength={100} data-autofocus {...form.getInputProps('name')} />
          <TextInput label="部门" maxLength={100} {...form.getInputProps('dept')} />
          <TextInput label="手机号" withAsterisk description="只保存后四位" {...form.getInputProps('phone')} />
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

export function AttendeeSection({ event }: { event: Event }) {
  const [params, setParams] = useSearchParams()
  const page = parsePage(params.get('page'))
  const attendees = useAttendees(event.id, page)
  const remove = useDeleteAttendee(event.id)
  const importer = useImportAttendees(event.id)
  const exporter = useExportAttendees(event.id, event.name)
  const [editing, setEditing] = useState<Attendee | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Attendee | null>(null)
  const failure = importFailure(importer.error)
  const closed = event.status === 'closed'
  const total = attendees.data?.total ?? 0

  const upload = (file: File | null) =>
    file &&
    importer.mutate(file, {
      onSuccess: ({ imported }) => showToast(`已导入 ${imported} 人`, 'success'),
    })

  return (
    <Stack gap={8}>
      <Group justify="space-between">
        <Text fw={500} size="sm">
          名单 {event.attendee_count} / {event.max_attendees}
        </Text>
        <Group gap={8}>
          <FileButton onChange={upload} accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet">
            {(props) => (
              <Button {...props} size="xs" variant="light" leftSection={<IconUpload size={16} />} disabled={closed} loading={importer.isPending}>
                导入 Excel
              </Button>
            )}
          </FileButton>
          <Button size="xs" variant="light" leftSection={<IconPlus size={16} />} disabled={closed} onClick={() => setEditing('new')}>
            添加人员
          </Button>
          <Button size="xs" variant="subtle" leftSection={<IconDownload size={16} />} loading={exporter.isPending} onClick={() => exporter.mutate()}>
            导出
          </Button>
        </Group>
      </Group>
      <Text size="xs" c="dimmed">
        Excel 第一行为表头：姓名、部门、手机号。多次导入只追加，任一行有误整份不导入。
      </Text>
      {failure && (
        <Alert color="red" variant="light" withCloseButton onClose={() => importer.reset()}>
          <Stack gap={4}>
            <Text size="sm">{failure.message}</Text>
            {failure.rows.length > 0 && (
              <List size="sm" spacing={2}>
                {failure.rows.slice(0, 20).map((r) => (
                  <List.Item key={r.row}>
                    第 {r.row} 行：{r.reason}
                  </List.Item>
                ))}
                {failure.rows.length > 20 && <List.Item>…另有 {failure.rows.length - 20} 行</List.Item>}
              </List>
            )}
          </Stack>
        </Alert>
      )}
      {attendees.isPending ? (
        <TableSkeleton rows={4} />
      ) : total === 0 ? (
        <Text size="sm" c="dimmed">
          名单为空。导入 Excel 或逐个添加后才能设为就绪。
        </Text>
      ) : (
        <>
          <Table verticalSpacing={8} horizontalSpacing={12}>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>姓名</Table.Th>
                <Table.Th>部门</Table.Th>
                <Table.Th>手机后四位</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {attendees.data?.items.map((a) => (
                <Table.Tr key={a.id}>
                  <Table.Td>{a.name}</Table.Td>
                  <Table.Td>{a.dept || '—'}</Table.Td>
                  <Table.Td ff="monospace">{a.phone_last4}</Table.Td>
                  <Table.Td>
                    <Group gap={4} justify="flex-end">
                      <Button size="compact-xs" variant="subtle" disabled={closed} onClick={() => setEditing(a)}>
                        编辑
                      </Button>
                      <Button size="compact-xs" variant="subtle" color="red" disabled={closed} onClick={() => setDeleting(a)}>
                        删除
                      </Button>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
          {total > PAGE_SIZE && (
            <Group justify="flex-end">
              <Pagination
                size="sm"
                total={Math.ceil(total / PAGE_SIZE)}
                value={page}
                onChange={(next) => setParams(next === 1 ? {} : { page: String(next) })}
              />
            </Group>
          )}
        </>
      )}
      {editing && <AttendeeModal event={event} attendee={editing === 'new' ? null : editing} onClose={() => setEditing(null)} />}
      <Modal opened={deleting !== null} onClose={() => setDeleting(null)} title="删除人员" centered>
        <Stack gap={16}>
          <Text size="sm">确定从名单中删除 {deleting?.name}？</Text>
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={() => setDeleting(null)}>
              取消
            </Button>
            <Button
              color="red"
              loading={remove.isPending}
              onClick={() =>
                deleting &&
                remove.mutate(deleting.id, {
                  onSuccess: () => setDeleting(null),
                  onError: (error) => showToast(error.message, 'error'),
                })
              }>
              删除
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  )
}
