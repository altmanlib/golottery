import type { RouteObject } from 'react-router-dom'

export const organizationRoutes: RouteObject[] = [
  {
    path: '/organization/login',
    lazy: async () => ({ Component: (await import('#/organization/pages/LoginPage')).LoginPage }),
  },
  {
    path: '/organization',
    lazy: async () => ({ Component: (await import('#/organization/components/OrganizationShell')).OrganizationShell }),
    children: [
      {
        index: true,
        lazy: async () => ({ Component: (await import('#/organization/pages/HomePage')).HomePage }),
      },
    ],
  },
]
