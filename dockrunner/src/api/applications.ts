import { apiRequest } from './client'

export interface ApiApplication {
  id: string
  name: string
  type: string
  status: string
  image: string
  node_id: string
  url: string
}

export interface CreateApplicationPayload {
  name: string
  environment: string
  source_type: string
  source: string
  cpu: number
  memory: number
  disk: number
  node_id?: string
  env_config_id?: string
}

export function fetchApplications() {
  return apiRequest<{ data: ApiApplication[] }>('/api/v1/applications')
}

export function fetchApplication(id: string) {
  return apiRequest<{ data: ApiApplication }>(`/api/v1/applications/${id}`)
}

export function createApplication(payload: CreateApplicationPayload) {
  return apiRequest<{ data: ApiApplication }>('/api/v1/applications', {
    method: 'POST',
    body: payload,
  })
}

export function deleteApplication(id: string) {
  return apiRequest<void>(`/api/v1/applications/${id}`, { method: 'DELETE' })
}

export function startApplication(id: string) {
  return apiRequest(`/api/v1/applications/${id}/start`, { method: 'POST' })
}

export function stopApplication(id: string) {
  return apiRequest(`/api/v1/applications/${id}/stop`, { method: 'POST' })
}

export function restartApplication(id: string) {
  return apiRequest(`/api/v1/applications/${id}/restart`, { method: 'POST' })
}

export function fetchApplicationLogs(id: string) {
  return apiRequest<{ data: string[] }>(`/api/v1/applications/${id}/logs`)
}

export function fetchApplicationEvents(id: string) {
  return apiRequest<{ data: { time: string; msg: string }[] }>(`/api/v1/applications/${id}/events`)
}
