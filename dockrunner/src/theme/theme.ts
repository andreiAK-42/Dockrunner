import { createTheme } from '@mui/material/styles'
import type { ThemeOptions } from '@mui/material/styles'
import { getComponents } from './components'
import type { CustomUIConfig } from './colors'

function buildPalette(config: CustomUIConfig): ThemeOptions['palette'] {
  const isDark = config.mode === 'dark'

  let primaryLight = '#3f3f46'
  let primaryDark = '#09090b'
  let primaryContrast = '#fafafa'
  let secondaryLight = '#a1a1aa'

  if (isDark) {
    primaryLight = '#ffffff'
    primaryDark = '#e4e4e7'
    primaryContrast = '#09090b'
    secondaryLight = '#d4d4d8'
  }

  let secondaryContrast = '#fafafa'
  if (isDark) {
    secondaryContrast = '#09090b'
  }

  return {
    mode: config.mode,
    primary: {
      main: config.primaryColor,
      light: primaryLight,
      dark: primaryDark,
      contrastText: primaryContrast,
    },
    secondary: {
      main: config.secondaryColor,
      light: secondaryLight,
      dark: '#52525b',
      contrastText: secondaryContrast,
    },
    error: { main: '#ef4444', light: '#f87171', dark: '#dc2626' },
    warning: { main: '#f59e0b', light: '#fbbf24', dark: '#d97706' },
    info: { main: '#3b82f6', light: '#60a5fa', dark: '#2563eb' },
    success: { main: '#10b981', light: '#34d399', dark: '#059669' },
    background: {
      default: config.backgroundDefault,
      paper: config.backgroundPaper,
    },
    text: {
      primary: config.textPrimary,
      secondary: config.textSecondary,
    },
    divider: config.dividerColor,
  }
}

const CUSTOM_SHADOWS = [
  'none',
  '0 1px 2px 0 rgb(0 0 0 / 0.05)',
  '0 1px 3px 0 rgb(0 0 0 / 0.1), 0 1px 2px -1px rgb(0 0 0 / 0.1)',
  '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
  '0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1)',
  '0 20px 25px -5px rgb(0 0 0 / 0.1), 0 8px 10px -6px rgb(0 0 0 / 0.1)',
  '0 25px 50px -12px rgb(0 0 0 / 0.25)',
  '0 2px 4px 0 rgb(0 0 0 / 0.06)',
  '0 4px 8px 0 rgb(0 0 0 / 0.08)',
  '0 6px 12px 0 rgb(0 0 0 / 0.1)',
  '0 8px 16px 0 rgb(0 0 0 / 0.12)',
  '0 12px 24px 0 rgb(0 0 0 / 0.14)',
  '0 16px 32px 0 rgb(0 0 0 / 0.16)',
  '0 20px 40px 0 rgb(0 0 0 / 0.18)',
  '0 24px 48px 0 rgb(0 0 0 / 0.2)',
  '0 28px 56px 0 rgb(0 0 0 / 0.22)',
  '0 32px 64px 0 rgb(0 0 0 / 0.24)',
  '0 36px 72px 0 rgb(0 0 0 / 0.26)',
  '0 40px 80px 0 rgb(0 0 0 / 0.28)',
  '0 44px 88px 0 rgb(0 0 0 / 0.3)',
  '0 48px 96px 0 rgb(0 0 0 / 0.32)',
  '0 52px 104px 0 rgb(0 0 0 / 0.34)',
  '0 56px 112px 0 rgb(0 0 0 / 0.36)',
  '0 60px 120px 0 rgb(0 0 0 / 0.38)',
  '0 64px 128px 0 rgb(0 0 0 / 0.4)',
]

export function createCustomTheme(config: CustomUIConfig) {
  const palette = buildPalette(config)
  const components = getComponents(config.borderRadius)

  return createTheme({
    palette,
    typography: {
      fontFamily: '"Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      h1: { fontSize: '3rem', fontWeight: 800, lineHeight: 1.2, letterSpacing: '-0.02em' },
      h2: { fontSize: '2.25rem', fontWeight: 700, lineHeight: 1.3, letterSpacing: '-0.01em' },
      h3: { fontSize: '1.875rem', fontWeight: 700, lineHeight: 1.4 },
      h4: { fontSize: '1.5rem', fontWeight: 700, lineHeight: 1.4 },
      h5: { fontSize: '1.25rem', fontWeight: 600, lineHeight: 1.5 },
      h6: { fontSize: '1.125rem', fontWeight: 600, lineHeight: 1.5 },
      body1: { fontSize: '1rem', lineHeight: 1.6 },
      body2: { fontSize: '0.875rem', lineHeight: 1.6 },
      button: { textTransform: 'none', fontWeight: 500 },
    },
    shape: { borderRadius: config.borderRadius },
    shadows: CUSTOM_SHADOWS as ThemeOptions['shadows'],
    components: components as ThemeOptions['components'],
  })
}
