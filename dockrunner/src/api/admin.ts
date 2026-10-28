import { apiRequest } from './client'

export interface AdminUser {
  id: string
  login: string
  role: string
  blocked: boolean
  isSuperuser: boolean
  planId: string
  planExpires: string | null
}

export interface TariffPlan {
  id: string
  name: string
  price: string
  apps: number
  domains: number
  ports: number
  envConfigs: number
  dbStorageGB: number
}

export interface AdminContainer {
  id: string
  app_id: string
  app_name: string
  owner_login: string
  image: string
  status: string
  node_id: string
  node_ip?: string
  ports: { host_port: number; container_port: number }[]
  volumes: { name?: string; host_path?: string; container_path: string }[]
  env: Record<string, string>
  cpu_cores?: number
  memory_gb?: number
  disk_gb?: number
  expires_at?: string | null
  restart_policy?: string
}

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
  price: number
  status: string
  comment: string
  decidedBy: string
  decidedAt: string | null
  deploymentId: string
  createdAt: string
  updatedAt: string
}

export interface PricingRate {
  agent_id: string
  cpu_per_core_hour: number
  mem_per_gb_hour: number
  disk_per_gb_hour: number
  updated_at: string
}

export interface BalanceTransaction {
  id: string
  ownerLogin: string
  amount: number
  balanceAfter: number
  reason: string
  createdAt: string
}

interface ApiUser {
  id: string
  login: string
  role: string
  blocked: boolean
  is_superuser: boolean
  plan_id: string
  plan_expires: string | null
}

function toAdminUser(u: ApiUser): AdminUser {
  return {
    id: u.id,
    login: u.login,
    role: u.role,
    blocked: u.blocked,
    isSuperuser: !!u.is_superuser,
    planId: u.plan_id,
    planExpires: u.plan_expires,
  }
}

export async function fetchAdminUsers(): Promise<AdminUser[]> {
  const res = await apiRequest<{ data: ApiUser[] }>('/api/v1/users')
  return res.data.map(toAdminUser)
}

export function createAdminUser(payload: { login: string; password: string; role: string; planId: string }) {
  return apiRequest('/api/v1/users', {
    method: 'POST',
    body: { login: payload.login, password: payload.password, role: payload.role, plan_id: payload.planId },
  })
}

export function setUserBlocked(id: string, blocked: boolean) {
  return apiRequest(`/api/v1/users/${id}/block`, {
    method: 'PATCH',
    body: { blocked },
  })
}

export function setUserPassword(id: string, password: string) {
  return apiRequest(`/api/v1/users/${id}/password`, {
    method: 'PATCH',
    body: { password },
  })
}

export function setUserPlan(id: string, planId: string, expires: string | null) {
  return apiRequest(`/api/v1/users/${id}/plan`, {
    method: 'PATCH',
    body: { plan_id: planId, expires },
  })
}

export function fetchTariffPlans() {
  return apiRequest<{ data: TariffPlan[] }>('/api/v1/admin/plans')
}

export async function fetchAdminContainers(): Promise<AdminContainer[]> {
  const res = await apiRequest<{ data: AdminContainer[] }>('/api/v1/admin/containers')
  return res.data
}

export function adminContainerAction(id: string, action: 'stop' | 'restart' | 'remove') {
  return apiRequest(`/api/v1/admin/containers/${id}/${action}`, { method: 'POST' })
}

export function rebootServer(id: string) {
  return apiRequest(`/api/v1/admin/servers/${id}/reboot`, { method: 'POST' })
}

export function adminUpdateContainerParams(id: string, params: {
  image?: string
  cpu_cores?: number
  memory_gb?: number
  disk_gb?: number
  ports?: { host_port: number; container_port: number }[]
  env?: Record<string, string>
  volumes?: { name?: string; host_path?: string; container_path: string }[]
  restart_policy?: string
  expires_at?: string | null
}) {
  return apiRequest(`/api/v1/admin/containers/${id}/params`, {
    method: 'PUT',
    body: params,
  })
}

// Deployment Requests
export async function fetchPendingRequests(): Promise<DeploymentRequest[]> {
  const res = await apiRequest<{ data: DeploymentRequest[] }>('/api/v1/admin/deployment-requests/pending')
  return res.data
}

export async function fetchAllRequests(): Promise<DeploymentRequest[]> {
  const res = await apiRequest<{ data: DeploymentRequest[] }>('/api/v1/admin/deployment-requests')
  return res.data
}

export function approveRequest(id: string) {
  return apiRequest<{ data: { message: string; deployment_id: string } }>(`/api/v1/admin/deployment-requests/${id}/approve`, {
    method: 'POST',
  })
}

export function rejectRequest(id: string, comment: string) {
  return apiRequest(`/api/v1/admin/deployment-requests/${id}/reject`, {
    method: 'POST',
    body: { comment },
  })
}

export function setRequestPrice(id: string, price: number) {
  return apiRequest(`/api/v1/admin/deployment-requests/${id}/price`, {
    method: 'PATCH',
    body: { price },
  })
}

// Pricing
export async function fetchPricing(): Promise<PricingRate[]> {
  const res = await apiRequest<{ data: PricingRate[] }>('/api/v1/pricing')
  return res.data
}

export function setPricing(payload: { agent_id: string; cpu_per_core_hour: number; mem_per_gb_hour: number; disk_per_gb_hour: number }) {
  return apiRequest('/api/v1/admin/pricing', {
    method: 'PUT',
    body: payload,
  })
}

// Balances
export async function fetchUserBalance(login: string): Promise<number> {
  const res = await apiRequest<{ data: number }>(`/api/v1/admin/balances/${login}`)
  return res.data
}

export function addUserBalance(login: string, amount: number, reason: string) {
  return apiRequest(`/api/v1/admin/balances/${login}/add`, {
    method: 'POST',
    body: { amount, reason },
  })
}

export function setUserBalance(login: string, balance: number) {
  return apiRequest(`/api/v1/admin/balances/${login}`, {
    method: 'PATCH',
    body: { balance },
  })
}

export async function fetchAllTransactions(page = 1, limit = 20): Promise<{ data: BalanceTransaction[]; total: number }> {
  const res = await apiRequest<{ data: BalanceTransaction[]; total: number; page: number; limit: number }>(`/api/v1/admin/balances/transactions?page=${page}&limit=${limit}`)
  return { data: res.data, total: res.total }
}