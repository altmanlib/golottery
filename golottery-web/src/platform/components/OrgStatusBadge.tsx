import { Badge } from '@mantine/core'
import type { OrgStatus } from '#/api-gen/types.gen'
import { STATUS_LABELS } from '#/platform/orgs'

export function OrgStatusBadge({ status }: { status: OrgStatus }) {
  return (
    <Badge variant="light" color={status === 'active' ? 'brand' : 'gray'}>
      {STATUS_LABELS[status]}
    </Badge>
  )
}
