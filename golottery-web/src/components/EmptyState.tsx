import { Center, Stack, Text } from '@mantine/core'
import type { ReactNode } from 'react'

type Props = {
  title: string
  description?: string
  action?: ReactNode
}

export function EmptyState({ title, description, action }: Props) {
  return (
    <Center h="100%" p={24}>
      <Stack gap={8} align="center">
        <Text fw={500}>{title}</Text>
        {description && (
          <Text size="sm" c="dimmed">
            {description}
          </Text>
        )}
        {action}
      </Stack>
    </Center>
  )
}
