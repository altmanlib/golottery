/**
 * Design tokens. The only place colors, radii and layout sizes are written down.
 *
 * - `palette`: raw values, named by what they are. Components never use them directly.
 * - `semantic`: what each value is for, once per color scheme. `cssVariablesResolver` in
 *   `theme.ts` emits them as `--gl-*` variables; CSS Modules read only those variables, so a
 *   dark scheme is a second column here, not a change in any component.
 * - `scale`: scheme-independent sizes, emitted as `--gl-*` too.
 */

export const palette = {
  // Deep teal ink: the sidebar and the primary color (Mantine `brand`).
  teal950: '#0F2623',
  teal900: '#16302F',
  teal850: '#1F3A38',
  teal800: '#24403E',
  teal700: '#1E4544',
  teal600: '#3A6361',
  teal500: '#1E7A6E',
  teal300: '#A9C0BD',
  teal200: '#C4D5D2',
  teal150: '#DCE7E5',
  teal100: '#E3EEEC',
  // Gold: the one warm accent.
  gold500: '#E9B949',
  gold600: '#C9A227',
  gold700: '#8A6A12',
  gold100: '#F6EEDA',
  // Warm neutrals.
  paper: '#FFFFFF',
  stone50: '#FAF9F6',
  stone100: '#F4F3EE',
  stone150: '#F0EEE8',
  stone200: '#ECEAE3',
  stone250: '#E4E2DA',
  stone300: '#DAD7CD',
  stone400: '#8A8F8C',
  stone500: '#6B7370',
  stone600: '#5B6461',
  stone700: '#4A504D',
  stone900: '#1C2321',
  draft100: '#EFEDE6',
  closed100: '#E6E5E0',
  // Dark scheme neutrals.
  night950: '#0E1413',
  night900: '#141B1A',
  night850: '#1A2322',
  night800: '#212B2A',
  night700: '#2C3836',
  night600: '#3A4745',
  night300: '#9AA6A3',
  night200: '#BAC4C1',
  night100: '#E4EAE8',
} as const

type Scheme = 'light' | 'dark'

const light = {
  /** Page ground behind cards. */
  canvas: palette.stone100,
  /** Cards, header bands, modals. */
  surface: palette.paper,
  /** Table heads and quiet fills inside a surface. */
  surfaceMuted: palette.stone50,
  border: palette.stone250,
  borderSubtle: palette.stone200,
  borderStrong: palette.stone300,
  text: palette.stone900,
  textSecondary: palette.stone600,
  textTertiary: palette.stone500,
  /** Progress tracks and meters. */
  track: palette.stone200,
  trackFill: palette.teal700,
  sidebar: palette.teal900,
  sidebarPanel: palette.teal850,
  sidebarActive: palette.teal800,
  sidebarText: palette.teal150,
  sidebarTextStrong: palette.paper,
  sidebarTextMuted: palette.teal300,
  accent: palette.gold500,
  accentContrast: palette.teal900,
  /** Items that still need attention, such as an unfinished checklist step. */
  pending: palette.gold600,
  pendingText: palette.gold700,
  pendingBg: palette.gold100,
  statusReadyBg: palette.teal100,
  statusReadyText: palette.teal700,
  statusReadyDot: palette.teal500,
  statusDraftBg: palette.draft100,
  statusDraftText: palette.stone700,
  statusDraftDot: palette.stone400,
  statusClosedBg: palette.closed100,
  statusClosedText: palette.stone900,
  statusClosedDot: palette.stone900,
}

export type SemanticToken = keyof typeof light

const dark: Record<SemanticToken, string> = {
  canvas: palette.night950,
  surface: palette.night900,
  surfaceMuted: palette.night850,
  border: palette.night700,
  borderSubtle: palette.night800,
  borderStrong: palette.night600,
  text: palette.night100,
  textSecondary: palette.night200,
  textTertiary: palette.night300,
  track: palette.night700,
  trackFill: palette.teal300,
  sidebar: palette.teal950,
  sidebarPanel: palette.teal900,
  sidebarActive: palette.teal850,
  sidebarText: palette.teal150,
  sidebarTextStrong: palette.paper,
  sidebarTextMuted: palette.teal300,
  accent: palette.gold500,
  accentContrast: palette.teal900,
  pending: palette.gold500,
  pendingText: palette.gold500,
  pendingBg: '#2A2414',
  statusReadyBg: '#16332F',
  statusReadyText: palette.teal150,
  statusReadyDot: '#4FB3A2',
  statusDraftBg: palette.night800,
  statusDraftText: palette.night200,
  statusDraftDot: palette.night300,
  statusClosedBg: palette.night700,
  statusClosedText: palette.night100,
  statusClosedDot: palette.night100,
}

export const semantic: Record<Scheme, Record<SemanticToken, string>> = { light, dark }

export const scale = {
  radiusCard: '12px',
  radiusControl: '8px',
  radiusPill: '999px',
  sidebarWidth: '232px',
  pageGutterX: '32px',
  pageGutterY: '24px',
  fontDisplay: '"Noto Serif SC", "Songti SC", serif',
} as const

/** `textSecondary` → `--gl-text-secondary`. */
export function cssVar(name: string): string {
  return `--gl-${name.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)}`
}

export function toCssVariables(tokens: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(tokens).map(([name, value]) => [cssVar(name), value]))
}
