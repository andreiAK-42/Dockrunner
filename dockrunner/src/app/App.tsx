import { useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate, Outlet, useNavigate } from 'react-router-dom'
import { useAppSelector, useAppDispatch } from '../store/hooks'
import { authCleared } from '../store/slices/authSlice'
import Layout from '../components/Layout'
import Dashboard from '../pages/Dashboard'
import Applications from '../pages/Applications'
import CreateApplication from '../pages/CreateApplication'
import ApplicationDetails from '../pages/ApplicationDetails'
import ContainerDetails from '../pages/ContainerDetails'
import EnvConfigs from '../pages/EnvConfigs'
import Volumes from '../pages/Volumes'
import Appearance from '../pages/Appearance'
import Profile from '../pages/Profile'
import Integrations from '../pages/Integrations'
import Admin from '../pages/Admin'
import Files from '../pages/Files'
import Login from '../pages/Login'
import Landing from '../pages/Landing'
import Privacy from '../pages/Privacy'
import Docs from '../pages/Docs'

/**
 * Защищённые роуты: пускаем только после restoreAuth, и только авторизованных.
 * Неавторизованный — мягкий редирект на /login без перезагрузки SPA.
 */
function RequireAuth() {
  const { user, initialized } = useAppSelector((state) => state.auth)
  if (!initialized) {
    return null
  }
  if (!user) {
    return <Navigate to="/login" replace />
  }
  return <Outlet />
}

/**
 * Слушает событие forceLogout из api/client и переводит на /login через роутер.
 * Вызывает preventDefault, чтобы client не делал фолбэк через location.href.
 */
function ForceLogoutRedirect() {
  const navigate = useNavigate()
  const dispatch = useAppDispatch()

  useEffect(() => {
    const handler = (event: Event) => {
      event.preventDefault()
      dispatch(authCleared())
      navigate('/login', { replace: true })
    }
    window.addEventListener('dockrunner:force-logout', handler)
    return () => window.removeEventListener('dockrunner:force-logout', handler)
  }, [navigate, dispatch])

  return null
}

export default function App() {
  return (
    <BrowserRouter>
      <ForceLogoutRedirect />
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/docs" element={<Docs />} />
        <Route path="/privacy" element={<Privacy />} />
        <Route path="/login" element={<Login />} />
        <Route element={<RequireAuth />}>
          <Route element={<Layout />}>
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/applications" element={<Applications />} />
          <Route path="/applications/new" element={<CreateApplication />} />
          <Route path="/applications/:id" element={<ApplicationDetails />} />
          <Route path="/containers/:id" element={<ContainerDetails />} />
          <Route path="/applications/:id/containers/:containerId" element={<ContainerDetails />} />
          <Route path="/env-configs" element={<EnvConfigs />} />
          <Route path="/volumes" element={<Volumes />} />
          <Route path="/appearance" element={<Appearance />} />
          <Route path="/profile" element={<Profile />} />
          <Route path="/integrations" element={<Integrations />} />
          <Route path="/databases" element={<Integrations />} />
          <Route path="/files" element={<Files />} />
          <Route path="/admin" element={<Admin />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
