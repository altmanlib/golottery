import { createHashRouter, Navigate } from 'react-router-dom'
import { platformRoutes } from '#/platform/routes'
import { HostView } from '#/views/HostView'

export const router = createHashRouter([
  { path: '/', element: <Navigate to="/platform" replace /> },
  ...platformRoutes,
  { path: '/host', element: <HostView /> },
])
