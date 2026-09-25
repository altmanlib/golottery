export const platformKeys = {
  all: ['platform'] as const,
  me: () => [...platformKeys.all, 'me'] as const,
}
