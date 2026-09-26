import { Alert, Badge, Button, Group, Paper, Radio, Stack, Text, TextInput } from '@mantine/core'
import { useState } from 'react'
import type { PendingRequest } from '#/api-gen/types.gen'
import { TableSkeleton } from '#/components/TableSkeleton'
import { errorMessage, type Resolution } from '#/guest/guest'
import { useApprove, useAttendeeSearch, usePendingRequests, useReject } from '#/guest/hooks/useStaff'
import { showToast } from '#/lib/message'
import { formatShanghai } from '#/organization/events'

type Draft = { choice: string; name: string; dept: string; phone: string }

/** What the draft means once a staff member picked a person or chose to add one. */
function resolutionOf(draft: Draft): Resolution | null {
  if (draft.choice === 'create') return { kind: 'create', name: draft.name, dept: draft.dept, phone: draft.phone }
  if (draft.choice.startsWith('link:')) return { kind: 'link', attendeeId: draft.choice.slice(5) }
  return null
}

/** For someone not yet bound: link a roster person or add a walk-in. */
function ResolutionPicker({ publicId, request, draft, onChange }: { publicId: string; request: PendingRequest; draft: Draft; onChange: (d: Draft) => void }) {
  const [q, setQ] = useState(request.claimed_name)
  const search = useAttendeeSearch(publicId, q)
  return (
    <Stack gap={8}>
      <TextInput size="xs" label="在名单中查找" value={q} onChange={(e) => setQ(e.currentTarget.value)} />
      <Radio.Group value={draft.choice} onChange={(choice) => onChange({ ...draft, choice })}>
        <Stack gap={8}>
          {search.data?.map((a) => (
            <Radio
              key={a.id}
              value={`link:${a.id}`}
              disabled={a.bound || a.checked_in}
              label={`${a.name}${a.dept ? ` · ${a.dept}` : ''} · 尾号 ${a.phone_last4}${a.checked_in ? '（已签到）' : a.bound ? '（已被绑定）' : ''}`}
            />
          ))}
          {search.data?.length === 0 && (
            <Text size="xs" c="dimmed">
              名单中没有匹配的人
            </Text>
          )}
          <Radio value="create" label="名单上没有，新增此人" />
        </Stack>
      </Radio.Group>
      {draft.choice === 'create' && (
        <Stack gap={8}>
          <TextInput size="xs" label="姓名" value={draft.name} onChange={(e) => onChange({ ...draft, name: e.currentTarget.value })} />
          <TextInput size="xs" label="部门" value={draft.dept} onChange={(e) => onChange({ ...draft, dept: e.currentTarget.value })} />
          <TextInput size="xs" label="手机号或后四位" value={draft.phone} onChange={(e) => onChange({ ...draft, phone: e.currentTarget.value })} />
        </Stack>
      )}
    </Stack>
  )
}

function RequestItem({ publicId, request }: { publicId: string; request: PendingRequest }) {
  const approve = useApprove(publicId)
  const reject = useReject(publicId)
  const [draft, setDraft] = useState<Draft>({ choice: '', name: request.claimed_name, dept: '', phone: request.claimed_phone_last4 })
  const bound = Boolean(request.attendee)
  const resolution = bound ? null : resolutionOf(draft)
  const done = (message: string) => ({ onSuccess: () => showToast(message, 'success') })

  return (
    <Paper withBorder radius="md" p={12}>
      <Stack gap={8}>
        <Group justify="space-between" align="flex-start" wrap="nowrap">
          <Stack gap={0}>
            <Text fw={500}>
              {request.attendee?.name ?? request.claimed_name}
              {request.attendee?.dept ? ` · ${request.attendee.dept}` : ''}
            </Text>
            <Text size="xs" c="dimmed">
              尾号 {request.attendee?.phone_last4 ?? request.claimed_phone_last4} · {formatShanghai(request.created_at)}
            </Text>
          </Stack>
          <Badge variant="light" color={bound ? 'brand' : 'yellow'}>
            {bound ? '已确认身份' : '未确认身份'}
          </Badge>
        </Group>
        {request.reason && <Text size="sm">{request.reason}</Text>}
        {!bound && <ResolutionPicker publicId={publicId} request={request} draft={draft} onChange={setDraft} />}
        {(approve.error || reject.error) && (
          <Alert color="red" variant="light">
            {errorMessage(approve.error ?? reject.error)}
          </Alert>
        )}
        <Group gap={8} justify="flex-end">
          <Button size="xs" variant="default" loading={reject.isPending} onClick={() => reject.mutate(request.id, done('已驳回'))}>
            驳回
          </Button>
          <Button
            size="xs"
            disabled={!bound && !resolution}
            loading={approve.isPending}
            onClick={() => approve.mutate({ requestId: request.id, resolution }, done('已签到'))}>
            确认签到
          </Button>
        </Group>
      </Stack>
    </Paper>
  )
}

export function RequestList({ publicId }: { publicId: string }) {
  const requests = usePendingRequests(publicId, true)
  if (requests.isPending) return <TableSkeleton rows={3} />
  if (requests.isError) return <Alert color="red">{requests.error.message}</Alert>
  if (!requests.data.length) {
    return (
      <Text size="sm" c="dimmed" ta="center" py={24}>
        暂无待处理的协助请求
      </Text>
    )
  }
  return (
    <Stack gap={8}>
      {requests.data.map((r) => (
        <RequestItem key={r.id} publicId={publicId} request={r} />
      ))}
    </Stack>
  )
}
