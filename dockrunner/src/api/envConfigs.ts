import { apiRequest } from './client'

export interface ApiEnvVar {
  key: string
  value: string
}

export interface ApiEnvConfig {
  id: string
  name: string
  description: string
  vars: ApiEnvVar[]
  created_at: string
  updated_at: string
}

export function fetchEnvConfigs() {
  return apiRequest<{ data: ApiEnvConfig[] }>('/api/v1/env-configs')
}

export function createEnvConfig(payload: { name: string; description: string; vars: ApiEnvVar[] }) {
  return apiRequest<{ data: ApiEnvConfig }>('/api/v1/env-configs', {
    method: 'POST',
    body: payload,
  })
}

export function updateEnvConfig(id: string, payload: { name: string; description: string; vars: ApiEnvVar[] }) {
  return apiRequest<{ data: ApiEnvConfig }>(`/api/v1/env-configs/${id}`, {
    method: 'PATCH',
    body: payload,
  })
}

export function deleteEnvConfig(id: string) {
  return apiRequest<void>(`/api/v1/env-configs/${id}`, { method: 'DELETE' })
}
