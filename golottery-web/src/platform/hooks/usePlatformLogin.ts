import { useMutation } from '@tanstack/react-query'
import { setToken, unwrap } from '#/api'
import { platformLogin } from '#/api-gen/sdk.gen'
import type { PlatformLoginRequest } from '#/api-gen/types.gen'

export function usePlatformLogin(onSuccess: () => void) {
  return useMutation({
    mutationFn: (body: PlatformLoginRequest) => unwrap(platformLogin({ body })),
    onSuccess: (session) => {
      setToken('platform', session.token)
      onSuccess()
    },
  })
}
