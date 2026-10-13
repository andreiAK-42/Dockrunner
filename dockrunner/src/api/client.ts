const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

const TOKEN_KEY = 'dockrunner_token'
const REFRESH_KEY = 'dockrunner_refresh'

const LOGIN_PATH = '/login'

export function apiBase() {
  return API_URL
}

export function authHeaders(): Record<string, string> {
  const token = getToken()
  if (token) {
    return { Authorization: `Bearer ${token}` }
  }
  return {}
}

export function getToken() {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

export function setTokens(access: string, refresh: string) {
  try {
    localStorage.setItem(TOKEN_KEY, access)
    localStorage.setItem(REFRESH_KEY, refresh)
  } catch {
    // localStorage недоступен — токены останутся только в памяти сессии
  }
}

export function getRefreshToken() {
  try {
    return localStorage.getItem(REFRESH_KEY)
  } catch {
    return null
  }
}

export function clearTokens() {
  try {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_KEY)
  } catch {
    // localStorage недоступен — нечего очищать
  }
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  auth?: boolean
  retry?: boolean
}

let refreshPromise: Promise<void> | null = null

async function requestNewTokens(refreshToken: string): Promise<void> {
  const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })

  if (!res.ok) {
    throw new Error('Refresh rejected')
  }

  const data = await res.json()
  setTokens(data.access_token, data.refresh_token)
}

function tryRefresh(): Promise<void> {
  if (refreshPromise) {
    return refreshPromise
  }

  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    return Promise.reject(new Error('Нет refresh-токена'))
  }

  refreshPromise = requestNewTokens(refreshToken).finally(() => {
    refreshPromise = null
  })

  return refreshPromise
}

function forceLogout() {
  clearTokens()

  if (typeof window === 'undefined') {
    return
  }

  const isAlreadyOnLogin = window.location.pathname.startsWith(LOGIN_PATH)
  if (!isAlreadyOnLogin) {
    window.location.href = LOGIN_PATH
  }
}

function buildAuthHeaders(options: RequestOptions): Record<string, string> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }

  const needsAuth = options.auth !== false
  if (needsAuth) {
    const token = getToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
  }

  return headers
}

function extractErrorMessage(data: unknown, status: number): string {
  if (data && typeof data === 'object') {
    const errorData = data as { error?: string; message?: string }
    const serverMessage = errorData.error || errorData.message
    if (serverMessage) {
      return serverMessage
    }
  }
  return `Ошибка запроса: ${status}`
}

async function doFetch<T>(path: string, options: RequestOptions): Promise<T> {
  const headers = buildAuthHeaders(options)
  const method = options.method || 'GET'
  const body = options.body !== undefined ? JSON.stringify(options.body) : undefined

  const res = await fetch(`${API_URL}${path}`, { method, headers, body })

  if (!res.ok) {
    const data = await res.json().catch(() => null)
    const message = extractErrorMessage(data, res.status)
    throw new ApiError(res.status, message)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json() as Promise<T>
}

function shouldTryRefresh(error: unknown, options: RequestOptions): boolean {
  if (!(error instanceof ApiError)) {
    return false
  }
  if (error.status !== 401) {
    return false
  }
  if (options.auth === false) {
    return false
  }
  if (options.retry) {
    return false
  }
  return true
}

async function doFetchWithRetry<T>(path: string, options: RequestOptions): Promise<T> {
  try {
    return await doFetch<T>(path, { ...options, retry: true })
  } catch (retryError) {
    const isUnauthorizedAgain = retryError instanceof ApiError && retryError.status === 401
    if (isUnauthorizedAgain) {
      forceLogout()
    }
    throw retryError
  }
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  try {
    return await doFetch<T>(path, options)
  } catch (error) {
    const canRefresh = shouldTryRefresh(error, options)
    if (!canRefresh) {
      throw error
    }

    try {
      await tryRefresh()
    } catch {
      forceLogout()
      throw error
    }

    return doFetchWithRetry<T>(path, options)
  }
}
