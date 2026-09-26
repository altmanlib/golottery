import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'

import '@fontsource/roboto/400.css'
import '@fontsource/roboto/500.css'
import '@fontsource/roboto/700.css'
import '@fontsource/roboto-condensed/700.css'
import '@fontsource/roboto-mono/500.css'
import '@fontsource/noto-serif-sc/700.css'

import './styles/base.css'

import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import React from 'react'
import ReactDOM from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { QUERY_ROOTS, redirectAfterExpiry, setUnauthorizedHandler } from '#/api'
import { queryClient } from '#/lib/queryClient'
import { router } from '#/router'
import { cssVariablesResolver, theme } from '#/theme'

setUnauthorizedHandler((scope) => {
  // Only this scope's data goes; another signed-in entry in the same browser keeps its cache.
  const queryKey = [QUERY_ROOTS[scope]]
  void queryClient.cancelQueries({ queryKey })
  queryClient.removeQueries({ queryKey })
  // Navigate through the router: writing location.hash behind its back can leave it
  // rendering a stale page while another <Navigate> is in flight.
  const target = redirectAfterExpiry(scope, router.state.location.pathname)
  if (target) {
    void router.navigate(target, { replace: true })
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
