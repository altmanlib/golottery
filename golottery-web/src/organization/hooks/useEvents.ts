import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { unwrap } from '#/api'
import {
  createAttendee,
  createEvent,
  createPrize,
  createStaffInvite,
  deleteAttendee,
  deletePrize,
  exportAttendees,
  exportCheckinAttempts,
  exportDrawLog,
  exportWinners,
  getEvent,
  getEventQrCode,
  importAttendees,
  listAttendees,
  listEventStaff,
  listEvents,
  listPrizes,
  removeEventStaff,
  resetLiveData,
  updateAttendee,
  updateEvent,
  updatePrize,
  upsertEventHost,
} from '#/api-gen/sdk.gen'
import type { AttendeeInput, AttendeeUpdate, PrizeInput, PrizeUpdate, StaffRole, UpdateEventRequest } from '#/api-gen/types.gen'
import { saveBlob } from '#/lib/download'
import { PAGE_SIZE } from '#/organization/events'
import { organizationKeys } from '#/organization/queryKeys'

export function useEventPage(page: number) {
  return useQuery({
    queryKey: organizationKeys.eventPage(page),
    queryFn: () => unwrap(listEvents({ query: { offset: (page - 1) * PAGE_SIZE, limit: PAGE_SIZE } })),
    placeholderData: keepPreviousData,
  })
}

export function useEvent(eventId: string) {
  return useQuery({
    queryKey: organizationKeys.event(eventId),
    queryFn: () => unwrap(getEvent({ path: { eventId } })),
    retry: false,
  })
}

export function useAttendees(eventId: string, page: number) {
  return useQuery({
    queryKey: organizationKeys.attendees(eventId, page),
    queryFn: () => unwrap(listAttendees({ path: { eventId }, query: { offset: (page - 1) * PAGE_SIZE, limit: PAGE_SIZE } })),
    placeholderData: keepPreviousData,
  })
}

export function usePrizes(eventId: string) {
  return useQuery({
    queryKey: organizationKeys.prizes(eventId),
    queryFn: () => unwrap(listPrizes({ path: { eventId } })),
  })
}

/** Any event change can move counts, status and credits, so all of them refresh. */
function useRefresh() {
  const queryClient = useQueryClient()
  return () => {
    void queryClient.invalidateQueries({ queryKey: organizationKeys.events() })
    void queryClient.invalidateQueries({ queryKey: organizationKeys.me() })
  }
}

export function useCreateEvent() {
  const refresh = useRefresh()
  return useMutation({ mutationFn: (name: string) => unwrap(createEvent({ body: { name } })), onSuccess: refresh })
}

export function useUpdateEvent(eventId: string) {
  const refresh = useRefresh()
  return useMutation({ mutationFn: (body: UpdateEventRequest) => unwrap(updateEvent({ path: { eventId }, body })), onSuccess: refresh })
}

export function useAddAttendee(eventId: string) {
  const refresh = useRefresh()
  return useMutation({ mutationFn: (body: AttendeeInput) => unwrap(createAttendee({ path: { eventId }, body })), onSuccess: refresh })
}

export function useUpdateAttendee(eventId: string) {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: ({ attendeeId, body }: { attendeeId: string; body: AttendeeUpdate }) => unwrap(updateAttendee({ path: { eventId, attendeeId }, body })),
    onSuccess: refresh,
  })
}

export function useDeleteAttendee(eventId: string) {
  const refresh = useRefresh()
  return useMutation({ mutationFn: (attendeeId: string) => unwrap(deleteAttendee({ path: { eventId, attendeeId } })), onSuccess: refresh })
}

export function useImportAttendees(eventId: string) {
  const refresh = useRefresh()
  return useMutation({ mutationFn: (file: File) => unwrap(importAttendees({ path: { eventId }, body: { file } })), onSuccess: refresh })
}

export function useAddPrize(eventId: string) {
  const refresh = useRefresh()
  return useMutation({ mutationFn: (body: PrizeInput) => unwrap(createPrize({ path: { eventId }, body })), onSuccess: refresh })
}

export function useUpdatePrize(eventId: string) {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: ({ prizeId, body }: { prizeId: string; body: PrizeUpdate }) => unwrap(updatePrize({ path: { eventId, prizeId }, body })),
    onSuccess: refresh,
  })
}

export function useDeletePrize(eventId: string) {
  const refresh = useRefresh()
  return useMutation({ mutationFn: (prizeId: string) => unwrap(deletePrize({ path: { eventId, prizeId } })), onSuccess: refresh })
}

export function useExportAttendees(eventId: string, eventName: string) {
  return useMutation({
    mutationFn: async () => {
      const blob = await unwrap<Blob>(exportAttendees({ path: { eventId }, parseAs: 'blob' }))
      saveBlob(blob, `${eventName}-名单.xlsx`)
    },
  })
}

export function useDownloadQRCode(eventId: string, eventName: string) {
  return useMutation({
    mutationFn: async () => {
      const blob = await unwrap<Blob>(getEventQrCode({ path: { eventId }, parseAs: 'blob' }))
      saveBlob(blob, `${eventName}-小程序码.png`)
    },
  })
}

export function useExportAttempts(eventId: string, eventName: string) {
  return useMutation({
    mutationFn: async () => {
      const blob = await unwrap<Blob>(exportCheckinAttempts({ path: { eventId }, parseAs: 'blob' }))
      saveBlob(blob, `${eventName}-签到明细.xlsx`)
    },
  })
}

export function useExportWinners(eventId: string, eventName: string) {
  return useMutation({
    mutationFn: async () => {
      const blob = await unwrap<Blob>(exportWinners({ path: { eventId }, parseAs: 'blob' }))
      saveBlob(blob, `${eventName}-中奖名单.xlsx`)
    },
  })
}

export function useExportDrawLog(eventId: string, eventName: string) {
  return useMutation({
    mutationFn: async () => {
      const blob = await unwrap<Blob>(exportDrawLog({ path: { eventId }, parseAs: 'blob' }))
      saveBlob(blob, `${eventName}-抽奖日志.xlsx`)
    },
  })
}

/** The host password comes back once; the caller shows it and resets the mutation. */
export function useUpsertHost(eventId: string) {
  return useMutation({
    mutationFn: () => unwrap(upsertEventHost({ path: { eventId } })),
  })
}

export function useStaff(eventId: string) {
  return useQuery({
    queryKey: organizationKeys.staff(eventId),
    queryFn: () => unwrap(listEventStaff({ path: { eventId } })),
  })
}

/** The invite code comes back once; the caller shows it and resets the mutation. */
export function useCreateInvite(eventId: string) {
  return useMutation({ mutationFn: (role: StaffRole) => unwrap(createStaffInvite({ path: { eventId }, body: { role } })) })
}

export function useRemoveStaff(eventId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (staffId: string) => unwrap(removeEventStaff({ path: { eventId, staffId } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: organizationKeys.staff(eventId) }),
  })
}

export function useResetLiveData(eventId: string) {
  const refresh = useRefresh()
  return useMutation({
    mutationFn: (confirmName: string) => unwrap(resetLiveData({ path: { eventId }, body: { confirm_name: confirmName } })),
    onSuccess: refresh,
  })
}
