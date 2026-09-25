export const organizationKeys = {
  all: ['organization'] as const,
  me: () => [...organizationKeys.all, 'me'] as const,
}
