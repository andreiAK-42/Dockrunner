import { apiRequest } from './client'

export interface StorageUsage {
  containers_gb: number
  files_gb: number
  total_gb: number
}

export function fetchStorageUsage() {
  return apiRequest<{ data: StorageUsage }>('/api/v1/storage/usage')
}
