import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './src/index.css'
import App from './src/app/App.tsx'
import { CustomThemeProvider } from './src/theme/CustomThemeProvider.tsx'
import { AuthProvider } from './src/auth/AuthContext.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <CustomThemeProvider>
      <AuthProvider>
        <App />
      </AuthProvider>
    </CustomThemeProvider>
  </StrictMode>,
)
