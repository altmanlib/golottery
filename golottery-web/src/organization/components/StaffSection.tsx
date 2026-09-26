import { Alert, Button, Code, CopyButton, Group, Image, Modal, Stack, Table, Text } from '@mantine/core'
import { useQueryClient } from '@tanstack/react-query'
import QRCode from 'qrcode'
import { useEffect, useState } from 'react'
import type { Event, StaffInvite, StaffRole } from '#/api-gen/types.gen'
import { TableSkeleton } from '#/components/TableSkeleton'
import { showToast } from '#/lib/message'
import { formatShanghai } from '#/organization/events'
import { useCreateInvite, useRemoveStaff, useStaff } from '#/organization/hooks/useEvents'
import { organizationKeys } from '#/organization/queryKeys'

const ROLE_LABELS: Record<StaffRole, string> = { staff: '工作人员', admin: '现场管理员' }

/** Shows a fresh invite once; the code is not kept anywhere else. */
function InviteModal({ invite, onClose }: { invite: StaffInvite; onClose: () => void }) {
  const url = window.location.origin + window.location.pathname.replace(/\/$/, '') + invite.path
  const [qr, setQr] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    QRCode.toDataURL(url, { margin: 1, width: 512 }).then((data) => alive && setQr(data))
    return () => {
      alive = false
    }
  }, [url])

  return (
    <Modal opened onClose={onClose} title={`邀请${ROLE_LABELS[invite.role]}`} centered>
      <Stack gap={12}>
        <Text size="sm" c="dimmed">
          链接只能使用一次，{formatShanghai(invite.expires_at)} 前有效。关闭后无法再次查看，请先发给对方。
        </Text>
        {qr && <Image src={qr} alt="" w={180} h={180} mx="auto" />}
        <Code block>{url}</Code>
        <Group justify="flex-end" gap={8}>
          <CopyButton value={url}>
            {({ copied, copy }) => (
              <Button variant="light" onClick={copy}>
                {copied ? '已复制' : '复制链接'}
              </Button>
            )}
          </CopyButton>
          <Button onClick={onClose}>完成</Button>
        </Group>
      </Stack>
    </Modal>
  )
}

export function StaffSection({ event }: { event: Event }) {
  const staff = useStaff(event.id)
  const create = useCreateInvite(event.id)
  const remove = useRemoveStaff(event.id)
  const queryClient = useQueryClient()
  const [invite, setInvite] = useState<StaffInvite | null>(null)
  const closed = event.status === 'closed'

  const issue = (role: StaffRole) =>
    create.mutate(role, {
      onSuccess: (data) => {
        setInvite(data)
        create.reset()
      },
    })
  const closeInvite = () => {
    setInvite(null)
    void queryClient.invalidateQueries({ queryKey: organizationKeys.staff(event.id) })
  }

  return (
    <Stack gap={8}>
      <Text size="sm" c="dimmed">
        工作人员在手机上处理协助请求、代签；现场管理员还能切换签到方式、调整签到范围。
      </Text>
      <Group gap={8}>
        <Button size="xs" variant="light" disabled={closed} loading={create.isPending && create.variables === 'staff'} onClick={() => issue('staff')}>
          邀请工作人员
        </Button>
        <Button size="xs" variant="light" disabled={closed} loading={create.isPending && create.variables === 'admin'} onClick={() => issue('admin')}>
          邀请现场管理员
        </Button>
      </Group>
      {create.error && (
        <Alert color="red" variant="light">
          {create.error.message}
        </Alert>
      )}
      {staff.isPending ? (
        <TableSkeleton rows={2} />
      ) : staff.data?.length ? (
        <Table verticalSpacing={8} horizontalSpacing={12}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>角色</Table.Th>
              <Table.Th>名单姓名</Table.Th>
              <Table.Th>加入时间</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {staff.data.map((m) => (
              <Table.Tr key={m.id}>
                <Table.Td>{ROLE_LABELS[m.role]}</Table.Td>
                <Table.Td>{m.attendee_name ?? '—'}</Table.Td>
                <Table.Td ff="monospace">{formatShanghai(m.created_at)}</Table.Td>
                <Table.Td ta="right">
                  <Button
                    size="compact-xs"
                    variant="subtle"
                    color="red"
                    loading={remove.isPending && remove.variables === m.id}
                    onClick={() => remove.mutate(m.id, { onError: (error) => showToast(error.message, 'error') })}>
                    撤销
                  </Button>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      ) : (
        <Text size="sm" c="dimmed">
          还没有工作人员
        </Text>
      )}
      {invite && <InviteModal invite={invite} onClose={closeInvite} />}
    </Stack>
  )
}
