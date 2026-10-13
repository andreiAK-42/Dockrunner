import { apiRequest } from './client'

export type DbId = 'postgres' | 'redis' | 'mongo'

export interface DatabaseInfo {
  id: DbId
  name: string
  defaultPort: string
  description: string
}

export interface CredentialRow {
  label: string
  value: string
}

export interface DbUser {
  id: string
  ownerLogin: string
  dbType: DbId
  dbName: string
  username: string
  host: string
  port: number
  sizeGB: number
  createdAt: string
}

export interface DbUserResult {
  user: DbUser
  rows: CredentialRow[]
  connectionString: string | null
  sizeGB: number
}

export interface DbUsersResponse {
  users: DbUser[]
  usage_gb: number
  limit_gb: number
}

export async function fetchDatabases(): Promise<DatabaseInfo[]> {
  const res = await apiRequest<{ data: DatabaseInfo[] }>('/api/v1/databases')
  return res.data
}

export async function fetchMyDbUsers(): Promise<DbUsersResponse> {
  const res = await apiRequest<{ data: DbUsersResponse }>('/api/v1/databases/users')
  return res.data
}

export async function createDatabaseUser(db: DbId): Promise<DbUserResult> {
  const res = await apiRequest<{ data: DbUserResult }>('/api/v1/databases/users', {
    method: 'POST',
    body: { db_type: db },
  })
  return res.data
}

export function deleteDatabaseUser(id: string) {
  return apiRequest(`/api/v1/databases/users/${id}`, { method: 'DELETE' })
}
