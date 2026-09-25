import { Button, Group, Modal, Stack, Text } from '@mantine/core'
import { useState } from 'react'
import type { OrgStatus } from '#/api-gen/types.gen'
import { showToast } from '#/lib/message'
import { useSetOrgStatus } from '#/platform/hooks/useOrgMutations'

/** Disabling asks for confirmation; enabling does not. */
export function OrgStatusControl({ orgId, status }: { orgId: string; status: OrgStatus }) {
  const [confirming, setConfirming] = useState(false)
  const setStatus = useSetOrgStatus(orgId)
  const apply = (active: boolean) =>
    setStatus.mutate(active, {
      onSuccess: () => {
        setConfirming(false)
        showToast(active ? '组织已启用' : '组织已停用', 'success')
      },
      onError: (error) => showToast(error.message, 'error'),
    })

  if (status === 'disabled') {
    return (
      <Button size="xs" loading={setStatus.isPending} onClick={() => apply(true)}>
        启用
      </Button>
    )
  }
  return (
    <>
      <Button size="xs" color="red" variant="light" onClick={() => setConfirming(true)}>
        停用
      </Button>
      <Modal opened={confirming} onClose={() => setConfirming(false)} title="停用组织" centered>
        <Stack gap={16}>
          <Text size="sm">停用后管理员无法登录，也不能创建或修改活动。已就绪活动的签到与抽奖不受影响，场次与流水保留。</Text>
          <Group justify="flex-end" gap={8}>
            <Button variant="default" onClick={() => setConfirming(false)}>
              取消
            </Button>
            <Button color="red" loading={setStatus.isPending} onClick={() => apply(false)}>
              停用
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  )
}
