import { Alert, Button, Group, Modal, NumberInput, Select, Stack, Text, TextInput } from '@mantine/core'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { Navigate, useNavigate, useParams } from 'react-router-dom'
import type { DrawResult, HostPrize, HostSnapshot } from '#/api-gen/types.gen'
import { EmptyState } from '#/components/EmptyState'
import { TableSkeleton } from '#/components/TableSkeleton'
import classes from '#/host/components/HostShell.module.css'
import { useCreateDraw, useHostLogout, useHostPool, useHostSnapshot, useVoidResult } from '#/host/hooks/useHost'
import { hostKeys } from '#/host/queryKeys'
import { hasHostSession } from '#/host/session'
import { readHostStream } from '#/host/stream'
import { showToast } from '#/lib/message'

function nextRequestId(): string {
  return crypto.randomUUID()
}

export function HostScreenPage() {
  const { publicId = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const authed = hasHostSession(publicId)
  const snapshot = useHostSnapshot(authed)
  const pool = useHostPool(authed)
  const draw = useCreateDraw()
  const voidResult = useVoidResult()
  const logout = useHostLogout(() => navigate(`/host/${publicId}/login`, { replace: true }))
  const [prizeId, setPrizeId] = useState<string | null>(null)
  const [count, setCount] = useState<number | string>(1)
  const [streamError, setStreamError] = useState<string | null>(null)
  const [latest, setLatest] = useState<DrawResult[]>([])
  const [voiding, setVoiding] = useState<DrawResult | null>(null)
  const [reason, setReason] = useState('缺席')

  useEffect(() => {
    if (!authed) {
      return
    }
    const controller = new AbortController()
    let stopped = false
    const connect = async () => {
      while (!stopped && !controller.signal.aborted) {
        try {
          setStreamError(null)
          await readHostStream(controller.signal, (event) => {
            if (event.type === 'snapshot') {
              queryClient.setQueryData(hostKeys.snapshot(), event.data)
              return
            }
            if (event.type === 'stats') {
              queryClient.setQueryData(hostKeys.snapshot(), (prev: HostSnapshot | undefined) => {
                if (!prev) {
                  return prev
                }
                if (event.data.draw_version !== prev.draw_version) {
                  void queryClient.invalidateQueries({ queryKey: hostKeys.snapshot() })
                }
                return { ...prev, checked_in: event.data.checked_in, draw_version: event.data.draw_version }
              })
              return
            }
            setLatest(event.data.results)
            void queryClient.invalidateQueries({ queryKey: hostKeys.snapshot() })
            void queryClient.invalidateQueries({ queryKey: hostKeys.pool() })
          })
        } catch (error) {
          if (controller.signal.aborted || stopped) {
            return
          }
          setStreamError(error instanceof Error ? error.message : '实时连接中断')
          await new Promise((resolve) => setTimeout(resolve, 2000))
        }
      }
    }
    void connect()
    return () => {
      stopped = true
      controller.abort()
    }
  }, [authed, queryClient])

  const prizes = snapshot.data?.prizes ?? []
  const selected = useMemo(() => prizes.find((prize) => prize.id === prizeId) ?? prizes[0] ?? null, [prizeId, prizes])
  useEffect(() => {
    if (!prizeId && prizes[0]) {
      setPrizeId(prizes[0].id)
    }
  }, [prizeId, prizes])

  if (!authed) {
    return <Navigate to={`/host/${publicId}/login`} replace />
  }
  if (snapshot.isPending) {
    return (
      <div className={classes.shell}>
        <main className={classes.main}>
          <TableSkeleton rows={6} />
        </main>
      </div>
    )
  }
  if (snapshot.isError || !snapshot.data) {
    return (
      <div className={classes.shell}>
        <main className={classes.main}>
          <EmptyState
            title="大屏状态加载失败"
            description={snapshot.error?.message}
            action={
              <Button size="xs" onClick={() => logout()}>
                重新登录
              </Button>
            }
          />
        </main>
      </div>
    )
  }

  const data = snapshot.data
  const startDraw = () => {
    if (!selected) {
      return
    }
    const n = Number(count)
    draw.mutate(
      { prize_id: selected.id, count: n, request_id: nextRequestId() },
      {
        onSuccess: (batch) => {
          setLatest(batch.results)
          showToast(`已抽出 ${batch.results.length} 人`, 'success')
        },
        onError: (error) => showToast(error.message, 'error'),
      },
    )
  }

  return (
    <div className={classes.shell}>
      <header className={classes.header}>
        <div className={classes.brand}>
          <div className={classes.logo} aria-hidden />
          <div className={classes.titleBlock}>
            <h1 className={classes.title}>{data.event_name}</h1>
            <p className={classes.subtitle}>现场抽奖 · {data.public_id}</p>
          </div>
        </div>
        <Group gap={12}>
          <Text size="sm" className={classes.muted}>
            版本 {data.draw_version}
          </Text>
          <Button size="xs" variant="white" onClick={() => logout()}>
            退出
          </Button>
        </Group>
      </header>
      <main className={classes.main}>
        <Stack gap={16}>
          {streamError && (
            <Alert color="yellow" variant="filled">
              实时连接异常：{streamError}。正在重连，也可刷新页面恢复。
            </Alert>
          )}
          <Group gap={16} align="stretch" grow>
            <div className={classes.panel}>
              <Text size="sm" className={classes.muted}>
                已签到
              </Text>
              <div className={classes.stat}>{data.checked_in}</div>
            </div>
            <div className={classes.panel}>
              <Text size="sm" className={classes.muted}>
                奖池人数
              </Text>
              <div className={classes.stat}>{pool.data?.items.length ?? '—'}</div>
            </div>
            <div className={classes.panel}>
              <Text size="sm" className={classes.muted}>
                活动状态
              </Text>
              <div className={classes.stat} style={{ fontSize: 28 }}>
                {data.status}
              </div>
            </div>
          </Group>

          <div className={classes.panel}>
            <Stack gap={12}>
              <Text fw={600}>抽奖控制</Text>
              <Group align="flex-end" gap={12}>
                <Select
                  label="奖项"
                  data={prizes.map((prize: HostPrize) => ({
                    value: prize.id,
                    label: `${prize.name}（剩 ${prize.remaining}/${prize.quota}）`,
                  }))}
                  value={selected?.id ?? null}
                  onChange={setPrizeId}
                  w={320}
                />
                <NumberInput label="一次抽取" min={1} max={selected?.remaining || 1} value={count} onChange={setCount} w={120} />
                <Button size="md" loading={draw.isPending} disabled={!selected || selected.remaining < 1} onClick={startDraw}>
                  开始抽奖
                </Button>
              </Group>
            </Stack>
          </div>

          <div className={classes.panel}>
            <Stack gap={12}>
              <Text fw={600}>本轮结果</Text>
              {latest.length === 0 ? (
                <Text className={classes.muted}>抽奖后会显示在这里</Text>
              ) : (
                <div className={classes.winnerGrid}>
                  {latest.map((row) => (
                    <div key={row.id} className={classes.winnerCard}>
                      <div className={classes.winnerName}>{row.attendee_name}</div>
                      <Text size="sm" className={classes.muted}>
                        {row.attendee_dept || '—'} · {row.prize_name}
                      </Text>
                      {row.status === 'valid' && (
                        <Button size="compact-xs" variant="white" mt={8} onClick={() => setVoiding(row)}>
                          作废重抽
                        </Button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </Stack>
          </div>

          <div className={classes.panel}>
            <Stack gap={12}>
              <Text fw={600}>有效中奖</Text>
              {data.winners.length === 0 ? (
                <Text className={classes.muted}>还没有有效中奖记录</Text>
              ) : (
                <div className={classes.winnerGrid}>
                  {data.winners.map((row) => (
                    <div key={row.id} className={classes.winnerCard}>
                      <div className={classes.winnerName}>{row.attendee_name}</div>
                      <Text size="sm" className={classes.muted}>
                        {row.prize_name}
                      </Text>
                    </div>
                  ))}
                </div>
              )}
            </Stack>
          </div>
        </Stack>
      </main>

      <Modal opened={voiding !== null} onClose={() => setVoiding(null)} title="作废中奖" centered>
        {voiding && (
          <Stack gap={12}>
            <Text size="sm">
              将 {voiding.attendee_name} 的「{voiding.prize_name}」标记为作废。补抽请再点一次开始抽奖。
            </Text>
            <TextInput label="原因" value={reason} onChange={(e) => setReason(e.currentTarget.value)} />
            <Group justify="flex-end" gap={8}>
              <Button variant="default" onClick={() => setVoiding(null)}>
                取消
              </Button>
              <Button
                color="red"
                loading={voidResult.isPending}
                onClick={() =>
                  voidResult.mutate(
                    { resultId: voiding.id, reason },
                    {
                      onSuccess: () => {
                        setVoiding(null)
                        showToast('已作废', 'success')
                      },
                      onError: (error) => showToast(error.message, 'error'),
                    },
                  )
                }>
                确认作废
              </Button>
            </Group>
          </Stack>
        )}
      </Modal>
    </div>
  )
}
