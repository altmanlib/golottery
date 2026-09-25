import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'

import '@fontsource/roboto/400.css'
import '@fontsource/roboto/500.css'
import '@fontsource/roboto/700.css'
import '@fontsource/roboto-condensed/700.css'
import '@fontsource/roboto-mono/500.css'

import './styles/base.css'

import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import React from 'react'
import ReactDOM from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { setUnauthorizedHandler, type TokenScope } from '#/api'
import { queryClient } from '#/lib/queryClient'
import { router } from '#/router'
import { cssVariablesResolver, theme } from '#/theme'

/** Where each scope goes after its token is rejected; console and host join in later phases. */
const LOGIN_ROUTES: Partial<Record<TokenScope, string>> = {
  platform: '/platform/login',
}

setUnauthorizedHandler((scope) => {
  void queryClient.cancelQueries()
  queryClient.clear()
  const target = LOGIN_ROUTES[scope]
  if (target && !window.location.hash.startsWith(`#${target}`)) {
    window.location.hash = `#${target}`
  }
})

const root = document.getElementById('root')
if (!root) {
  throw new Error('root element missing')
}

ReactDOM.createRoot(root).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <MantineProvider theme={theme} cssVariablesResolver={cssVariablesResolver} forceColorScheme="light">
        <Notifications position="bottom-right" />
        <RouterProvider router={router} />
      </MantineProvider>
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  </React.StrictMode>,
)
