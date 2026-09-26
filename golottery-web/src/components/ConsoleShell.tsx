import { ActionIcon, Box, Center, Flex, Group, Stack, Text, Tooltip } from '@mantine/core'
import type { ReactNode } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import classes from './ConsoleShell.module.css'

export type ConsoleNavItem = { to: string; label: string; icon: ReactNode }

export type ConsoleAction = { label: string; icon: ReactNode; onClick: () => void; loading?: boolean }

type Props = {
  /** One character shown in the gold mark. */
  mark: string
  title: string
  subtitle: string
  nav: ConsoleNavItem[]
  /** Optional block above the user row, e.g. remaining credits. */
  panel?: ReactNode
  user: string | undefined
  actions: ConsoleAction[]
}

/** The sidebar layout shared by the operator back office and the organization console. */
export function ConsoleShell({ mark, title, subtitle, nav, panel, user, actions }: Props) {
  return (
    <Flex className={classes.root}>
      <Stack component="nav" className={classes.nav} gap={24} px={12} py={24} aria-label={subtitle}>
        <Group gap={12} px={8} wrap="nowrap">
          <Center className={classes.mark}>{mark}</Center>
          <Stack gap={0} miw={0}>
            <Text className={classes.title} fw={600} size="md" truncate>
              {title}
            </Text>
            <Text className={classes.muted} size="xs">
              {subtitle}
            </Text>
          </Stack>
        </Group>
        <Stack gap={4}>
          {nav.map((item) => (
            <Group key={item.to} renderRoot={(props) => <NavLink to={item.to} {...props} />} className={classes.link} gap={12} wrap="nowrap">
              {item.icon}
              {item.label}
            </Group>
          ))}
        </Stack>
        <Stack gap={12} mt="auto">
          {panel && (
            <Box className={classes.panel} p={12}>
              {panel}
            </Box>
          )}
          <Group gap={8} px={8} wrap="nowrap">
            <Center className={classes.avatar}>{user?.slice(0, 1) ?? ''}</Center>
            <Text size="sm" flex={1} truncate>
              {user}
            </Text>
            {actions.map((action) => (
              <Tooltip key={action.label} label={action.label}>
                <ActionIcon variant="transparent" className={classes.iconButton} aria-label={action.label} loading={action.loading} onClick={action.onClick}>
                  {action.icon}
                </ActionIcon>
              </Tooltip>
            ))}
          </Group>
        </Stack>
      </Stack>
      <Box component="main" className={classes.main} flex={1}>
        <Outlet />
      </Box>
    </Flex>
  )
}
