import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { unwrap } from '#/api'
import { createDraw, getHostPool, getHostSnapshot, hostLogin, voidDrawResult } from '#/api-gen/sdk.gen'
import { hostKeys } from '#/host/queryKeys'
import { clearHostSession, setHostSession } from '#/host/session'

export function useHostLogin(publicId: string, onSuccess: () => void) {
  return useMutation({
    mutationFn: (password: string) => unwrap(hostLogin({ body: { event_public_id: publicId, password } })),
    onSuccess: (session) => {
      setHostSession(publicId, session.token)
      onSuccess()
    },
  })
}

export function useHostSnapshot(enabled: boolean) {
  return useQuery({
    queryKey: hostKeys.snapshot(),
    queryFn: () => unwrap(getHostSnapshot()),
    enabled,
    retry: false,
  })
}

export function useHostPool(enabled: boolean) {
  return useQuery({
    queryKey: hostKeys.pool(),
    queryFn: () => unwrap(getHostPool()),
    enabled,
    refetchInterval: 10_000,
  })
}

export function useCreateDraw() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: { prize_id: string; count: number; request_id: string }) => unwrap(createDraw({ body })),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: hostKeys.snapshot() })
      queryClient.invalidateQueries({ queryKey: hostKeys.pool() })
    },
  })
}

export function useVoidResult() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ resultId, reason }: { resultId: string; reason: string }) => unwrap(voidDrawResult({ path: { resultId }, body: { reason } })),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: hostKeys.snapshot() })
      queryClient.invalidateQueries({ queryKey: hostKeys.pool() })
    },
  })
}

export function useHostLogout(onDone: () => void) {
  const queryClient = useQueryClient()
  return () => {
    clearHostSession()
    queryClient.removeQueries({ queryKey: hostKeys.all })
    onDone()
  }
}
