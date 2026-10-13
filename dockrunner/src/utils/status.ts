export function statusLabel(status: string) {
  if (status === 'running') return 'Запущено'
  if (status === 'stopped') return 'Остановлено'
  if (status === 'error') return 'Ошибка'
  if (status === 'building') return 'Сборка'
  if (status === 'created') return 'Создан'
  return status
}

export function statusColor(status: string): 'success' | 'warning' | 'error' | 'default' {
  if (status === 'running') return 'success'
  if (status === 'building') return 'warning'
  if (status === 'error') return 'error'
  return 'default'
}
