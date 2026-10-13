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

const MS_PER_MINUTE = 60000

function minutesAgo(minutes: number) {
  return new Date(Date.now() - minutes * MS_PER_MINUTE).toISOString()
}

export const WORKER_NODES: WorkerNode[] = [
  {
    id: 'worker-01',
    name: 'worker-01',
    city: 'New York',
    coordinates: [-74.006, 40.7128],
    ip: '',
    status: 'online',
    cpu: { used: 10, total: 16 },
    memory: { used: 15.4, total: 32 },
    disk: { used: 710, total: 1000 },
    containers: 24,
    net: { down: 0, up: 0 },
    lastSeen: minutesAgo(1),
    healthUrl: 'http://worker-01:8090',
    description: 'Основной сервер, восточное побережье',
  },
  {
    id: 'worker-02',
    name: 'worker-02',
    city: 'San Francisco',
    coordinates: [-122.4194, 37.7749],
    ip: '',
    status: 'online',
    cpu: { used: 5, total: 16 },
    memory: { used: 9, total: 32 },
    disk: { used: 430, total: 1000 },
    containers: 12,
    net: { down: 0, up: 0 },
    lastSeen: minutesAgo(2),
    healthUrl: 'http://worker-02:8090',
    description: 'Резервный сервер, западное побережье',
  },
  {
    id: 'worker-03',
    name: 'worker-03',
    city: 'London',
    coordinates: [-0.1276, 51.5074],
    ip: '',
    status: 'online',
    cpu: { used: 12.8, total: 16 },
    memory: { used: 29, total: 32 },
    disk: { used: 820, total: 1000 },
    containers: 31,
    net: { down: 0, up: 0 },
    lastSeen: minutesAgo(5),
    healthUrl: 'http://worker-03:8090',
    description: 'Европейский сервер, почти заполнен',
  },
  {
    id: 'worker-04',
    name: 'worker-04',
    city: 'Frankfurt',
    coordinates: [8.6821, 50.1109],
    ip: '',
    status: 'online',
    cpu: { used: 4, total: 16 },
    memory: { used: 11, total: 32 },
    disk: { used: 520, total: 1000 },
    containers: 14,
    net: { down: 0, up: 0 },
    lastSeen: minutesAgo(1),
    healthUrl: 'http://worker-04:8090',
    description: '',
  },
  {
    id: 'worker-05',
    name: 'worker-05',
    city: 'Singapore',
    coordinates: [103.8198, 1.3521],
    ip: '',
    status: 'online',
    cpu: { used: 7, total: 16 },
    memory: { used: 18, total: 32 },
    disk: { used: 640, total: 1000 },
    containers: 19,
    net: { down: 0, up: 0 },
    lastSeen: minutesAgo(3),
    healthUrl: 'http://worker-05:8090',
    description: 'Азиатский сервер',
  },
  {
    id: 'worker-06',
    name: 'worker-06',
    city: 'Tokyo',
    coordinates: [139.6917, 35.6895],
    ip: '',
    status: 'offline',
    cpu: { used: 0, total: 16 },
    memory: { used: 0, total: 32 },
    disk: { used: 380, total: 1000 },
    containers: 0,
    net: { down: 0, up: 0 },
    lastSeen: minutesAgo(180),
    healthUrl: '',
    description: 'На обслуживании',
  },
]

export function percent(used: number, total: number) {
  if (total <= 0) {
    return 0
  }
  return Math.round((used / total) * 100)
}
