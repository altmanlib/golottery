import { createHashRouter, Navigate } from 'react-router-dom'
import { guestRoutes } from '#/guest/routes'
import { hostRoutes } from '#/host/routes'
import { organizationRoutes } from '#/organization/routes'
import { platformRoutes } from '#/platform/routes'

export const router = createHashRouter([
  { path: '/', element: <Navigate to="/platform" replace /> },
  ...platformRoutes,
  ...organizationRoutes,
  ...guestRoutes,
  ...hostRoutes,
])
