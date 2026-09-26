import { Alert, Button, Paper, Stack, Text } from '@mantine/core'
import { IconMapPin } from '@tabler/icons-react'
import type { GuestStatus } from '#/api-gen/types.gen'
import { errorMessage } from '#/guest/guest'
import { useCheckin } from '#/guest/hooks/useGuest'

export function CheckinCard({ publicId, status, onHelp }: { publicId: string; status: GuestStatus; onHelp: (error: unknown) => void }) {
  const checkin = useCheckin(publicId)
  const geo = status.event.checkin_mode === 'geo'

  return (
    <Paper withBorder radius="md" p={16}>
      <Stack gap={12}>
        <Stack gap={4}>
          <Text fw={500}>
            {status.attendee?.name}
            {status.attendee?.dept ? ` · ${status.attendee.dept}` : ''}
          </Text>
          <Text size="sm" c="dimmed">
            {geo ? '到达现场后点击签到，需要允许浏览器使用定位' : '点击即可签到'}
          </Text>
        </Stack>
        {checkin.error && (
          <Alert color="red" variant="light">
            {errorMessage(checkin.error)}
          </Alert>
        )}
        <Button
          size="lg"
          leftSection={geo ? <IconMapPin size={20} /> : undefined}
          loading={checkin.isPending}
          onClick={() => checkin.mutate(status.event.checkin_mode, { onError: onHelp })}>
          签到
        </Button>
      </Stack>
    </Paper>
  )
}
