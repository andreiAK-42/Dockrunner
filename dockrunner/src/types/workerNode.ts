export type WorkerStatus = 'online' | 'offline' | 'maintenance'

export interface WorkerNode {
  id: string
  name: string
  city: string
  ip: string
  coordinates: [number, number]
  status: WorkerStatus
  cpu: { used: number; total: number; mhz?: number }
  memory: { used: number; total: number; type?: string; speedMhz?: number }
  disk: { used: number; total: number; type?: string }
  containers: number
  net: { down: number; up: number }
  lastSeen: string
  healthUrl: string
  description: string
}

export function percent(used: number, total: number) {
  if (total <= 0) {
    return 0
  }
  return Math.round((used / total) * 100)
}
