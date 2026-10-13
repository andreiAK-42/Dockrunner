import type { ThemeOptions } from '@mui/material/styles'

export const getComponents = (borderRadius: number): ThemeOptions['components'] => ({
  MuiButtonBase: {
    defaultProps: { disableRipple: true },
  },
  MuiButton: {
    defaultProps: { disableElevation: true, disableRipple: true },
    styleOverrides: {
      root: {
        borderRadius: 6,
        fontWeight: 500,
        fontSize: '0.875rem',
        padding: '0.35rem 1rem',
        transition: 'all 0.2s ease-in-out',
        '&:hover': { opacity: 0.85 },
      },
      sizeSmall: { padding: '0.2rem 0.8rem', fontSize: '0.8125rem' },
      sizeLarge: { padding: '0.5rem 1.5rem', fontSize: '0.9375rem' },
      contained: {
        '&:hover': { boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)' },
      },
      outlined: {
        borderWidth: '1.5px',
        '&:hover': { borderWidth: '1.5px' },
      },
    },
  },
  MuiIconButton: { defaultProps: { disableRipple: true } },
  MuiCheckbox: { defaultProps: { disableRipple: true } },
  MuiRadio: { defaultProps: { disableRipple: true } },
  MuiSwitch: { defaultProps: { disableRipple: true } },
  MuiChip: {
    styleOverrides: { root: { borderRadius: 6, fontWeight: 500 } },
  },
  MuiCard: {
    styleOverrides: {
      root: {
        borderRadius,
        boxShadow: '0 1px 3px 0 rgb(0 0 0 / 0.1), 0 1px 2px -1px rgb(0 0 0 / 0.1)',
      },
    },
  },
  MuiPaper: {
    styleOverrides: { root: { borderRadius, backgroundImage: 'none' } },
  },
  MuiTextField: {
    styleOverrides: {
      root: {
        '& .MuiOutlinedInput-root': {
          borderRadius: 6,
          '& input': { padding: '0.5rem 0.75rem' },
        },
        '& .MuiInputLabel-root': {
          transform: 'translate(0.75rem, 0.5rem) scale(1)',
          '&.MuiInputLabel-shrink': { transform: 'translate(0.875rem, -0.5625rem) scale(0.75)' },
        },
      },
    },
  },
  MuiInputBase: {
    styleOverrides: { input: { '&::placeholder': { opacity: 0.5 } } },
  },
  MuiDrawer: {
    styleOverrides: {
      paper: {
        border: 'none',
      },
    },
  },
})
