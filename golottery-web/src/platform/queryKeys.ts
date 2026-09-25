export const platformKeys = {
  all: ['platform'] as const,
  me: () => [...platformKeys.all, 'me'] as const,
  orgs: () => [...platformKeys.all, 'orgs'] as const,
  orgPage: (page: number) => [...platformKeys.orgs(), 'page', page] as const,
  org: (id: string) => [...platformKeys.orgs(), 'detail', id] as const,
}
