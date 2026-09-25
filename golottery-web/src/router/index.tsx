import { createHashRouter, Navigate } from 'react-router-dom'
import { organizationRoutes } from '#/organization/routes'
import { platformRoutes } from '#/platform/routes'
import { HostView } from '#/views/HostView'

export const router = createHashRouter([
  { path: '/', element: <Navigate to="/platform" replace /> },
  ...platformRoutes,
  ...organizationRoutes,
  { path: '/host', element: <HostView /> },
])
