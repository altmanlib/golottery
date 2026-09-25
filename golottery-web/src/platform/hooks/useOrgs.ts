import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { unwrap } from '#/api'
import { getOrg, listOrgs } from '#/api-gen/sdk.gen'
import { PAGE_SIZE, pageOffset } from '#/platform/orgs'
import { platformKeys } from '#/platform/queryKeys'

export function useOrgPage(page: number) {
  return useQuery({
    queryKey: platformKeys.orgPage(page),
    queryFn: () => unwrap(listOrgs({ query: { offset: pageOffset(page), limit: PAGE_SIZE } })),
    placeholderData: keepPreviousData,
  })
}

export function useOrg(orgId: string) {
  return useQuery({
    queryKey: platformKeys.org(orgId),
    queryFn: () => unwrap(getOrg({ path: { orgId } })),
    retry: false,
  })
}
