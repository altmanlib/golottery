import { Button, type CSSVariablesResolver, createTheme, type MantineColorsTuple } from '@mantine/core';

const brand: MantineColorsTuple = ['#E8EEEE', '#C5D4D3', '#9FB8B7', '#7A9C9A', '#55807E', '#3A6361', '#1E4544', '#0F2623', '#081614', '#040505'];

export const theme = createTheme({
  fontFamily: 'Roboto, "PingFang SC", "Microsoft YaHei", "Noto Sans SC", system-ui, sans-serif',
  fontFamilyMonospace: '"Roboto Mono", ui-monospace, SFMono-Regular, Menlo, monospace',
  primaryColor: 'brand',
  primaryShade: 6,
  defaultRadius: 'sm',
  colors: { brand },
  fontSizes: {
    xs: '12px',
    sm: '13px',
    md: '14px',
    lg: '16px',
    xl: '18px',
  },
  other: {
    body: '#ffffff',
    text: '#000000',
    textDimmed: '#636363',
    border: '#bfbfbf',
  },
  components: {
    Button: Button.extend({
      defaultProps: { radius: 'sm', size: 'sm' },
    }),
    TextInput: { defaultProps: { radius: 'sm', size: 'sm' } },
    PasswordInput: { defaultProps: { radius: 'sm', size: 'sm' } },
  },
});

export const cssVariablesResolver: CSSVariablesResolver = (t) => ({
  variables: {},
  light: {
    '--mantine-color-body': t.other.body,
    '--mantine-color-text': t.other.text,
    '--mantine-color-dimmed': t.other.textDimmed,
    '--mantine-color-default-border': t.other.border,
  },
  dark: {},
});
