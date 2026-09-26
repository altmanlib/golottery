import { Grid, SimpleGrid, Stack, Tabs } from '@mantine/core'
import { useParams, useSearchParams } from 'react-router-dom'
import { EmptyState } from '#/components/EmptyState'
import { Card, Page, PageBand, PageHeader, Section } from '#/components/Page'
import { TableSkeleton } from '#/components/TableSkeleton'
import { AttendeeSection } from '#/organization/components/AttendeeSection'
import { CheckinSettingsForm } from '#/organization/components/CheckinSettingsForm'
import { EntrySection } from '#/organization/components/EntrySection'
import { EventStatusBadge } from '#/organization/components/EventStatusBadge'
import { EventStatusBar } from '#/organization/components/EventStatusBar'
import { HostSection } from '#/organization/components/HostSection'
import { LiveDataSection } from '#/organization/components/LiveDataSection'
import { PrizeSection } from '#/organization/components/PrizeSection'
import { ReadinessChecklist } from '#/organization/components/ReadinessChecklist'
import { StaffSection } from '#/organization/components/StaffSection'
import { DETAIL_TABS, type DetailTab, MODE_LABELS, parseTab } from '#/organization/events'
import { useEvent } from '#/organization/hooks/useEvents'
import { useOrganizationMe } from '#/organization/hooks/useOrganizationSession'
import classes from './EventDetailPage.module.css'

const BACK = { to: '/organization/events', label: '活动' }

export function EventDetailPage() {
  const { eventId = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const tab = parseTab(params.get('tab'))
  const event = useEvent(eventId)
  const me = useOrganizationMe()
  const open = (next: DetailTab) => setParams(next === 'overview' ? {} : { tab: next })

  if (event.isPending || event.isError) {
    return (
      <Page>
        <PageHeader title="活动" back={BACK} />
        {event.isPending ? <TableSkeleton rows={4} /> : <EmptyState title={event.error.message} />}
      </Page>
    )
  }

  const ev = event.data
  const counts: Partial<Record<DetailTab, number>> = { roster: ev.attendee_count, prizes: ev.prize_count }
  return (
    <Stack gap={0}>
      <PageBand
        back={BACK}
        title={ev.name}
        badge={<EventStatusBadge status={ev.status} />}
        description={`${MODE_LABELS[ev.checkin_mode]}${ev.checkin_mode === 'geo' ? ` · 半径 ${ev.radius_m} 米` : ''} · 名单上限 ${ev.max_attendees} 人`}
        actions={<EventStatusBar event={ev} credits={me.data?.event_credits} />}>
        <Tabs value={tab} onChange={(next) => open(parseTab(next))} classNames={{ list: classes.tabList, tab: classes.tab }}>
          <Tabs.List>
            {DETAIL_TABS.map((t) => (
              <Tabs.Tab key={t.value} value={t.value} rightSection={counts[t.value] !== undefined && <span className={classes.count}>{counts[t.value]}</span>}>
                {t.label}
              </Tabs.Tab>
            ))}
          </Tabs.List>
        </Tabs>
      </PageBand>
      <Page>
        {tab === 'overview' && (
          <Grid gap={24}>
            <Grid.Col span={{ base: 12, lg: 8 }}>
              <Section title="开场前检查">
                <ReadinessChecklist event={ev} onOpen={open} />
              </Section>
            </Grid.Col>
            <Grid.Col span={{ base: 12, lg: 4 }}>
              <Section title="签到入口">
                <EntrySection event={ev} />
              </Section>
            </Grid.Col>
          </Grid>
        )}
        {tab === 'settings' && (
          <Stack maw={720}>
            <Section title="签到设置">
              <CheckinSettingsForm
                key={`${ev.id}-${ev.name}-${ev.checkin_mode}-${ev.checkin_start}-${ev.checkin_end}-${ev.center_lat}-${ev.center_lng}-${ev.radius_m}-${ev.allow_multi_win}`}
                event={ev}
              />
            </Section>
          </Stack>
        )}
        {tab === 'roster' && (
          <Card>
            <AttendeeSection event={ev} />
          </Card>
        )}
        {tab === 'prizes' && (
          <Card>
            <PrizeSection event={ev} />
          </Card>
        )}
        {tab === 'onsite' && (
          <SimpleGrid cols={{ base: 1, lg: 2 }} spacing={24} verticalSpacing={24}>
            <Section title="现场工作人员">
              <StaffSection event={ev} />
            </Section>
            <Section title="大屏主持人">
              <HostSection event={ev} />
            </Section>
          </SimpleGrid>
        )}
        {tab === 'data' && (
          <Stack maw={720}>
            <Section title="现场数据">
              <LiveDataSection event={ev} />
            </Section>
          </Stack>
        )}
      </Page>
    </Stack>
  )
}
