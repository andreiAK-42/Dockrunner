import { apiRequest, setTokens, clearTokens } from './client'

export interface AuthUser {
  id: string
  login: string
  role: string
  is_superuser?: boolean
}

export interface LoginResponse {
  user: AuthUser
  access_token: string
  refresh_token: string
  expires_at: string
}

export function register(login: string, password: string) {
  return apiRequest<{ data: AuthUser }>('/api/v1/auth/register', {
    method: 'POST',
    auth: false,
    body: { login, password },
  })
}

export async function login(login: string, password: string) {
  const res = await apiRequest<{ data: LoginResponse }>('/api/v1/auth/login', {
    method: 'POST',
    auth: false,
    body: { login, password },
  })
  setTokens(res.data.access_token, res.data.refresh_token, res.data.expires_at)
  return res.data
}

export function fetchMe() {
  return apiRequest<{ data: AuthUser }>('/api/v1/auth/me')
}

export async function logout() {
  try {
    // noRefresh: при протухшем токене не гонять refresh-петлю —
    // logout луч-effort, токены всё равно чистим ниже.
    await apiRequest('/api/v1/auth/logout', { method: 'POST', noRefresh: true })
  } finally {
    clearTokens()
  }
}
