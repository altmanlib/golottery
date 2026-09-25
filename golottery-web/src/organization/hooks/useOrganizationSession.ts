import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getToken, setToken, unwrap } from '#/api'
import { getOrganizationMe, organizationLogin, organizationLogout } from '#/api-gen/sdk.gen'
import type { OrganizationLoginRequest } from '#/api-gen/types.gen'
import { organizationKeys } from '#/organization/queryKeys'

export function useOrganizationMe() {
  return useQuery({
    queryKey: organizationKeys.me(),
    queryFn: () => unwrap(getOrganizationMe()),
    enabled: Boolean(getToken('console')),
    retry: false,
  })
}

export function useOrganizationLogin(onSuccess: () => void) {
  return useMutation({
    mutationFn: (body: OrganizationLoginRequest) => unwrap(organizationLogin({ body })),
    onSuccess: (session) => {
      setToken('console', session.token)
      onSuccess()
    },
  })
}

/** Clears the local session even when the server call fails; the token is useless either way. */
export function useOrganizationLogout(onDone: () => void) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => unwrap(organizationLogout()),
    onSettled: () => {
      setToken('console', null)
      queryClient.removeQueries({ queryKey: organizationKeys.all })
      onDone()
    },
  })
}
