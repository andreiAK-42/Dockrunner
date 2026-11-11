import { Box, Toolbar, Divider, ListItem, ListItemButton, ListItemIcon, ListItemText, Typography } from '@mui/material'
import { Drawer } from '@mui/material'
import { Server, Home, LogOut, LogIn } from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'
import { APP_NAME } from '../../config/appConfig'
import { SidebarNav } from './SidebarNav'
import type { NavItem } from '../../config/navigation'

const DRAWER_WIDTH = 240

interface SidebarDrawerProps {
  visibleNavigation: NavItem[]
  location: { pathname: string; hash: string }
  handleNavigate: (path: string, hash?: string) => void
  expandedGroups: Record<string, boolean>
  toggleGroup: (key: string) => void
  mobileOpen: boolean
  onClose: () => void
}

export function SidebarDrawer({
  visibleNavigation,
  location,
  handleNavigate,
  expandedGroups,
  toggleGroup,
  mobileOpen,
  onClose,
}: SidebarDrawerProps) {
  const { logout, isAuthenticated } = useAuth()

  const drawerContent = (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', bgcolor: 'background.paper' }}>
      <Toolbar sx={{ px: 2, display: 'flex', alignItems: 'center', gap: 1.5 }}>
        <Box
          sx={{
            width: 32,
            height: 32,
            bgcolor: 'primary.main',
            borderRadius: 1.5,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'primary.contrastText',
          }}
        >
          <Server size={18} />
        </Box>
        <Typography variant="h6" sx={{ fontWeight: 700, fontSize: '1.125rem', letterSpacing: '-0.5px' }}>
          {APP_NAME}
        </Typography>
      </Toolbar>
      <Divider sx={{ mb: 1 }} />

      <SidebarNav
        visibleNavigation={visibleNavigation}
        location={location}
        handleNavigate={handleNavigate}
        expandedGroups={expandedGroups}
        toggleGroup={toggleGroup}
      />

      <Box sx={{ px: 1.5, pb: 1 }}>
        <Divider sx={{ mb: 1.5 }} />
        <ListItem disablePadding sx={{ mb: 0.5 }}>
          <ListItemButton onClick={() => handleNavigate('/')} sx={{ borderRadius: 1.5, py: 0.8 }}>
            <ListItemIcon sx={{ minWidth: 40, color: 'text.secondary' }}>
              <Home size={18} />
            </ListItemIcon>
            <ListItemText primary={<Typography sx={{ fontSize: '0.85rem', fontWeight: 500 }}>Главная</Typography>} />
          </ListItemButton>
        </ListItem>
        {isAuthenticated ? (
          <ListItem disablePadding sx={{ mb: 0.5 }}>
            <ListItemButton onClick={() => { logout(); handleNavigate('/'); }} sx={{ borderRadius: 1.5, py: 0.8 }}>
              <ListItemIcon sx={{ minWidth: 40, color: 'text.secondary' }}>
                <LogOut size={18} />
              </ListItemIcon>
              <ListItemText primary={<Typography sx={{ fontSize: '0.85rem', fontWeight: 500 }}>Выйти</Typography>} />
            </ListItemButton>
          </ListItem>
        ) : (
          <ListItem disablePadding sx={{ mb: 0.5 }}>
            <ListItemButton onClick={() => handleNavigate('/login')} sx={{ borderRadius: 1.5, py: 0.8 }}>
              <ListItemIcon sx={{ minWidth: 40, color: 'text.secondary' }}>
                <LogIn size={18} />
              </ListItemIcon>
              <ListItemText primary={<Typography sx={{ fontSize: '0.85rem', fontWeight: 500 }}>Войти</Typography>} />
            </ListItemButton>
          </ListItem>
        )}
      </Box>
    </Box>
  )

  return (
    <>
      <Drawer
        variant="temporary"
        open={mobileOpen}
        onClose={onClose}
        ModalProps={{ keepMounted: true }}
        sx={{
          display: { xs: 'block', md: 'none' },
          '& .MuiDrawer-paper': { boxSizing: 'border-box', width: DRAWER_WIDTH, borderTopLeftRadius: 0, borderBottomLeftRadius: 0 },
        }}
      >
        {drawerContent}
      </Drawer>
      <Drawer
        variant="permanent"
        sx={{
          display: { xs: 'none', md: 'block' },
          '& .MuiDrawer-paper': { boxSizing: 'border-box', width: DRAWER_WIDTH, borderRight: '1px solid', borderColor: 'divider', borderTopLeftRadius: 0, borderBottomLeftRadius: 0 },
        }}
        open
      >
        {drawerContent}
      </Drawer>
    </>
  )
}