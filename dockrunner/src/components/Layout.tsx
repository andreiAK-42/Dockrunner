import { Box } from '@mui/material'
import { Outlet, useLocation } from 'react-router-dom'
import { useCustomTheme } from '../theme/CustomThemeProvider'
import { useAuth } from '../auth/AuthContext'
import { useNavigation } from '../hooks/useNavigation'
import { getBreadcrumbs, getVisibleMenuItems } from '../utils/navigation'
import { NAVIGATION } from '../config/navigation'
import { TopBar } from './layout/TopBar'
import { SidebarDrawer } from './layout/SidebarDrawer'

const DRAWER_WIDTH = 240

export default function Layout() {
  const { config } = useCustomTheme()
  const { user } = useAuth()
  const location = useLocation()
  const hasBg = !!config.backgroundImage
  const userRole = user?.role

  const visibleNavigation = getVisibleMenuItems(NAVIGATION, userRole)
  const breadcrumbs = getBreadcrumbs(location.pathname)

  const {
    mobileOpen,
    expandedGroups,
    toggleGroup,
    handleNavigate,
    handleDrawerToggle,
  } = useNavigation()

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh', bgcolor: hasBg ? 'transparent' : 'background.default', position: 'relative' }}>
      <TopBar handleDrawerToggle={handleDrawerToggle} breadcrumbs={breadcrumbs} />

      <Box component="nav" sx={{ width: { md: DRAWER_WIDTH }, flexShrink: { md: 0 } }}>
        <SidebarDrawer
          visibleNavigation={visibleNavigation}
          location={{ pathname: location.pathname, hash: location.hash }}
          handleNavigate={handleNavigate}
          expandedGroups={expandedGroups}
          toggleGroup={toggleGroup}
          mobileOpen={mobileOpen}
          onClose={handleDrawerToggle}
        />
      </Box>

      <Box component="main" sx={{ flexGrow: 1, p: { xs: 2, md: 3 }, width: { md: `calc(100% - ${DRAWER_WIDTH}px)` }, mt: '64px', bgcolor: 'transparent', position: 'relative' }}>
        <Outlet />
      </Box>
    </Box>
  )
}