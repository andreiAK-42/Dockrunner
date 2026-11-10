import { useState } from 'react'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useCustomTheme } from '../theme/CustomThemeProvider'
import { useAuth } from '../auth/AuthContext'
import {
  Box,
  Drawer,
  AppBar,
  Toolbar,
  Typography,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  IconButton,
  Divider,
  Breadcrumbs,
  Collapse,
} from '@mui/material'
import {
  LayoutDashboard,
  Box as BoxIcon,
  Server,
  Menu,
  ChevronRight,
  LogIn,
  LogOut,
  Home,
  FileCode2,
  Palette,
  User,
  Puzzle,
  ShieldCheck,
  FolderOpen,
  BookOpen,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'
import { APP_NAME } from '../config/appConfig'
import { NAVIGATION } from '../config/navigation'
import {
  filterVisible,
  getBreadcrumbs,
  getPageTitle,
  getVisibleMenuItems,
  isNavActive,
  type NavItem,
} from '../utils/navigation'

const DRAWER_WIDTH = 240

export default function Layout() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({})
  const navigate = useNavigate()
  const location = useLocation()
  const { config } = useCustomTheme()
  const { logout, isAuthenticated, user } = useAuth()
  const hasBg = !!config.backgroundImage
  const userRole = user?.role

  const visibleNavigation = getVisibleMenuItems(NAVIGATION, userRole)
  const breadcrumbs = getBreadcrumbs(location.pathname)
  const pageTitle = getPageTitle(location.pathname)

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen)
  }

  const handleNavigate = (path: string) => {
    navigate(path)
    setMobileOpen(false)
  }

  const toggleGroup = (key: string) => {
    setExpandedGroups(prev => ({ ...prev, [key]: !prev[key] }))
  }

  const renderNavItem = (item: NavItem, depth = 0) => {
    const active = isNavActive(location.pathname, item.path)
    const hasChildren = item.children && item.children.length > 0
    const isExpanded = expandedGroups[item.key]

    // Group header (секция без линка)
    if (item.isGroupHeader) {
      return (
        <Box key={item.key} sx={{ px: 2, py: 0.5, mt: depth === 0 ? 1 : 0.5 }}>
          <Typography
            variant="caption"
            sx={{
              color: 'text.secondary',
              fontWeight: 600,
              letterSpacing: 0.5,
              textTransform: 'uppercase',
              fontSize: '0.7rem',
            }}
          >
            {item.label}
          </Typography>
        </Box>
      )
    }

    // Группа с вложенными элементами
    if (hasChildren) {
      return (
        <Box key={item.key} sx={{ mb: 0.5 }}>
          <ListItemButton
            onClick={() => toggleGroup(item.key)}
            sx={{
              borderRadius: 1.5,
              py: 1,
              pl: 2 + depth * 2,
              pr: 2,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              bgcolor: active ? 'primary.main' : 'transparent',
              color: active ? 'primary.contrastText' : 'text.primary',
              '&:hover': {
                bgcolor: active ? 'primary.dark' : 'action.hover',
              },
              '& .MuiListItemIcon-root': {
                color: active ? 'primary.contrastText' : 'text.secondary',
                minWidth: 40,
              },
            }}
          >
            <ListItemIcon>{item.icon}</ListItemIcon>
            <ListItemText
              primary={
                <Typography
                  sx={{
                    fontSize: '0.875rem',
                    fontWeight: active ? 600 : 500,
                    flexGrow: 1,
                  }}
                >
                  {item.label}
                </Typography>
              }
            />
            {isExpanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          </ListItemButton>
          <Collapse in={isExpanded} timeout="auto" unmountOnExit>
            <Box sx={{ pl: depth === 0 ? 2 : 4 }}>
              {item.children!.map(child => renderNavItem(child, depth + 1))}
            </Box>
          </Collapse>
        </Box>
      )
    }

    // Обычный пункт меню
    return (
      <ListItem key={item.key} disablePadding sx={{ mb: 0.5 }}>
        <ListItemButton
          selected={active}
          onClick={() => item.path && handleNavigate(item.path)}
          sx={{
            borderRadius: 1.5,
            py: 1,
            pl: 2 + depth * 2,
            pr: 2,
            '&.Mui-selected': {
              bgcolor: 'primary.main',
              color: 'primary.contrastText',
              '&:hover': { bgcolor: 'primary.dark' },
              '& .MuiListItemIcon-root': { color: 'primary.contrastText' },
            },
          }}
        >
          <ListItemIcon
            sx={{
              minWidth: 40,
              color: active ? 'inherit' : 'text.secondary',
            }}
          >
            {item.icon}
          </ListItemIcon>
          <ListItemText
            primary={
              <Typography
                sx={{
                  fontSize: '0.875rem',
                  fontWeight: active ? 600 : 500,
                }}
              >
                {item.label}
              </Typography>
            }
          />
        </ListItemButton>
      </ListItem>
    )
  }

  const drawer = (
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

      <List sx={{ px: 1.5, flexGrow: 1, overflowY: 'auto' }}>
        {visibleNavigation.map(item => renderNavItem(item))}
      </List>

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
            <ListItemButton onClick={() => { logout(); navigate('/'); }} sx={{ borderRadius: 1.5, py: 0.8 }}>
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
    <Box sx={{ display: 'flex', minHeight: '100vh', bgcolor: hasBg ? 'transparent' : 'background.default', position: 'relative' }}>
      <AppBar
        position="fixed"
        elevation={0}
        sx={{
          width: { md: `calc(100% - ${DRAWER_WIDTH}px)` },
          ml: { md: `${DRAWER_WIDTH}px` },
          bgcolor: hasBg ? 'transparent' : 'background.default',
          backgroundColor: hasBg ? 'color-mix(in srgb, var(--custom-paper) 78%, transparent)' : undefined,
          backdropFilter: hasBg ? 'blur(10px)' : undefined,
          borderBottom: '1px solid',
          borderColor: 'divider',
          color: 'text.primary',
        }}
      >
        <Toolbar sx={{ justifyContent: 'space-between' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <IconButton color="inherit" edge="start" onClick={handleDrawerToggle} sx={{ display: { md: 'none' } }}>
              <Menu />
            </IconButton>
            <Breadcrumbs separator={<ChevronRight size={14} />} aria-label="breadcrumb">
              <Typography color="text.secondary" variant="body2" sx={{ fontWeight: 500 }}>
                {APP_NAME}
              </Typography>
              {breadcrumbs.map((item, index) => (
                <Typography
                  key={item.key}
                  color={index === breadcrumbs.length - 1 ? 'text.primary' : 'text.secondary'}
                  variant="body2"
                  sx={{ fontWeight: index === breadcrumbs.length - 1 ? 600 : 500 }}
                >
                  {item.breadcrumbLabel || item.label}
                </Typography>
              ))}
            </Breadcrumbs>
          </Box>
        </Toolbar>
      </AppBar>

      <Box component="nav" sx={{ width: { md: DRAWER_WIDTH }, flexShrink: { md: 0 } }}>
        <Drawer
          variant="temporary"
          open={mobileOpen}
          onClose={handleDrawerToggle}
          ModalProps={{ keepMounted: true }}
          sx={{
            display: { xs: 'block', md: 'none' },
            '& .MuiDrawer-paper': { boxSizing: 'border-box', width: DRAWER_WIDTH, borderTopLeftRadius: 0, borderBottomLeftRadius: 0 },
          }}
        >
          {drawer}
        </Drawer>
        <Drawer
          variant="permanent"
          sx={{
            display: { xs: 'none', md: 'block' },
            '& .MuiDrawer-paper': { boxSizing: 'border-box', width: DRAWER_WIDTH, borderRight: '1px solid', borderColor: 'divider', borderTopLeftRadius: 0, borderBottomLeftRadius: 0 },
          }}
          open
        >
          {drawer}
        </Drawer>
      </Box>

      <Box component="main" sx={{ flexGrow: 1, p: { xs: 2, md: 3 }, width: { md: `calc(100% - ${DRAWER_WIDTH}px)` }, mt: '64px', bgcolor: 'transparent', position: 'relative' }}>
        <Outlet />
      </Box>
    </Box>
  )
}