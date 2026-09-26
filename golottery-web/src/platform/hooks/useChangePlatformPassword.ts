import { useMutation } from '@tanstack/react-query'
import { setToken, unwrap } from '#/api'
import { changePlatformPassword } from '#/api-gen/sdk.gen'
import type { ChangePasswordRequest } from '#/api-gen/types.gen'

/** Replaces the local platform token with the one returned after a password change. */
export function useChangePlatformPassword() {
  return useMutation({
    mutationFn: (body: ChangePasswordRequest) => unwrap(changePlatformPassword({ body })),
    onSuccess: (session) => {
      setToken('platform', session.token)
    },
  })
}
