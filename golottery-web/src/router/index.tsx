import { createHashRouter, Navigate } from 'react-router-dom'
import { guestRoutes } from '#/guest/routes'
import { organizationRoutes } from '#/organization/routes'
import { platformRoutes } from '#/platform/routes'
import { HostView } from '#/views/HostView'

export const router = createHashRouter([
  { path: '/', element: <Navigate to="/platform" replace /> },
  ...platformRoutes,
  ...organizationRoutes,
  ...guestRoutes,
  { path: '/host', element: <HostView /> },
])
