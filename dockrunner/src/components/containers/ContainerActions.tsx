import { Button, Stack } from '@mui/material'
import { RefreshCw, Play, Square, Download, Trash2 } from 'lucide-react'
import type { MineContainer } from '../../api/containers'

interface ContainerActionsProps {
  record: MineContainer | null
  busy: boolean
  syncing: boolean
  updating: boolean
  deleting: boolean
  confirmDelete: boolean
  acting: '' | 'start' | 'stop' | 'restart'
  showRefresh?: boolean
  showDelete?: boolean
  onVerifyStatus: () => void
  onRestart: () => void
  onRefresh: () => void
  onStart: () => void
  onStop: () => void
  onDelete: () => void
}

export default function ContainerActions({
  record,
  busy,
  syncing,
  updating,
  deleting,
  confirmDelete,
  acting,
  showRefresh = true,
  showDelete = true,
  onVerifyStatus,
  onRestart,
  onRefresh,
  onStart,
  onStop,
  onDelete,
}: ContainerActionsProps) {
  const disabled = !record || busy

  return (
    <Stack direction="row" spacing={1} className="header-actions">
      <Button
        variant="outlined"
        startIcon={<RefreshCw size={16} />}
        onClick={onVerifyStatus}
        disabled={disabled || syncing}
      >
        {syncing ? 'Проверка...' : 'Проверить статус'}
      </Button>
      <Button
        variant="outlined"
        startIcon={<RefreshCw size={16} />}
        disabled={disabled}
        onClick={onRestart}
      >
        {acting === 'restart' ? 'Перезапуск...' : 'Перезапустить'}
      </Button>
      {showRefresh && (
        <Button
          variant="outlined"
          startIcon={<Download size={16} />}
          disabled={disabled}
          onClick={onRefresh}
        >
          {updating ? 'Обновление...' : 'Обновить'}
        </Button>
      )}
      <Button
        variant="contained"
        color="success"
        startIcon={<Play size={16} />}
        sx={{ color: '#fff' }}
        disabled={disabled}
        onClick={onStart}
      >
        {acting === 'start' ? 'Запуск...' : 'Старт'}
      </Button>
      <Button
        variant="contained"
        color="error"
        startIcon={<Square size={16} />}
        sx={{ color: '#fff' }}
        disabled={disabled}
        onClick={onStop}
      >
        {acting === 'stop' ? 'Остановка...' : 'Стоп'}
      </Button>
      {showDelete && (
        <Button
          variant="outlined"
          color="error"
          startIcon={<Trash2 size={16} />}
          disabled={disabled}
          onClick={onDelete}
        >
          {deleting ? 'Удаление...' : confirmDelete ? 'Точно удалить?' : 'Удалить'}
        </Button>
      )}
    </Stack>
  )
}