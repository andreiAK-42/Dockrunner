import { apiRequest } from './client'

export interface PortSpec {
  host_port: number
  container_port: number
}

export interface VolumeSpec {
  name?: string
  host_path?: string
  container_path: string
}

export interface CheckStatus {
  id: string
  app_name: string
  repo_url: string
  node_id: string
  status: string
  detected_dockerfile: boolean
  compose_file: string
  services: string[]
  error: string
}

export interface PortMapping {
  host_port: number
  container_port: number
}

export interface MineContainer {
  id: string
  app_id: string
  app_name: string
  owner_login: string
  image: string
  status: string
  node_id: string
  ports: PortMapping[]
  cpu_cores?: number
  memory_gb?: number
  disk_gb?: number
  expires_at?: string | null
  updated_at: string
}

export function fetchMine() {
  return apiRequest<{ data: MineContainer[] }>('/api/v1/containers/mine')
}

export function fetchContainerLogs(id: string, tail = 200) {
  return apiRequest<{ data: string }>(`/api/v1/containers/${id}/logs?tail=${tail}`)
}

export interface ContainerStats {
  cpu_percent: number
  mem_mb: number
}

export function fetchContainerStats(id: string) {
  return apiRequest<{ data: ContainerStats }>(`/api/v1/containers/${id}/stats`)
}

export interface ContainerSpec {
  id: string
  name: string
  image: string
  status: string
  ports: { host: number; container: number }[]
  volumes: { name: string; host_path: string; container_path: string }[]
  env: Record<string, string>
}

export function fetchContainerSpec(id: string) {
  return apiRequest<{ data: ContainerSpec }>(`/api/v1/containers/${id}/spec`)
}

export function checkRepository(payload: { app_name: string; repo_url: string; node_id: string }) {
  return apiRequest<{ data: CheckStatus }>('/api/v1/containers/check', {
    method: 'POST',
    body: payload,
  })
}

export function fetchCheck(id: string) {
  return apiRequest<{ data: CheckStatus }>(`/api/v1/containers/check/${id}`)
}

export function runContainer(payload: {
  app_name: string
  repo_url?: string
  image?: string
  container_name?: string
  node_id: string
  ports: PortSpec[]
  volumes: VolumeSpec[]
  env: Record<string, string>
}) {
  return apiRequest<{ data: { deployment_id: string } }>('/api/v1/containers/run', {
    method: 'POST',
    body: payload,
  })
}

export function refreshContainerByApp(app_name: string) {
  return apiRequest<{ data: { deployment_id: string } }>('/api/v1/containers/refresh', {
    method: 'POST',
    body: { app_name },
  })
}

export function removeContainer(id: string) {
  return apiRequest<{ data: { message: string } }>(`/api/v1/containers/${id}`, {
    method: 'DELETE',
  })
}

export function startContainer(id: string) {
  return apiRequest<{ data: { message: string } }>(`/api/v1/containers/${id}/start`, {
    method: 'POST',
  })
}

export function stopContainer(id: string) {
  return apiRequest<{ data: { message: string } }>(`/api/v1/containers/${id}/stop`, {
    method: 'POST',
  })
}

export function fetchBuildLog(id: string) {
  return apiRequest<{ data: string }>(`/api/v1/containers/${id}/build-log`)
}
