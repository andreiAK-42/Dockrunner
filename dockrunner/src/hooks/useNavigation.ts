import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'

interface UseNavigationReturn {
  mobileOpen: boolean
  setMobileOpen: (v: boolean) => void
  expandedGroups: Record<string, boolean>
  toggleGroup: (key: string) => void
  handleNavigate: (path: string, hash?: string) => void
  handleDrawerToggle: () => void
}

export function useNavigation(): UseNavigationReturn {
  const navigate = useNavigate()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({})

  const handleDrawerToggle = useCallback(() => {
    setMobileOpen(prev => !prev)
  }, [])

  const handleNavigate = useCallback((path: string, hash?: string) => {
    if (hash) {
      navigate(`${path}${hash}`)
    } else {
      navigate(path)
    }
    setMobileOpen(false)
  }, [navigate])

  const toggleGroup = useCallback((key: string) => {
    setExpandedGroups(prev => ({ ...prev, [key]: !prev[key] }))
  }, [])

  return {
    mobileOpen,
    setMobileOpen,
    expandedGroups,
    toggleGroup,
    handleNavigate,
    handleDrawerToggle,
  }
}