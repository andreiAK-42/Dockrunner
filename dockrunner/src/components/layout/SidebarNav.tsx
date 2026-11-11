import { Box, List, ListItem, ListItemButton, ListItemIcon, ListItemText, Typography, Collapse } from '@mui/material'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { isNavActive } from '../../utils/navigation'
import type { NavItem } from '../../config/navigation'

interface SidebarNavProps {
  visibleNavigation: NavItem[]
  location: { pathname: string; hash: string }
  handleNavigate: (path: string, hash?: string) => void
  expandedGroups: Record<string, boolean>
  toggleGroup: (key: string) => void
}

export function SidebarNav({
  visibleNavigation,
  location,
  handleNavigate,
  expandedGroups,
  toggleGroup,
}: SidebarNavProps) {
  const currentPath = location.pathname + location.hash

  const renderNavItem = (item: NavItem, depth = 0) => {
    const active = isNavActive(currentPath, item.path, item.hash)
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
              gap: 1,
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
          onClick={() => item.path && handleNavigate(item.path, item.hash)}
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

  return (
    <List sx={{ px: 1.5, flexGrow: 1, overflowY: 'auto' }}>
      {visibleNavigation.map(item => renderNavItem(item))}
    </List>
  )
}