import type { RouteObject } from 'react-router-dom'

export const platformRoutes: RouteObject[] = [
  {
    path: '/platform/login',
    lazy: async () => ({ Component: (await import('#/platform/pages/LoginPage')).LoginPage }),
  },
  {
    path: '/platform',
    lazy: async () => ({ Component: (await import('#/platform/components/PlatformShell')).PlatformShell }),
    children: [
      {
        index: true,
        lazy: async () => ({ Component: (await import('#/platform/pages/HomePage')).HomePage }),
      },
    ],
  },
]
