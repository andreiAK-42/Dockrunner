import { apiRequest } from './client'

export interface ApiVolume {
  node_id: string
  name: string
  owner_login: string
  container_path: string
  size_gb: number
  status: string
  updated_at: string
}

export function fetchVolumes() {
  return apiRequest<{ data: ApiVolume[] }>('/api/v1/volumes')
}

export function createVolume(payload: { name: string; node_id: string; size_gb: number; container_path: string }) {
  return apiRequest<{ data: ApiVolume }>('/api/v1/volumes', {
    method: 'POST',
    body: payload,
  })
}

export function deleteVolume(node: string, name: string) {
  return apiRequest(`/api/v1/volumes/${encodeURIComponent(name)}?node=${encodeURIComponent(node)}`, {
    method: 'DELETE',
  })
}
