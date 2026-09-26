import { Button, type CSSVariablesResolver, createTheme, type MantineColorsTuple } from '@mantine/core'
import { scale, semantic, toCssVariables } from '#/tokens'

const brand: MantineColorsTuple = ['#E8EEEE', '#C5D4D3', '#9FB8B7', '#7A9C9A', '#55807E', '#3A6361', '#1E4544', '#0F2623', '#081614', '#040505']

export const theme = createTheme({
  fontFamily: 'Roboto, "PingFang SC", "Microsoft YaHei", "Noto Sans SC", system-ui, sans-serif',
  fontFamilyMonospace: '"Roboto Mono", ui-monospace, SFMono-Regular, Menlo, monospace',
  headings: { fontFamily: scale.fontDisplay, fontWeight: '700' },
  primaryColor: 'brand',
  primaryShade: 6,
  defaultRadius: 'md',
  radius: { sm: '6px', md: scale.radiusControl, lg: scale.radiusCard },
  colors: { brand },
  fontSizes: {
    xs: '12px',
    sm: '13px',
    md: '14px',
    lg: '16px',
    xl: '18px',
  },
  components: {
    Button: Button.extend({
      defaultProps: { size: 'sm' },
    }),
    TextInput: { defaultProps: { size: 'sm' } },
    PasswordInput: { defaultProps: { size: 'sm' } },
    Paper: { defaultProps: { radius: 'lg' } },
  },
})

/**
 * Emits every design token as a CSS variable, per color scheme. Mantine's own surface and
 * text variables point at the same tokens so its components match ours in either scheme.
 */
export const cssVariablesResolver: CSSVariablesResolver = () => ({
  variables: toCssVariables(scale),
  light: {
    ...toCssVariables(semantic.light),
    '--mantine-color-body': semantic.light.surface,
    '--mantine-color-text': semantic.light.text,
    '--mantine-color-dimmed': semantic.light.textSecondary,
    '--mantine-color-default-border': semantic.light.borderStrong,
  },
  dark: {
    ...toCssVariables(semantic.dark),
    '--mantine-color-body': semantic.dark.surface,
    '--mantine-color-text': semantic.dark.text,
    '--mantine-color-dimmed': semantic.dark.textSecondary,
    '--mantine-color-default-border': semantic.dark.borderStrong,
  },
})
