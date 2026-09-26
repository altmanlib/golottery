import { Alert, Button, Code, CopyButton, Group, Modal, Stack, Text } from '@mantine/core'
import { useState } from 'react'
import type { Event, HostCredential } from '#/api-gen/types.gen'
import { showToast } from '#/lib/message'
import { useUpsertHost } from '#/organization/hooks/useEvents'

function hostUrl(publicId: string): string {
  const base = window.location.origin + window.location.pathname.replace(/\/$/, '')
  return `${base}#/host/${publicId}`
}

/** Creates or resets the host password; the password is shown once. */
export function HostSection({ event }: { event: Event }) {
  const upsert = useUpsertHost(event.id)
  const [reveal, setReveal] = useState<HostCredential | null>(null)

  const create = () =>
    upsert.mutate(undefined, {
      onSuccess: (cred) => {
        upsert.reset()
        setReveal(cred)
      },
      onError: (error) => showToast(error.message, 'error'),
    })

  return (
    <Stack gap={8}>
      <Text size="sm" c="dimmed">
        大屏用活动专属链接和主持人口令登录，只能操作这一场抽奖。重置后旧口令与已登录的大屏会话立即失效。
      </Text>
      <Group gap={8}>
        <Button size="xs" variant="light" loading={upsert.isPending} onClick={create}>
          {reveal ? '重新生成口令' : '生成主持人口令'}
        </Button>
        <CopyButton value={hostUrl(event.public_id)}>
          {({ copied, copy }) => (
            <Button size="xs" variant="subtle" onClick={copy}>
              {copied ? '已复制大屏链接' : '复制大屏链接'}
            </Button>
          )}
        </CopyButton>
      </Group>
      <Code block>{hostUrl(event.public_id)}</Code>
      <Modal opened={reveal !== null} onClose={() => setReveal(null)} title="主持人口令" centered>
        {reveal && (
          <Stack gap={12}>
            <Alert color="yellow" variant="light">
              口令只显示一次，关闭后无法再查看。请交给现场主持人。
            </Alert>
            <Code block>{reveal.password}</Code>
            <Group justify="flex-end" gap={8}>
              <CopyButton value={reveal.password}>
                {({ copied, copy }) => (
                  <Button size="xs" variant="light" onClick={copy}>
                    {copied ? '已复制' : '复制口令'}
                  </Button>
                )}
              </CopyButton>
              <Button size="xs" onClick={() => setReveal(null)}>
                关闭
              </Button>
            </Group>
          </Stack>
        )}
      </Modal>
    </Stack>
  )
}
