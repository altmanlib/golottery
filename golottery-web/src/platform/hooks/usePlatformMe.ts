import { useQuery } from '@tanstack/react-query'
import { getToken, unwrap } from '#/api'
import { getPlatformMe } from '#/api-gen/sdk.gen'
import { platformKeys } from '#/platform/queryKeys'

export function usePlatformMe() {
  return useQuery({
    queryKey: platformKeys.me(),
    queryFn: () => unwrap(getPlatformMe()),
    enabled: Boolean(getToken('platform')),
    retry: false,
  })
}
