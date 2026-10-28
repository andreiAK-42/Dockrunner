export const IN_PROGRESS_STATUSES = new Set([
  'building',
  'pending',
  'deploying',
  'creating',
  'starting',
  'pulling',
  'preparing',
  'queued',
])

export function statusLabel(status: string) {
  if (status === 'running') return 'Запущено'
  if (status === 'stopped') return 'Остановлено'
  if (status === 'error') return 'Ошибка'
  if (IN_PROGRESS_STATUSES.has(status)) return 'Сборка'
  if (status === 'created') return 'Создан'
  return status
}

export function statusColor(status: string): 'success' | 'warning' | 'error' | 'default' {
  if (status === 'running') return 'success'
  if (IN_PROGRESS_STATUSES.has(status)) return 'warning'
  if (status === 'error') return 'error'
  return 'default'
}
