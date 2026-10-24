import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
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

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/docs" element={<Docs />} />
        <Route path="/privacy" element={<Privacy />} />
        <Route path="/login" element={<Login />} />
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
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
