import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setToken, unwrap } from '#/api'
import { platformLogout } from '#/api-gen/sdk.gen'
import { platformKeys } from '#/platform/queryKeys'

/** Clears the local session even when the server call fails; the token is useless either way. */
export function usePlatformLogout(onDone: () => void) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => unwrap(platformLogout()),
    onSettled: () => {
      setToken('platform', null)
      queryClient.removeQueries({ queryKey: platformKeys.all })
      onDone()
    },
  })
}
