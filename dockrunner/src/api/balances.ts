import { apiRequest } from './client'

export function fetchMyBalance() {
  return apiRequest<{ data: number }>('/api/v1/balances/me')
}
