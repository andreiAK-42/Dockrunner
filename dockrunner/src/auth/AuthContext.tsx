import { useCallback } from 'react'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { loginUser, logoutUser, type AuthUser } from '../store/slices/authSlice'

export type { AuthUser as User }

interface AuthValue {
  isAuthenticated: boolean
  user: AuthUser | null
  login: (login: string, password: string) => Promise<void>
  logout: () => void
}

/**
 * Auth state lives in Redux. The hook keeps the previous `useAuth()` API so
 * components do not need to know about the store shape.
 */
export function useAuth(): AuthValue {
  const dispatch = useAppDispatch()
  const user = useAppSelector((state) => state.auth.user)

  const login = useCallback(
    async (loginText: string, password: string) => {
      await dispatch(loginUser({ login: loginText, password })).unwrap()
    },
    [dispatch],
  )

  const logout = useCallback(() => {
    void dispatch(logoutUser())
  }, [dispatch])

  return {
    isAuthenticated: user !== null,
    user,
    login,
    logout,
  }
}
