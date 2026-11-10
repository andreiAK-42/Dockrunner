import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Provider } from 'react-redux'
import './src/index.css'
import App from './src/app/App.tsx'
import { CustomThemeProvider } from './src/theme/CustomThemeProvider.tsx'
import { store } from './src/store'
import { restoreAuth } from './src/store/slices/authSlice'
import { startRealtimeBridge } from './src/store/realtimeBridge'

startRealtimeBridge()
void store.dispatch(restoreAuth())

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Provider store={store}>
      <CustomThemeProvider>
        <App />
      </CustomThemeProvider>
    </Provider>
  </StrictMode>,
)
