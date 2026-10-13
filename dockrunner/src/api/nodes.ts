import { apiRequest } from './client'

export interface WorkerNode {
  id: string
  name: string
  city: string
  status: string
  cpu: { used: number; total: number }
  memory: { used: number; total: number }
  disk: { used: number; total: number }
  containers: number
}

export function fetchNodes() {
  return apiRequest<{ data: WorkerNode[] }>('/api/v1/servers')
}

export function fetchNode(id: string) {
  return apiRequest<{ data: WorkerNode }>(`/api/v1/servers/${id}`)
}

export function createNode(payload: { name: string; city: string }) {
  return apiRequest<{ data: WorkerNode }>('/api/v1/servers', {
    method: 'POST',
    body: payload,
  })
}

export function deleteNode(id: string) {
  return apiRequest<void>(`/api/v1/servers/${id}`, { method: 'DELETE' })
}
