import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { unwrap } from '#/api'
import { createOrgUser, disableOrgUser, enableOrgUser, listOrgUsers, resetOrgUserPassword } from '#/api-gen/sdk.gen'
import type { CreateOrgUserRequest } from '#/api-gen/types.gen'
import { platformKeys } from '#/platform/queryKeys'

export function useOrgUsers(orgId: string) {
  return useQuery({
    queryKey: platformKeys.orgUsers(orgId),
    queryFn: () => unwrap(listOrgUsers({ path: { orgId } })),
  })
}

function useInvalidateUsers(orgId: string) {
  const queryClient = useQueryClient()
  return () => queryClient.invalidateQueries({ queryKey: platformKeys.orgUsers(orgId) })
}

export function useCreateOrgUser(orgId: string) {
  const invalidate = useInvalidateUsers(orgId)
  return useMutation({
    mutationFn: (body: CreateOrgUserRequest) => unwrap(createOrgUser({ path: { orgId }, body })),
    onSuccess: invalidate,
  })
}

export function useResetOrgUserPassword(orgId: string) {
  return useMutation({
    mutationFn: (userId: string) => unwrap(resetOrgUserPassword({ path: { orgId, userId } })),
  })
}

export function useSetOrgUserStatus(orgId: string) {
  const invalidate = useInvalidateUsers(orgId)
  return useMutation({
    mutationFn: ({ userId, active }: { userId: string; active: boolean }) =>
      unwrap(active ? enableOrgUser({ path: { orgId, userId } }) : disableOrgUser({ path: { orgId, userId } })),
    onSuccess: invalidate,
  })
}
