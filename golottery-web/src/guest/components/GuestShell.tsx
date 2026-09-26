import { Box, Stack, Text, Title } from '@mantine/core'
import type { ReactNode } from 'react'
import classes from './GuestShell.module.css'

/** The phone layout shared by the guest and staff pages. */
export function GuestShell({ title, subtitle, children }: { title: string; subtitle?: string; children: ReactNode }) {
  return (
    <Box className={classes.root}>
      <Box className={classes.header} px={16} py={16}>
        <Stack gap={4} maw={480} mx="auto">
          <Title order={1} fz={18}>
            {title}
          </Title>
          {subtitle && (
            <Text size="sm" opacity={0.85}>
              {subtitle}
            </Text>
          )}
        </Stack>
      </Box>
      <Stack className={classes.body} gap={12} maw={480} mx="auto" p={16}>
        {children}
      </Stack>
    </Box>
  )
}
