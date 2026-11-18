import { createAsyncThunk, createSlice } from '@reduxjs/toolkit'
import { fetchMe, login as apiLogin, logout as apiLogout } from '../../api/auth'
import { clearTokens, getToken } from '../../api/client'

export interface AuthUser {
  login: string
  name: string
  role: string
  isSuperuser: boolean
}

const STORAGE_KEY = 'dockrunner-auth-v1'

export function toAuthUser(login: string, role: string, isSuperuser?: boolean): AuthUser {
  const displayName = login.charAt(0).toUpperCase() + login.slice(1)
  return { login, role, name: displayName, isSuperuser: !!isSuperuser }
}

function loadStoredUser(): AuthUser | null {
  try {
    const storedJson = localStorage.getItem(STORAGE_KEY)
    if (storedJson) {
      return JSON.parse(storedJson) as AuthUser
    }
  } catch {
    // нет сохранённой сессии
  }
  return null
}

export function persistUser(user: AuthUser | null) {
  try {
    if (user) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(user))
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  } catch {
    // localStorage недоступен
  }
}

interface AuthState {
  user: AuthUser | null
  initialized: boolean
  status: 'idle' | 'loading'
  error: string
}

const initialState: AuthState = {
  user: loadStoredUser(),
  initialized: false,
  status: 'idle',
  error: '',
}

export const restoreAuth = createAsyncThunk<AuthUser | null>('auth/restore', async () => {
  if (!getToken()) {
    return null
  }
  // access-токен мог протухнуть — всё равно идём в fetchMe:
  // apiRequest сам сделает refresh по 401, refresh-токен живёт дольше access.
  try {
    const res = await fetchMe()
    const user = toAuthUser(res.data.login, res.data.role, res.data.is_superuser)
    persistUser(user)
    return user
  } catch {
    clearTokens()
    persistUser(null)
    return null
  }
})

export const loginUser = createAsyncThunk<AuthUser, { login: string; password: string }, { rejectValue: string }>(
  'auth/login',
  async (credentials, { rejectWithValue }) => {
    try {
      const res = await apiLogin(credentials.login, credentials.password)
      const user = toAuthUser(res.user.login, res.user.role, res.user.is_superuser)
      persistUser(user)
      return user
    } catch (error) {
      return rejectWithValue(error instanceof Error ? error.message : 'Не удалось войти')
    }
  },
)

export const logoutUser = createAsyncThunk<void>('auth/logout', async () => {
  try {
    await apiLogout()
  } catch {
    // токены всё равно очищаются в apiLogout, выход не должен падать
  }
  persistUser(null)
})

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    authCleared(state) {
      state.user = null
      state.initialized = true
      state.error = ''
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(restoreAuth.pending, (state) => {
        state.status = 'loading'
      })
      .addCase(restoreAuth.fulfilled, (state, action) => {
        state.status = 'idle'
        state.initialized = true
        state.user = action.payload
      })
      .addCase(restoreAuth.rejected, (state) => {
        state.status = 'idle'
        state.initialized = true
      })
      .addCase(loginUser.pending, (state) => {
        state.status = 'loading'
        state.error = ''
      })
      .addCase(loginUser.fulfilled, (state, action) => {
        state.status = 'idle'
        state.initialized = true
        state.user = action.payload
      })
      .addCase(loginUser.rejected, (state, action) => {
        state.status = 'idle'
        state.error = action.payload || 'Не удалось войти'
      })
      .addCase(logoutUser.fulfilled, (state) => {
        state.user = null
        state.initialized = true
      })
  },
})

export const { authCleared } = authSlice.actions

export default authSlice.reducer
