export const hostKeys = {
  all: ['host'] as const,
  snapshot: () => [...hostKeys.all, 'snapshot'] as const,
  pool: () => [...hostKeys.all, 'pool'] as const,
}
