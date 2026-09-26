import { Alert, Badge, Button, Group, Paper, Stack, Text, TextInput } from '@mantine/core'
import { useDebouncedValue } from '@mantine/hooks'
import { IconSearch } from '@tabler/icons-react'
import { useState } from 'react'
import { errorMessage, METHOD_LABELS } from '#/guest/guest'
import { useAttendeeSearch, useProxyCheckin } from '#/guest/hooks/useStaff'
import { showToast } from '#/lib/message'

/** Check in someone who can't do it themselves, found by name. */
export function ProxyPanel({ publicId }: { publicId: string }) {
  const [q, setQ] = useState('')
  const [term] = useDebouncedValue(q, 300)
  const search = useAttendeeSearch(publicId, term)
  const proxy = useProxyCheckin(publicId)

  return (
    <Stack gap={8}>
      <TextInput placeholder="输入姓名查找" leftSection={<IconSearch size={16} />} value={q} onChange={(e) => setQ(e.currentTarget.value)} />
      {proxy.error && (
        <Alert color="red" variant="light">
          {errorMessage(proxy.error)}
        </Alert>
      )}
      {search.data?.map((a) => (
        <Paper key={a.id} withBorder radius="md" p={12}>
          <Group justify="space-between" wrap="nowrap">
            <Stack gap={0}>
              <Text fw={500}>
                {a.name}
                {a.dept ? ` · ${a.dept}` : ''}
              </Text>
              <Text size="xs" c="dimmed">
                尾号 {a.phone_last4}
              </Text>
            </Stack>
            {a.checked_in ? (
              <Badge variant="light" color="green">
                {a.checkin_method ? METHOD_LABELS[a.checkin_method] : '已签到'}
              </Badge>
            ) : (
              <Button
                size="xs"
                loading={proxy.isPending && proxy.variables === a.id}
                onClick={() => proxy.mutate(a.id, { onSuccess: () => showToast(`${a.name} 已签到`, 'success') })}>
                代签
              </Button>
            )}
          </Group>
        </Paper>
      ))}
      {term.trim() && search.data?.length === 0 && (
        <Text size="sm" c="dimmed" ta="center" py={16}>
          名单中没有匹配的人
        </Text>
      )}
    </Stack>
  )
}
