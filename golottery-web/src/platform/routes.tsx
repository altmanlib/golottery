import { Navigate, type RouteObject } from 'react-router-dom'

export const platformRoutes: RouteObject[] = [
  {
    path: '/platform/login',
    lazy: async () => ({ Component: (await import('#/platform/pages/LoginPage')).LoginPage }),
  },
  {
    path: '/platform',
    lazy: async () => ({ Component: (await import('#/platform/components/PlatformShell')).PlatformShell }),
    children: [
      { index: true, element: <Navigate to="orgs" replace /> },
      {
        path: 'orgs',
        lazy: async () => ({ Component: (await import('#/platform/pages/OrgListPage')).OrgListPage }),
      },
      {
        path: 'orgs/:orgId',
        lazy: async () => ({ Component: (await import('#/platform/pages/OrgDetailPage')).OrgDetailPage }),
      },
    ],
  },
]
