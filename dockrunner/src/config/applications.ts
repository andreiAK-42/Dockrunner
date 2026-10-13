export interface CatalogApp {
  id: string
  name: string
  type: string
  status: string
  replicas: string
  url: string
  branch: string
  commit: string
}

export const APPLICATIONS: CatalogApp[] = [
  { id: 'app-01', name: 'frontend-web', type: 'Docker Compose', status: 'Запущено', replicas: '3/3', url: 'app.opscluster.local', branch: 'main', commit: 'a1b2c3d' },
  { id: 'app-02', name: 'payment-service', type: 'Dockerfile', status: 'Запущено', replicas: '2/2', url: 'api.opscluster.local', branch: 'main', commit: 'f3e2d1c' },
  { id: 'app-03', name: 'auth-api', type: 'GitHub Repo', status: 'Остановлено', replicas: '0/1', url: 'auth.opscluster.local', branch: 'main', commit: '—' },
  { id: 'app-04', name: 'redis-cache', type: 'Docker Compose', status: 'Запущено', replicas: '1/1', url: 'Внутренний', branch: 'main', commit: '—' },
  { id: 'app-05', name: 'postgres-db', type: 'Docker Compose', status: 'Запущено', replicas: '1/1', url: 'Внутренний', branch: 'main', commit: '—' },
  { id: 'app-06', name: 'background-worker', type: 'Dockerfile', status: 'Обновление', replicas: '5/5', url: 'Внутренний', branch: 'main', commit: '—' },
]

function buildUnknownApp(id: string): CatalogApp {
  return {
    id,
    name: id || 'unknown',
    type: 'Docker Compose',
    status: 'Запущено',
    replicas: '1/1',
    url: '—',
    branch: 'main',
    commit: '—',
  }
}

export function getAppById(id: string): CatalogApp {
  const foundApp = APPLICATIONS.find((app) => app.id === id || app.name === id)

  if (foundApp) {
    return foundApp
  }

  return buildUnknownApp(id)
}
