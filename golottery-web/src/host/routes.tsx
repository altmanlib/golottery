import { Navigate, type RouteObject } from 'react-router-dom'

export const hostRoutes: RouteObject[] = [
  { path: '/host', element: <Navigate to="/organization" replace /> },
  {
    path: '/host/:publicId/login',
    lazy: async () => ({ Component: (await import('#/host/pages/HostLoginPage')).HostLoginPage }),
  },
  {
    path: '/host/:publicId',
    lazy: async () => ({ Component: (await import('#/host/pages/HostScreenPage')).HostScreenPage }),
  },
]
