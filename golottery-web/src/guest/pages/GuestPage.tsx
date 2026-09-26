import { Alert, Anchor, Button, Center, Loader, Text } from '@mantine/core'
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { ApiError } from '#/api'
import { EmptyState } from '#/components/EmptyState'
import { BindCard } from '#/guest/components/BindCard'
import { CheckinCard } from '#/guest/components/CheckinCard'
import { DoneCard } from '#/guest/components/DoneCard'
import { GuestShell } from '#/guest/components/GuestShell'
import { HelpCard } from '#/guest/components/HelpCard'
import { checkinWindow, guestStep, suggestsHelp } from '#/guest/guest'
import { useGuestStatus } from '#/guest/hooks/useGuest'

export function GuestPage() {
  const { publicId = '' } = useParams()
  const status = useGuestStatus(publicId)
  const [helping, setHelping] = useState(false)
  const offerHelp = (error: unknown) => suggestsHelp(error) && setHelping(true)

  if (status.isPending) {
    return (
      <Center mih="100dvh">
        <Loader />
      </Center>
    )
  }
  if (status.isError) {
    const missing = status.error instanceof ApiError && status.error.status === 404
    return (
      <GuestShell title="签到">
        <EmptyState
          title={missing ? '活动不存在' : status.error.message}
          description={missing ? '请确认扫描的是本次活动的签到码' : undefined}
          action={!missing && <Button onClick={() => status.refetch()}>重试</Button>}
        />
      </GuestShell>
    )
  }

  const data = status.data
  const step = guestStep(data)
  return (
    <GuestShell title={data.event.name} subtitle={checkinWindow(data.event)}>
      {step === 'done' && data.attendee && <DoneCard attendee={data.attendee} />}
      {step === 'closed' && <Alert variant="light">活动已结束</Alert>}
      {step === 'bind' && <BindCard publicId={publicId} onHelp={offerHelp} />}
      {step === 'checkin' && <CheckinCard publicId={publicId} status={data} onHelp={offerHelp} />}
      {(step === 'bind' || step === 'checkin') &&
        (helping || data.request?.status === 'pending' ? (
          <HelpCard publicId={publicId} status={data} onCancel={() => setHelping(false)} />
        ) : (
          <Text size="sm" c="dimmed" ta="center">
            遇到问题？
            <Anchor component="button" size="sm" onClick={() => setHelping(true)}>
              请工作人员协助
            </Anchor>
          </Text>
        ))}
    </GuestShell>
  )
}
