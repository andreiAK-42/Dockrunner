import { apiRequest } from './client'

export interface DeploymentRequest {
  id: string
  ownerLogin: string
  appName: string
  repoUrl: string
  image: string
  containerName: string
  nodeId: string
  ports: { host_port: number; container_port: number }[]
  volumes: { name?: string; host_path?: string; container_path: string }[]
  env: Record<string, string>
  cpuCores: number
  memoryGB: number
  diskGB: number
  durationHours: number
  services: string[]
  buildMode?: 'dockerfile' | 'compose'
  branch?: string
  price: number
  status: string
  comment: string
  decidedBy: string
  decidedAt: string | null
  deploymentId: string
  createdAt: string
  updatedAt: string
}

export function createDeploymentRequest(payload: {
  app_name: string
  repo_url?: string
  image?: string
  container_name?: string
  node_id: string
  ports: { host_port: number; container_port: number }[]
  volumes: { name?: string; host_path?: string; container_path: string }[]
  env: Record<string, string>
  cpu_cores: number
  memory_gb: number
  disk_gb: number
  duration_hours: number
  services: string[]
  build_mode: 'dockerfile' | 'compose'
  branch?: string
}) {
  return apiRequest<{ data: DeploymentRequest }>('/api/v1/deployment-requests', {
    method: 'POST',
    body: payload,
  })
}

export async function fetchDeploymentRequest(id: string): Promise<DeploymentRequest> {
  const res = await apiRequest<{ data: DeploymentRequest }>(`/api/v1/deployment-requests/${id}`)
  return res.data
}

export async function fetchMyDeploymentRequests(): Promise<DeploymentRequest[]> {
  const res = await apiRequest<{ data: DeploymentRequest[] }>('/api/v1/deployment-requests/mine')
  return res.data
}