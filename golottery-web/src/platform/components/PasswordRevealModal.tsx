import { Alert, Button, Code, CopyButton, Group, Modal, Stack, Text } from '@mantine/core'
import type { Reveal } from '#/platform/admins'

type Props = { reveal: Reveal | null; onDismiss: () => void }

/** Shows a one-time password; once dismissed it cannot be shown again. */
export function PasswordRevealModal({ reveal, onDismiss }: Props) {
  return (
    <Modal opened={reveal !== null} onClose={onDismiss} title={reveal?.title} centered closeOnClickOutside={false}>
      {reveal && (
        <Stack gap={12}>
          <Text size="sm">
            登录邮箱 <Code>{reveal.email}</Code>
          </Text>
          <Group gap={8}>
            <Code fz={18} px={12} py={8}>
              {reveal.password}
            </Code>
            <CopyButton value={reveal.password}>
              {({ copied, copy }) => (
                <Button size="xs" variant="light" onClick={copy}>
                  {copied ? '已复制' : '复制口令'}
                </Button>
              )}
            </CopyButton>
          </Group>
          <Alert color="yellow" variant="light">
            口令只显示这一次，关闭后无法再次查看。请交给管理员，并提醒登录后修改。
          </Alert>
          <Group justify="flex-end">
            <Button onClick={onDismiss}>我已记下</Button>
          </Group>
        </Stack>
      )}
    </Modal>
  )
}
