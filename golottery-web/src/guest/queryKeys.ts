export const guestKeys = {
  all: ['guest'] as const,
  status: (publicId: string) => [...guestKeys.all, 'status', publicId] as const,
  summary: (publicId: string) => [...guestKeys.all, 'summary', publicId] as const,
  requests: (publicId: string) => [...guestKeys.all, 'requests', publicId] as const,
  search: (publicId: string, q: string) => [...guestKeys.all, 'search', publicId, q] as const,
}
