import { createHashRouter, Navigate } from 'react-router-dom'
import { AppLayout } from '#/layouts/AppLayout'
import { ConsoleHomeView } from '#/views/ConsoleHomeView'
import { HostView } from '#/views/HostView'
import { LoginView } from '#/views/LoginView'

export const router = createHashRouter([
  {
    path: '/login',
    element: <LoginView />,
  },
  {
    path: '/host',
    element: <HostView />,
  },
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: <Navigate to="/console" replace /> },
      {
        path: 'console',
        element: <ConsoleHomeView />,
      },
    ],
  },
])
