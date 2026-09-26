import { Alert, Button, Paper, PasswordInput, Stack, Text, Title } from '@mantine/core'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ApiError } from '#/api'
import classes from '#/host/components/HostShell.module.css'
import { useHostLogin } from '#/host/hooks/useHost'
import { hasHostSession } from '#/host/session'

export function HostLoginPage() {
  const { publicId = '' } = useParams()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const login = useHostLogin(publicId, () => navigate(`/host/${publicId}`, { replace: true }))

  if (hasHostSession(publicId)) {
    navigate(`/host/${publicId}`, { replace: true })
  }

  return (
    <div className={classes.shell}>
      <header className={classes.header}>
        <div className={classes.brand}>
          <div className={classes.logo} aria-hidden />
          <div className={classes.titleBlock}>
            <h1 className={classes.title}>现场大屏</h1>
            <p className={classes.subtitle}>输入本场主持人口令后进入抽奖</p>
          </div>
        </div>
      </header>
      <main className={classes.main}>
        <Paper className={`${classes.panel} ${classes.loginCard}`} withBorder={false}>
          <form
            onSubmit={(event) => {
              event.preventDefault()
              login.mutate(password)
            }}>
            <Stack gap={12}>
              <Title order={2} fz={18}>
                主持人登录
              </Title>
              <Text size="sm" className={classes.muted}>
                活动码 {publicId}
              </Text>
              {login.error && (
                <Alert color="red" variant="light">
                  {login.error instanceof ApiError ? login.error.message : '登录失败，请稍后重试'}
                </Alert>
              )}
              <PasswordInput label="主持人口令" value={password} onChange={(e) => setPassword(e.currentTarget.value)} autoFocus />
              <Button type="submit" loading={login.isPending} disabled={!password}>
                进入大屏
              </Button>
            </Stack>
          </form>
        </Paper>
      </main>
    </div>
  )
}
