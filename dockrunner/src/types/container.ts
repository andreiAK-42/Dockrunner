export type ContainerStatus = 'running' | 'stopped' | 'restarting' | 'created' | 'error'

export interface ContainerPort {
  hostPort: number
  containerPort: number
  protocol: 'tcp' | 'udp'
}

export interface ContainerVolume {
  id: string
  name: string
  mountPath: string
  sizeGB: number
  readOnly?: boolean
}

export interface ContainerEnv {
  key: string
  value: string
}

export interface Container {
  id: string
  name: string
  appId: string
  image: string
  status: ContainerStatus
  nodeId: string
  ports: ContainerPort[]
  volumes: ContainerVolume[]
  env: ContainerEnv[]
  envConfigId?: string
  command?: string
  restartPolicy: 'no' | 'always' | 'on-failure' | 'unless-stopped'
  createdAt: string
  logs: string[]
}

export const MOCK_CONTAINERS: Container[] = [
  {
    id: 'ctr-01', name: 'frontend-web-1', appId: 'app-01', image: 'nginx:1.27', status: 'running', nodeId: 'worker-01',
    ports: [{ hostPort: 3000, containerPort: 80, protocol: 'tcp' }],
    volumes: [{ id: 'vol-01', name: 'uploads', mountPath: '/usr/share/nginx/html', sizeGB: 10 }],
    env: [{ key: 'NODE_ENV', value: 'production' }],
    command: 'nginx -g daemon off;',
    restartPolicy: 'unless-stopped', createdAt: '2026-09-01T10:00:00Z',
    logs: [
      '2026-09-06 12:00:01 [notice] start worker process',
      '2026-09-06 12:00:02 [info] listening on 0.0.0.0:80',
      '2026-09-06 12:01:10 GET /api/health 200 2ms',
      '2026-09-06 12:02:44 GET / 200 12ms',
    ]
  },
  {
    id: 'ctr-02', name: 'frontend-web-2', appId: 'app-01', image: 'nginx:1.27', status: 'running', nodeId: 'worker-03',
    ports: [{ hostPort: 3001, containerPort: 80, protocol: 'tcp' }],
    volumes: [{ id: 'vol-02', name: 'uploads-2', mountPath: '/usr/share/nginx/html', sizeGB: 10 }],
    env: [{ key: 'NODE_ENV', value: 'production' }],
    restartPolicy: 'unless-stopped', createdAt: '2026-09-01T10:00:00Z',
    logs: ['2026-09-06 12:00:03 [notice] worker started', '2026-09-06 12:05:00 GET /assets/main.js 200']
  },
  {
    id: 'ctr-03', name: 'frontend-web-3', appId: 'app-01', image: 'nginx:1.27', status: 'running', nodeId: 'worker-04',
    ports: [{ hostPort: 3002, containerPort: 80, protocol: 'tcp' }],
    volumes: [], env: [], restartPolicy: 'always', createdAt: '2026-09-01T10:00:00Z',
    logs: ['2026-09-06 12:00:04 [notice] worker started']
  },
  {
    id: 'ctr-04', name: 'payment-service-1', appId: 'app-02', image: 'payment-api:2.1', status: 'running', nodeId: 'worker-02',
    ports: [{ hostPort: 3003, containerPort: 8080, protocol: 'tcp' }],
    volumes: [{ id: 'vol-03', name: 'pg-data', mountPath: '/var/lib/postgresql/data', sizeGB: 20 }],
    env: [{ key: 'DATABASE_URL', value: 'postgres://***' }, { key: 'PORT', value: '8080' }],
    command: 'node server.js',
    restartPolicy: 'always', createdAt: '2026-09-02T08:00:00Z',
    logs: [
      '2026-09-06 12:00:00 payment-service listening on :8080',
      '2026-09-06 12:10:12 [db] connected',
      '2026-09-06 12:15:33 POST /charge 200 44ms',
      '2026-09-06 12:18:01 ERROR timeout upstream',
    ]
  },
  {
    id: 'ctr-05', name: 'payment-service-2', appId: 'app-02', image: 'payment-api:2.1', status: 'error', nodeId: 'worker-02',
    ports: [{ hostPort: 3004, containerPort: 8080, protocol: 'tcp' }],
    volumes: [], env: [{ key: 'PORT', value: '8080' }], restartPolicy: 'on-failure', createdAt: '2026-09-02T08:00:00Z',
    logs: ['2026-09-06 12:00:00 starting...', '2026-09-06 12:00:05 panic: cannot connect to db', '2026-09-06 12:00:06 restarting...']
  },
  {
    id: 'ctr-06', name: 'auth-api-1', appId: 'app-03', image: 'auth:1.0', status: 'stopped', nodeId: 'worker-01',
    ports: [], volumes: [], env: [], restartPolicy: 'no', createdAt: '2026-09-03T09:00:00Z',
    logs: ['2026-09-06 11:00:00 stopped by user']
  },
  {
    id: 'ctr-07', name: 'redis-cache-1', appId: 'app-04', image: 'redis:7', status: 'running', nodeId: 'worker-05',
    ports: [{ hostPort: 6379, containerPort: 6379, protocol: 'tcp' }],
    volumes: [{ id: 'vol-04', name: 'redis-data', mountPath: '/data', sizeGB: 5 }],
    env: [], restartPolicy: 'always', createdAt: '2026-08-28T07:00:00Z',
    logs: ['1:M 06 Sep 12:00:00 * Ready to accept connections']
  },
]

export function getContainersByApp(appId: string): Container[] {
  return MOCK_CONTAINERS.filter((container) => container.appId === appId)
}

export function getContainerById(id: string): Container | undefined {
  return MOCK_CONTAINERS.find((container) => container.id === id)
}
