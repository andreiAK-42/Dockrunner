import { apiRequest } from './client'

export interface Quotas {
  apps_used: number
  apps_limit: number
  ports_used: number
  ports_limit: number
  ports_min: number
  ports_max: number
  env_used: number
  env_limit: number
  storage_gb_used: number
  storage_gb_limit: number
}

export function fetchQuotas() {
  return apiRequest<{ data: Quotas }>('/api/v1/quotas')
}
