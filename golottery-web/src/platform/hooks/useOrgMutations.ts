import { useMutation, useQueryClient } from '@tanstack/react-query'
import { unwrap } from '#/api'
import { adjustOrgCredits, createOrg, disableOrg, enableOrg, setOrgMaxAttendees } from '#/api-gen/sdk.gen'
import type { AdjustCreditsRequest, CreateOrgRequest } from '#/api-gen/types.gen'
import { platformKeys } from '#/platform/queryKeys'

/** Every organization change refreshes both the list and the detail. */
function useInvalidateOrgs() {
  const queryClient = useQueryClient()
  return () => queryClient.invalidateQueries({ queryKey: platformKeys.orgs() })
}

export function useCreateOrg() {
  const invalidate = useInvalidateOrgs()
  return useMutation({
    mutationFn: (body: CreateOrgRequest) => unwrap(createOrg({ body })),
    onSuccess: invalidate,
  })
}

export function useSetOrgStatus(orgId: string) {
  const invalidate = useInvalidateOrgs()
  return useMutation({
    mutationFn: (active: boolean) => unwrap(active ? enableOrg({ path: { orgId } }) : disableOrg({ path: { orgId } })),
    onSuccess: invalidate,
  })
}

export function useAdjustCredits(orgId: string) {
  const invalidate = useInvalidateOrgs()
  return useMutation({
    mutationFn: (body: AdjustCreditsRequest) => unwrap(adjustOrgCredits({ path: { orgId }, body })),
    onSuccess: invalidate,
  })
}

export function useSetMaxAttendees(orgId: string) {
  const invalidate = useInvalidateOrgs()
  return useMutation({
    mutationFn: (maxAttendees: number) => unwrap(setOrgMaxAttendees({ path: { orgId }, body: { max_attendees: maxAttendees } })),
    onSuccess: invalidate,
  })
}
