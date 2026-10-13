import { apiRequest } from './client'

export interface ApiAgent {
  id: string
  name: string
  description: string
  city: string
  ip: string
  lat: number
  lng: number
  api_url: string
  cpu_total: number
  cpu_percent: number
  cpu_mhz?: number
  mem_used_gb: number
  mem_total_gb: number
  mem_type?: string
  mem_speed_mhz?: number
  disk_used_gb: number
  disk_total_gb: number
  disk_type?: string
  containers: number
  net_down_mbps: number
  net_up_mbps: number
  last_seen: string
}

export function fetchAgents() {
  return apiRequest<{ data: ApiAgent[] }>('/api/v1/agents')
}
