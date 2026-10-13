import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { ThemeProvider, CssBaseline } from '@mui/material'
import { createCustomTheme } from './theme'
import { DEFAULT_DARK, DEFAULT_LIGHT, STORAGE_KEY, type CustomUIConfig } from './colors'

interface CustomThemeContextValue {
  config: CustomUIConfig
  setConfig: React.Dispatch<React.SetStateAction<CustomUIConfig>>
  updateConfig: (patch: Partial<CustomUIConfig>) => void
  reset: () => void
}

const CustomThemeContext = createContext<CustomThemeContextValue | null>(null)

function getDefaultsForMode(mode: string): CustomUIConfig {
  if (mode === 'dark') {
    return DEFAULT_DARK
  }
  return DEFAULT_LIGHT
}

function loadInitial(): CustomUIConfig {
  try {
    const storedJson = localStorage.getItem(STORAGE_KEY)
    if (storedJson) {
      const parsed = JSON.parse(storedJson) as CustomUIConfig
      const defaults = getDefaultsForMode(parsed.mode)
      return { ...defaults, ...parsed }
    }
  } catch {
    // нет сохранённой темы — определим по системным настройкам ниже
  }

  const prefersDark = window.matchMedia?.('(prefers-color-scheme: dark)').matches
  if (prefersDark) {
    return DEFAULT_DARK
  }
  return DEFAULT_LIGHT
}

function setCssVar(name: string, value: string) {
  document.documentElement.style.setProperty(name, value)
}

function applyBackgroundImage(config: CustomUIConfig) {
  if (config.backgroundImage) {
    setCssVar('--custom-bg-image', `url("${config.backgroundImage}")`)
    setCssVar('--custom-bg-opacity', String(config.backgroundOpacity))
    setCssVar('--custom-bg-blur', `${config.backgroundBlur}px`)
    document.body.classList.add('has-bg-image')
    return
  }

  setCssVar('--custom-bg-image', 'none')
  setCssVar('--custom-bg-opacity', '0')
  setCssVar('--custom-bg-blur', '0px')
  document.body.classList.remove('has-bg-image')
}

function applyCssVariables(config: CustomUIConfig) {
  setCssVar('--custom-primary', config.primaryColor)
  setCssVar('--custom-secondary', config.secondaryColor)
  setCssVar('--custom-bg', config.backgroundDefault)
  setCssVar('--custom-paper', config.backgroundPaper)
  setCssVar('--custom-accent', config.accentColor)
  setCssVar('--custom-text', config.textPrimary)
  setCssVar('--custom-text-secondary', config.textSecondary)
  setCssVar('--custom-divider', config.dividerColor)
  setCssVar('--custom-radius', `${config.borderRadius}px`)

  applyBackgroundImage(config)

  setCssVar('--custom-overlay', config.backgroundOverlay)
  setCssVar('--custom-overlay-opacity', String(config.overlayOpacity))
  document.body.dataset.theme = config.mode
}

function loadInitialSafe(): CustomUIConfig {
  if (typeof window === 'undefined') {
    return DEFAULT_DARK
  }
  return loadInitial()
}

export function useCustomTheme() {
  const ctx = useContext(CustomThemeContext)
  if (!ctx) {
    throw new Error('useCustomTheme must be used within CustomThemeProvider')
  }
  return ctx
}

export function CustomThemeProvider({ children }: { children: React.ReactNode }) {
  const [config, setConfig] = useState<CustomUIConfig>(loadInitialSafe)

  const updateConfig = (patch: Partial<CustomUIConfig>) => {
    setConfig((prev) => ({ ...prev, ...patch }))
  }

  const reset = () => {
    setConfig((prev) => getDefaultsForMode(prev.mode))
  }

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(config))
    } catch {
      // localStorage недоступен — тема останется только до перезагрузки
    }
  }, [config])

  useEffect(() => {
    applyCssVariables(config)
  }, [config])

  const theme = useMemo(() => createCustomTheme(config), [config])

  return (
    <CustomThemeContext.Provider value={{ config, setConfig, updateConfig, reset }}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        {children}
      </ThemeProvider>
    </CustomThemeContext.Provider>
  )
}
