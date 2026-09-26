import { Alert, Button, Code, CopyButton, Group, Image, Stack, Text } from '@mantine/core'
import { IconDownload } from '@tabler/icons-react'
import QRCode from 'qrcode'
import { useEffect, useState } from 'react'
import type { Event } from '#/api-gen/types.gen'
import { guestEntryUrl } from '#/organization/events'
import { useDownloadQRCode } from '#/organization/hooks/useEvents'

export function EntrySection({ event }: { event: Event }) {
  const url = guestEntryUrl(window.location.origin + window.location.pathname.replace(/\/$/, ''), event.public_id)
  const [qr, setQr] = useState<string | null>(null)
  const mpCode = useDownloadQRCode(event.id, event.name)

  useEffect(() => {
    let alive = true
    QRCode.toDataURL(url, { margin: 1, width: 512 }).then((data) => alive && setQr(data))
    return () => {
      alive = false
    }
  }, [url])

  return (
    <Stack gap={12}>
      <Text size="sm" c="dimmed">
        宾客用手机扫码或打开链接进入签到页。活动需先设为就绪。
      </Text>
      <Group gap={16} align="flex-start" wrap="nowrap">
        {qr && <Image src={qr} alt="" w={132} h={132} />}
        <Stack gap={8} miw={0}>
          <Code block>{url}</Code>
          <Group gap={8}>
            <CopyButton value={url}>
              {({ copied, copy }) => (
                <Button size="xs" variant="light" onClick={copy}>
                  {copied ? '已复制' : '复制链接'}
                </Button>
              )}
            </CopyButton>
            {qr && (
              <Button size="xs" variant="light" component="a" href={qr} download={`${event.name}-签到码.png`} leftSection={<IconDownload size={16} />}>
                下载二维码
              </Button>
            )}
            <Button size="xs" variant="subtle" loading={mpCode.isPending} onClick={() => mpCode.mutate()}>
              下载小程序码
            </Button>
          </Group>
          {mpCode.error && (
            <Alert color="yellow" variant="light">
              {mpCode.error.message}
            </Alert>
          )}
        </Stack>
      </Group>
    </Stack>
  )
}
