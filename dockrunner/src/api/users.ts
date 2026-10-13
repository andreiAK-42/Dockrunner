import { apiRequest } from './client'

export interface ApiUser {
  id: string
  login: string
  role: string
}

export function fetchUsers() {
  return apiRequest<{ data: ApiUser[] }>('/api/v1/users')
}

export function fetchUser(id: string) {
  return apiRequest<{ data: ApiUser }>(`/api/v1/users/${id}`)
}

export function updateUserRole(id: string, role: string) {
  return apiRequest<{ data: ApiUser }>(`/api/v1/users/${id}/role`, {
    method: 'PATCH',
    body: { role },
  })
}

export function deleteUser(id: string) {
  return apiRequest<void>(`/api/v1/users/${id}`, { method: 'DELETE' })
}
