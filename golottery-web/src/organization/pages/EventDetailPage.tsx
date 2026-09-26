import { Anchor, Group, Paper, SimpleGrid, Stack, Text, Title } from '@mantine/core'
import { IconArrowLeft } from '@tabler/icons-react'
import { Link, useParams } from 'react-router-dom'
import { EmptyState } from '#/components/EmptyState'
import { TableSkeleton } from '#/components/TableSkeleton'
import { AttendeeSection } from '#/organization/components/AttendeeSection'
import { CheckinSettingsForm } from '#/organization/components/CheckinSettingsForm'
import { EntrySection } from '#/organization/components/EntrySection'
import { EventStatusBadge } from '#/organization/components/EventStatusBadge'
import { EventStatusBar } from '#/organization/components/EventStatusBar'
import { HostSection } from '#/organization/components/HostSection'
import { LiveDataSection } from '#/organization/components/LiveDataSection'
import { PrizeSection } from '#/organization/components/PrizeSection'
import { StaffSection } from '#/organization/components/StaffSection'
import { useEvent } from '#/organization/hooks/useEvents'
import { useOrganizationMe } from '#/organization/hooks/useOrganizationSession'

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Paper withBorder radius="md" p={12}>
      <Stack gap={12}>
        <Text fw={500} size="sm">
          {title}
        </Text>
        {children}
      </Stack>
    </Paper>
  )
}

export function EventDetailPage() {
  const { eventId = '' } = useParams()
  const event = useEvent(eventId)
  const me = useOrganizationMe()

  const back = (
    <Anchor component={Link} to="/organization/events" size="sm">
      <Group gap={4}>
        <IconArrowLeft size={14} />
        活动列表
      </Group>
    </Anchor>
  )

  if (event.isPending) {
    return (
      <Stack gap={12}>
        {back}
        <TableSkeleton rows={4} />
      </Stack>
    )
  }
  if (event.isError) {
    return (
      <Stack gap={12}>
        {back}
        <EmptyState title={event.error.message} />
      </Stack>
    )
  }

  const ev = event.data
  return (
    <Stack gap={16} maw={1080}>
      {back}
      <Group justify="space-between">
        <Group gap={8}>
          <Title order={1} fz={18}>
            {ev.name}
          </Title>
          <EventStatusBadge status={ev.status} />
        </Group>
        <EventStatusBar event={ev} credits={me.data?.event_credits} />
      </Group>
      <SimpleGrid cols={{ base: 1, md: 2 }} spacing={16} verticalSpacing={16}>
        <Section title="签到设置">
          <CheckinSettingsForm
            key={`${ev.id}-${ev.name}-${ev.checkin_mode}-${ev.checkin_start}-${ev.checkin_end}-${ev.center_lat}-${ev.center_lng}-${ev.radius_m}-${ev.allow_multi_win}`}
            event={ev}
          />
        </Section>
        <Stack gap={16}>
          <Section title="签到入口">
            <EntrySection event={ev} />
          </Section>
          <Paper withBorder radius="md" p={12}>
            <PrizeSection event={ev} />
          </Paper>
        </Stack>
      </SimpleGrid>
      <SimpleGrid cols={{ base: 1, md: 2 }} spacing={16} verticalSpacing={16}>
        <Section title="现场工作人员">
          <StaffSection event={ev} />
        </Section>
        <Section title="大屏主持人">
          <HostSection event={ev} />
        </Section>
      </SimpleGrid>
      <Section title="现场数据">
        <LiveDataSection event={ev} />
      </Section>
      <Paper withBorder radius="md" p={12}>
        <AttendeeSection event={ev} />
      </Paper>
    </Stack>
  )
}
