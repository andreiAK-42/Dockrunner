import { Stack, TextField, IconButton } from '@mui/material'
import { Trash2 } from 'lucide-react'

export interface EnvRowData {
  key: string
  value: string
}

export interface EnvRowProps {
  row: EnvRowData
  index: number
  onUpdate: (patch: Partial<EnvRowData>) => void
  onRemove: () => void
  disabled?: boolean
  isLast?: boolean
}

export default function EnvRow({
  row,
  onUpdate,
  onRemove,
  disabled = false,
  isLast = false,
}: EnvRowProps) {
  return (
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ alignItems: { sm: 'flex-end' } }}>
      <TextField
        size="small"
        label="Ключ"
        placeholder="NODE_ENV"
        value={row.key}
        onChange={(e) => onUpdate({ key: e.target.value })}
        sx={{ flex: 1 }}
        disabled={disabled}
      />
      <TextField
        size="small"
        label="Значение"
        placeholder="production"
        value={row.value}
        onChange={(e) => onUpdate({ value: e.target.value })}
        sx={{ flex: 1.6 }}
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