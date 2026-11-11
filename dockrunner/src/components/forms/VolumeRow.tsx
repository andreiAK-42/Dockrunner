import { Stack, TextField, IconButton } from '@mui/material'
import { Trash2 } from 'lucide-react'

export interface VolumeRowData {
  hostPath: string
  containerPath: string
}

export interface VolumeRowProps {
  row: VolumeRowData
  index: number
  onUpdate: (patch: Partial<VolumeRowData>) => void
  onRemove: () => void
  disabled?: boolean
  isLast?: boolean
}

export default function VolumeRow({
  row,
  onUpdate,
  onRemove,
  disabled = false,
  isLast = false,
}: VolumeRowProps) {
  return (
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ alignItems: { sm: 'flex-end' } }}>
      <TextField
        size="small"
        label="Хост-путь или имя"
        placeholder="data/db"
        value={row.hostPath}
        onChange={(e) => onUpdate({ hostPath: e.target.value })}
        sx={{ flex: 1 }}
        disabled={disabled}
      />
      <TextField
        size="small"
        label="Путь в контейнере"
        placeholder="/data"
        value={row.containerPath}
        onChange={(e) => onUpdate({ containerPath: e.target.value })}
        sx={{ flex: 1 }}
        disabled={disabled}
      />
      <IconButton
        size="small"
        color="error"
        onClick={onRemove}
        disabled={disabled || isLast}
      >
        <Trash2 size={16} />
      </IconButton>
    </Stack>
  )
}