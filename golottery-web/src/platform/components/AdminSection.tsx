import { Badge, Button, Group, Modal, Paper, Stack, Table, Text } from '@mantine/core'
import { IconPlus } from '@tabler/icons-react'
import { useReducer, useState } from 'react'
import type { OrgUser } from '#/api-gen/types.gen'
import { TableSkeleton } from '#/components/TableSkeleton'
import { formatDateTime } from '#/lib/format'
import { showToast } from '#/lib/message'
import { revealReducer } from '#/platform/admins'
import { CreateAdminModal } from '#/platform/components/CreateAdminModal'
import { PasswordRevealModal } from '#/platform/components/PasswordRevealModal'
import { useOrgUsers, useResetOrgUserPassword, useSetOrgUserStatus } from '#/platform/hooks/useOrgUsers'

export function AdminSection({ orgId }: { orgId: string }) {
  const users = useOrgUsers(orgId)
  const setStatus = useSetOrgUserStatus(orgId)
  const reset = useResetOrgUserPassword(orgId)
  const [creating, setCreating] = useState(false)
  const [resetting, setResetting] = useState<OrgUser | null>(null)
  const [reveal, dispatch] = useReducer(revealReducer, null)

  const confirmReset = (user: OrgUser) =>
    reset.mutate(user.id, {
      onSuccess: ({ password }) => {
        setResetting(null)
        reset.reset()
        dispatch({ type: 'show', reveal: { title: '口令已重置', email: user.email, password } })
      },
      onError: (error) => showToast(error.message, 'error'),
    })

  const toggle = (user: OrgUser) =>
    setStatus.mutate(
      { userId: user.id, active: user.status === 'disabled' },
      {
        onSuccess: () => showToast(user.status === 'disabled' ? '管理员已启用' : '管理员已停用，其登录会话已失效', 'success'),
        onError: (error) => showToast(error.message, 'error'),
      },
    )

  return (
    <Stack gap={8}>
      <Group justify="space-between">
        <Text fw={500} size="sm">
          管理员
        </Text>
        <Button size="xs" variant="light" leftSection={<IconPlus size={16} />} onClick={() => setCreating(true)}>
          添加管理员
        </Button>
      </Group>
      <Paper withBorder radius="md">
        {users.isPending ? (
          <TableSkeleton rows={2} />
        ) : users.isError ? (
          <Text size="sm" c="red" p={12}>
            {users.error.message}
          </Text>
        ) : users.data.length === 0 ? (
          <Text size="sm" c="dimmed" p={12}>
            还没有管理员。添加后把临时口令交给对方登录 /organization
          </Text>
        ) : (
          <Table verticalSpacing={8} horizontalSpacing={12}>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>姓名</Table.Th>
                <Table.Th>邮箱</Table.Th>
                <Table.Th>状态</Table.Th>
                <Table.Th>创建时间</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {users.data.map((user) => (
                <Table.Tr key={user.id}>
                  <Table.Td>{user.name}</Table.Td>
                  <Table.Td>{user.email}</Table.Td>
                  <Table.Td>
                    <Badge variant="light" color={user.status === 'active' ? 'brand' : 'gray'}>
                      {user.status === 'active' ? '启用' : '停用'}
                    </Badge>
                  </Table.Td>
                  <Table.Td>{formatDateTime(user.created_at)}</Table.Td>
                  <Table.Td>
                    <Group gap={4} justify="flex-end">
                      <Button size="compact-xs" variant="subtle" onClick={() => setResetting(user)}>
                        重置口令
                      </Button>
                      <Button size="compact-xs" variant="subtle" color={user.status === 'active' ? 'red' : 'brand'} onClick={() => toggle(user)}>
                        {user.status === 'active' ? '停用' : '启用'}
                      </Button>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Paper>

      <CreateAdminModal orgId={orgId} opened={creating} onClose={() => setCreating(false)} onCreated={(next) => dispatch({ type: 'show', reveal: next })} />
      <Modal opened={resetting !== null} onClose={() => setResetting(null)} title="重置口令" centered>
        <Stack gap={16}>
          <Text size="sm">为 {resetting?.email} 生成新的临时口令，旧口令和已登录的会话会立即失效。</Text>
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={() => setResetting(null)}>
              取消
            </Button>
            <Button loading={reset.isPending} onClick={() => resetting && confirmReset(resetting)}>
              重置
            </Button>
          </Group>
        </Stack>
      </Modal>
      <PasswordRevealModal reveal={reveal} onDismiss={() => dispatch({ type: 'dismiss' })} />
    </Stack>
  )
}
