import type { RouteObject } from 'react-router-dom'

export const guestRoutes: RouteObject[] = [
  {
    path: '/m/:publicId',
    lazy: async () => ({ Component: (await import('#/guest/pages/GuestPage')).GuestPage }),
  },
  {
    path: '/m/:publicId/staff',
    lazy: async () => ({ Component: (await import('#/guest/pages/StaffPage')).StaffPage }),
  },
]
