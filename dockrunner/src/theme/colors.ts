export type ColorMode = 'light' | 'dark'

export interface CustomUIConfig {
  mode: ColorMode
  primaryColor: string
  secondaryColor: string
  backgroundDefault: string
  backgroundPaper: string
  accentColor: string
  textPrimary: string
  textSecondary: string
  dividerColor: string
  backgroundImage: string | null 
  backgroundOpacity: number
  backgroundBlur: number 
  backgroundOverlay: string
  overlayOpacity: number
  borderRadius: number
}

export const DEFAULT_LIGHT: CustomUIConfig = {
  mode: 'light',
  primaryColor: '#18181b',
  secondaryColor: '#71717a',
  backgroundDefault: '#fafafa',
  backgroundPaper: '#ffffff',
  accentColor: '#aa3bff',
  textPrimary: '#09090b',
  textSecondary: '#71717a',
  dividerColor: '#e4e4e7',
  backgroundImage: null,
  backgroundOpacity: 0.15,
  backgroundBlur: 0,
  backgroundOverlay: '#ffffff',
  overlayOpacity: 0.0,
  borderRadius: 12,
}

export const DEFAULT_DARK: CustomUIConfig = {
  mode: 'dark',
  primaryColor: '#fafafa',
  secondaryColor: '#a1a1aa',
  backgroundDefault: '#09090b',
  backgroundPaper: '#18181b',
  accentColor: '#c084fc',
  textPrimary: '#fafafa',
  textSecondary: '#a1a1aa',
  dividerColor: '#27272a',
  backgroundImage: null,
  backgroundOpacity: 0.12,
  backgroundBlur: 0,
  backgroundOverlay: '#09090b',
  overlayOpacity: 0.0,
  borderRadius: 12,
}

export const PRESET_BACKGROUNDS = [
  'https://images.unsplash.com/photo-1451187580459-43490279c429?w=1920&q=80',
  'https://images.unsplash.com/photo-1519681393784-d120267933ba?w=1920&q=80',
  'https://images.unsplash.com/photo-1534796636912-3b95b3ab5986?w=1920&q=80',
  'https://images.unsplash.com/photo-1505506874110-6a7a69069a08?w=1920&q=80',
  'https://images.unsplash.com/photo-1518837695005-2083093ee35b?w=1920&q=80',
]

export const STORAGE_KEY = 'dockyard-ui-config-v1'
