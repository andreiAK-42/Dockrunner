import { apiRequest } from './client'

export interface ResourceDay {
  day: string
  cpu: number
  ram: number
}

export function fetchResources(days = 14) {
  return apiRequest<{ data: ResourceDay[] }>(`/api/v1/overview/resources?days=${days}`)
}
