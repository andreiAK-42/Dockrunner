import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { fetchMe, login as apiLogin, logout as apiLogout } from '../api/auth'
import { clearTokens, getToken } from '../api/client'

interface User {
  login: string
  name: string
  role: string
  isSuperuser: boolean
}

interface AuthContextValue {
  isAuthenticated: boolean
  user: User | null
  login: (login: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)
const STORAGE_KEY = 'dockyard-auth-v1'

function toUser(userLogin: string, role: string, isSuperuser?: boolean): User {
  const displayName = userLogin.charAt(0).toUpperCase() + userLogin.slice(1)
  return { login: userLogin, role, name: displayName, isSuperuser: !!isSuperuser }
}

function loadStoredUser(): User | null {
  try {
    const storedJson = localStorage.getItem(STORAGE_KEY)
    if (storedJson) {
      return JSON.parse(storedJson)
    }
  } catch {
    // нет сохранённой сессии
  }
  return null
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(loadStoredUser)

  useEffect(() => {
    if (!user && getToken()) {
      fetchMe()
        .then((res) => setUser(toUser(res.data.login, res.data.role, res.data.is_superuser)))
        .catch(() => clearTokens())
    }
  }, [user])

  useEffect(() => {
    try {
      if (user) {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(user))
      } else {
        localStorage.removeItem(STORAGE_KEY)
      }
    } catch {
      // localStorage недоступен
    }
  }, [user])

  const login = async (userLogin: string, password: string) => {
    const res = await apiLogin(userLogin, password)
    setUser(toUser(res.user.login, res.user.role, res.user.is_superuser))
  }

  const logout = () => {
    apiLogout().finally(() => setUser(null))
  }

  const value: AuthContextValue = {
    isAuthenticated: user !== null,
    user,
    login,
    logout,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be within AuthProvider')
  }
  return ctx
}
