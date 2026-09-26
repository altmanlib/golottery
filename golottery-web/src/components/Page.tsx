import { Box, Group, Stack, Text, Title } from '@mantine/core'
import { IconChevronLeft } from '@tabler/icons-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import classes from './Page.module.css'

/** Padded page body. `fill` pins it to the viewport so an inner table scrolls instead of the page. */
export function Page({ fill = false, children }: { fill?: boolean; children: ReactNode }) {
  return (
    <Stack className={`${classes.page} ${fill ? classes.fill : ''}`} gap={24}>
      {children}
    </Stack>
  )
}

type HeaderProps = {
  title: ReactNode
  description?: ReactNode
  /** Rendered next to the title, e.g. a status badge. */
  badge?: ReactNode
  actions?: ReactNode
  back?: { to: string; label: string }
}

function HeaderContent({ title, description, badge, actions, back }: HeaderProps) {
  return (
    <Stack gap={12}>
      {back && (
        <Group renderRoot={(props) => <Link to={back.to} {...props} />} className={classes.back} gap={4} fz="sm" w="fit-content">
          <IconChevronLeft size={14} />
          {back.label}
        </Group>
      )}
      <Group justify="space-between" align="flex-end" gap={16} wrap="nowrap">
        <Stack gap={4} miw={0}>
          <Group gap={12} wrap="nowrap">
            <Title order={1} className={classes.title}>
              {title}
            </Title>
            {badge}
          </Group>
          {description && (
            <Text size="md" c="dimmed">
              {description}
            </Text>
          )}
        </Stack>
        {actions && (
          <Group gap={8} wrap="nowrap">
            {actions}
          </Group>
        )}
      </Group>
    </Stack>
  )
}

export function PageHeader(props: HeaderProps) {
  return <HeaderContent {...props} />
}

/** A full-width header band on the surface color; `children` go under it, e.g. tabs. */
export function PageBand({ children, ...props }: HeaderProps & { children?: ReactNode }) {
  return (
    <Stack className={classes.band} gap={16}>
      <HeaderContent {...props} />
      {children}
    </Stack>
  )
}

/** The bordered card every console section sits in. */
export function Card({ children, p = 20, flex, mih }: { children: ReactNode; p?: number; flex?: number; mih?: number }) {
  return (
    <Box className={classes.card} p={p} flex={flex} mih={mih}>
      {children}
    </Box>
  )
}

/** A titled card; `actions` sit on the title row. */
export function Section({ title, actions, children }: { title: string; actions?: ReactNode; children: ReactNode }) {
  return (
    <Card>
      <Stack gap={16}>
        <Group justify="space-between" gap={12}>
          <Title order={2} className={classes.sectionTitle}>
            {title}
          </Title>
          {actions}
        </Group>
        {children}
      </Stack>
    </Card>
  )
}
