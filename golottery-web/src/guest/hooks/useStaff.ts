import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { unwrap } from '#/api'
import { approveRequest, getStaffSummary, listPendingRequests, proxyCheckin, rejectRequest, searchStaffAttendees, updateStaffSettings } from '#/api-gen/sdk.gen'
import type { StaffSettingsRequest } from '#/api-gen/types.gen'
import { approvalBody, type Resolution } from '#/guest/guest'
import { guestKeys } from '#/guest/queryKeys'
import { asGuest } from '#/guest/session'

const POLL_MS = 10_000

export function useStaffSummary(publicId: string, enabled: boolean) {
  return useQuery({
    queryKey: guestKeys.summary(publicId),
    queryFn: () => asGuest(publicId, () => unwrap(getStaffSummary())),
    enabled,
    refetchInterval: POLL_MS,
  })
}

export function usePendingRequests(publicId: string, enabled: boolean) {
  return useQuery({
    queryKey: guestKeys.requests(publicId),
    queryFn: () => asGuest(publicId, () => unwrap(listPendingRequests())),
    enabled,
    refetchInterval: POLL_MS,
  })
}

export function useAttendeeSearch(publicId: string, q: string) {
  const term = q.trim()
  return useQuery({
    queryKey: guestKeys.search(publicId, term),
    queryFn: () => asGuest(publicId, () => unwrap(searchStaffAttendees({ query: { q: term } }))),
    enabled: term.length > 0,
    placeholderData: keepPreviousData,
  })
}

/** Handling a request or checking someone in moves the counts, the queue and search results. */
function useRefresh(publicId: string) {
  const queryClient = useQueryClient()
  return () => {
    void queryClient.invalidateQueries({ queryKey: guestKeys.summary(publicId) })
    void queryClient.invalidateQueries({ queryKey: guestKeys.requests(publicId) })
    void queryClient.invalidateQueries({ queryKey: [...guestKeys.all, 'search', publicId] })
  }
}

export function useApprove(publicId: string) {
  const refresh = useRefresh(publicId)
  return useMutation({
    mutationFn: ({ requestId, resolution }: { requestId: string; resolution: Resolution | null }) =>
      asGuest(publicId, () => unwrap(approveRequest({ path: { requestId }, body: approvalBody(resolution) }))),
    onSettled: refresh,
  })
}

export function useReject(publicId: string) {
  const refresh = useRefresh(publicId)
  return useMutation({
    mutationFn: (requestId: string) => asGuest(publicId, () => unwrap(rejectRequest({ path: { requestId } }))),
    onSettled: refresh,
  })
}

export function useProxyCheckin(publicId: string) {
  const refresh = useRefresh(publicId)
  return useMutation({
    mutationFn: (attendeeId: string) => asGuest(publicId, () => unwrap(proxyCheckin({ body: { attendee_id: attendeeId } }))),
    onSettled: refresh,
  })
}

export function useStaffSettings(publicId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: StaffSettingsRequest) => asGuest(publicId, () => unwrap(updateStaffSettings({ body }))),
    onSuccess: (status) => queryClient.setQueryData(guestKeys.status(publicId), status),
  })
}
