import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { unwrap } from '#/api'
import { getGuestStatus, guestBind, guestCheckin, joinStaff, submitManualRequest } from '#/api-gen/sdk.gen'
import type { GuestStatus, ManualRequestInput } from '#/api-gen/types.gen'
import { type BindValues, checkinBody, currentFix } from '#/guest/guest'
import { guestKeys } from '#/guest/queryKeys'
import { asGuest } from '#/guest/session'

const REQUEST_POLL_MS = 10_000

export function useGuestStatus(publicId: string) {
  return useQuery({
    queryKey: guestKeys.status(publicId),
    queryFn: () => asGuest(publicId, () => unwrap(getGuestStatus())),
    retry: false,
    // While staff look at a help request, pick up their decision without a reload.
    refetchInterval: (query) => (query.state.data?.request?.status === 'pending' && !query.state.data.attendee?.checked_in ? REQUEST_POLL_MS : false),
  })
}

/** Every guest mutation answers with the new status; it replaces the cached one. */
function useStatusMutation<V>(publicId: string, call: (values: V) => Promise<GuestStatus>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (values: V) => asGuest(publicId, () => call(values)),
    onSuccess: (status) => queryClient.setQueryData(guestKeys.status(publicId), status),
  })
}

export function useBind(publicId: string) {
  return useStatusMutation(publicId, (values: BindValues) => unwrap(guestBind({ body: { name: values.name.trim(), phone_last4: values.phoneLast4 } })))
}

export function useCheckin(publicId: string) {
  return useStatusMutation(publicId, async (mode: GuestStatus['event']['checkin_mode']) => {
    const fix = mode === 'geo' ? await currentFix() : null
    return unwrap(guestCheckin({ body: checkinBody(mode, fix) }))
  })
}

export function useHelpRequest(publicId: string) {
  return useStatusMutation(publicId, (body: ManualRequestInput) => unwrap(submitManualRequest({ body })))
}

export function useJoinStaff(publicId: string) {
  return useStatusMutation(publicId, (invite: string) => unwrap(joinStaff({ body: { invite } })))
}
