import { apiRequest } from './client'

export type IntegrationId = 'postgres' | 'redis' | 'mongo' | 'ollama'
export type IntegrationGroup = 'database' | 'ai' | string

export interface IntegrationKind {
  id: IntegrationId
  group: IntegrationGroup
  name: string
  defaultPort: string
  description: string
  hasQuota: boolean
  enabled: boolean
}

export interface CredentialRow {
  label: string
  value: string
}

export interface IntegrationAccount {
  id: string
  ownerLogin: string
  kind: IntegrationId
  resource: string
  username: string
  host: string
  port: number
  sizeGB: number
  createdAt: string
}

export interface IntegrationResult {
  user: IntegrationAccount
  rows: CredentialRow[]
  connection_string: string | null
  size_gb?: number
}

export interface IntegrationUsersResponse {
  users: IntegrationAccount[]
  usage_gb: number
  limit_gb: number
}

export async function fetchIntegrations(): Promise<IntegrationKind[]> {
  const res = await apiRequest<{ data: IntegrationKind[] }>('/api/v1/integrations')
  return res.data
}

export async function fetchMyIntegrationAccounts(): Promise<IntegrationUsersResponse> {
  const res = await apiRequest<{ data: IntegrationUsersResponse }>('/api/v1/integrations/users')
  return res.data
}

export async function createIntegrationAccount(kind: IntegrationId): Promise<IntegrationResult> {
  const res = await apiRequest<{ data: IntegrationResult }>('/api/v1/integrations/users', {
    method: 'POST',
    body: { kind },
  })
  return res.data
}

export async function resetIntegrationPassword(id: string): Promise<IntegrationResult> {
  const res = await apiRequest<{ data: IntegrationResult }>(`/api/v1/integrations/users/${id}/reset-password`, {
    method: 'POST',
  })
  return res.data
}

export function deleteIntegrationAccount(id: string) {
  return apiRequest(`/api/v1/integrations/users/${id}`, { method: 'DELETE' })
}
