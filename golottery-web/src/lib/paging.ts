/** Reads `?page=` as a 1-based page number; anything invalid is page 1. */
export function parsePage(raw: string | null): number {
  const page = Number(raw)
  return Number.isInteger(page) && page >= 1 ? page : 1
}
