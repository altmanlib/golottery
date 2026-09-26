import { Navigate, type RouteObject } from 'react-router-dom'

export const organizationRoutes: RouteObject[] = [
  {
    path: '/organization/login',
    lazy: async () => ({ Component: (await import('#/organization/pages/LoginPage')).LoginPage }),
  },
  {
    path: '/organization',
    lazy: async () => ({ Component: (await import('#/organization/components/OrganizationShell')).OrganizationShell }),
    children: [
      { index: true, element: <Navigate to="events" replace /> },
      {
        path: 'events',
        lazy: async () => ({ Component: (await import('#/organization/pages/EventListPage')).EventListPage }),
      },
      {
        path: 'events/:eventId',
        lazy: async () => ({ Component: (await import('#/organization/pages/EventDetailPage')).EventDetailPage }),
      },
    ],
  },
]
