import { AppBar, Toolbar, Box, Typography, IconButton, Breadcrumbs } from '@mui/material'
import { Menu, ChevronRight } from 'lucide-react'
import { useCustomTheme } from '../../theme/CustomThemeProvider'
import { APP_NAME } from '../../config/appConfig'

interface TopBarProps {
  handleDrawerToggle: () => void
  breadcrumbs: Array<{ key: string; label: string; breadcrumbLabel?: string }>
}

export function TopBar({ handleDrawerToggle, breadcrumbs }: TopBarProps) {
  const { config } = useCustomTheme()
  const hasBg = !!config.backgroundImage
  const DRAWER_WIDTH = 240

  return (
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
  )
}