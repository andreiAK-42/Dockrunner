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
  Database,
  ShieldCheck,
  FolderOpen,
  BookOpen,
} from 'lucide-react'
import { APP_NAME } from '../config/appConfig'

const DRAWER_WIDTH = 240

const NAVIGATION = [
  { text: 'Статистика', icon: <LayoutDashboard size={20} />, path: '/dashboard' },
  { text: 'Документация', icon: <BookOpen size={20} />, path: '/docs' },
  { text: 'Приложения', icon: <BoxIcon size={20} />, path: '/applications' },
  { text: 'Конфигурации .env', icon: <FileCode2 size={20} />, path: '/env-configs' },
 // { text: 'Базы данных', icon: <Database size={20} />, path: '/databases' },
  { text: 'Файлы', icon: <FolderOpen size={20} />, path: '/files' },
  { text: 'Персонализация', icon: <Palette size={20} />, path: '/appearance' },
  { text: 'Профиль', icon: <User size={20} />, path: '/profile' },
  { text: 'Администрирование', icon: <ShieldCheck size={20} />, path: '/admin' },
]

// Пункты, скрытые у админа: статистика, приложения и env уже есть в панели
// администрирования, домены удалены из продукта.
const ADMIN_HIDDEN_PATHS = ['/dashboard', '/applications', '/env-configs']

function isNavActive(currentPath: string, itemPath: string) {
  if (currentPath === itemPath) return true
  return currentPath.startsWith(itemPath + '/')
}

function getPageTitle(pathname: string) {
  const route = NAVIGATION.find((item) => item.path === pathname)
  if (route) return route.text
  if (pathname.startsWith('/docs')) return 'Документация'
  if (pathname.startsWith('/applications/new')) return 'Создание приложения'
  if (pathname.startsWith('/applications/')) return 'Детали приложения'
  if (pathname.startsWith('/containers/')) return 'Детали контейнера'
  if (pathname.startsWith('/nodes/')) return 'Детали узла'
  if (pathname.startsWith('/login')) return 'Вход'
  if (pathname.startsWith('/volumes')) return 'Тома'
  return 'Обзор'
}

export default function Layout() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const { config } = useCustomTheme()
  const { logout, isAuthenticated, user } = useAuth()
  const hasBg = !!config.backgroundImage
  const showAdmin = user?.role === 'admin'

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen)
  }

  const handleNavigate = (path: string) => {
    navigate(path)
    setMobileOpen(false)
  }

  const visibleNavigation = NAVIGATION.filter((item) => {
    const isAdminRoute = item.path === '/admin'
    if (isAdminRoute && !showAdmin) {
      return false
    }
    if (showAdmin && ADMIN_HIDDEN_PATHS.includes(item.path)) {
      return false
    }
    return true
  })

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

      <Box sx={{ px: 2, py: 1 }}>
        <Typography variant="caption" sx={{ color: 'text.secondary', fontWeight: 600, letterSpacing: 0.5, textTransform: 'uppercase' }}>
          Меню
        </Typography>
      </Box>

      <List sx={{ px: 1.5, flexGrow: 1, overflowY: 'auto' }}>
        {visibleNavigation.map((item) => {
          const active = isNavActive(location.pathname, item.path)
          const itemFontWeight = active ? 600 : 500
          const iconColor = active ? 'inherit' : 'text.secondary'
          return (
            <ListItem key={item.text} disablePadding sx={{ mb: 0.5 }}>
              <ListItemButton
                selected={active}
                onClick={() => handleNavigate(item.path)}
                sx={{
                  borderRadius: 1.5,
                  py: 1,
                  '&.Mui-selected': {
                    bgcolor: 'primary.main',
                    color: 'primary.contrastText',
                    '&:hover': { bgcolor: 'primary.dark' },
                    '& .MuiListItemIcon-root': { color: 'primary.contrastText' },
                  },
                }}
              >
                <ListItemIcon sx={{ minWidth: 40, color: iconColor }}>{item.icon}</ListItemIcon>
                <ListItemText
                  primary={<Typography sx={{ fontSize: '0.875rem', fontWeight: itemFontWeight }}>{item.text}</Typography>}
                />
              </ListItemButton>
            </ListItem>
          )
        })}
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
              <Typography color="text.primary" variant="body2" sx={{ fontWeight: 600 }}>
                {getPageTitle(location.pathname)}
              </Typography>
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
          sx={{ display: { xs: 'block', md: 'none' }, '& .MuiDrawer-paper': { boxSizing: 'border-box', width: DRAWER_WIDTH, borderTopLeftRadius: 0, borderBottomLeftRadius: 0 } }}
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
