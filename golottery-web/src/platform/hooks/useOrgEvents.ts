import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { unwrap } from '#/api'
import { listOrgEvents, setEventMaxAttendees } from '#/api-gen/sdk.gen'
import { PAGE_SIZE, pageOffset } from '#/platform/orgs'
import { platformKeys } from '#/platform/queryKeys'

export function useOrgEventPage(orgId: string, page: number) {
  return useQuery({
    queryKey: platformKeys.orgEventPage(orgId, page),
    queryFn: () => unwrap(listOrgEvents({ path: { orgId }, query: { offset: pageOffset(page), limit: PAGE_SIZE } })),
    enabled: Boolean(orgId),
    placeholderData: keepPreviousData,
  })
}

export function useSetEventMaxAttendees(orgId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ eventId, maxAttendees }: { eventId: string; maxAttendees: number }) =>
      unwrap(setEventMaxAttendees({ path: { orgId, eventId }, body: { max_attendees: maxAttendees } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: platformKeys.orgEvents(orgId) }),
  })
}
