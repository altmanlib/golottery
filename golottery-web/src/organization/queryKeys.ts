export const organizationKeys = {
  all: ['organization'] as const,
  me: () => [...organizationKeys.all, 'me'] as const,
  events: () => [...organizationKeys.all, 'events'] as const,
  eventPage: (page: number) => [...organizationKeys.events(), 'page', page] as const,
  event: (id: string) => [...organizationKeys.events(), 'detail', id] as const,
  attendees: (id: string, page: number) => [...organizationKeys.events(), 'attendees', id, page] as const,
  prizes: (id: string) => [...organizationKeys.events(), 'prizes', id] as const,
  staff: (id: string) => [...organizationKeys.events(), 'staff', id] as const,
}
