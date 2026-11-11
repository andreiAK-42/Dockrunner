import { Stack, TextField, FormControl, InputLabel, Select, MenuItem, IconButton } from '@mui/material'
import { Trash2 } from 'lucide-react'
import { parsePort } from '../../utils/ports'
import type { PortInfo } from '../../hooks/usePorts'

export interface PortRowProps {
  row: { host: string; container: string }
  index: number
  suggestions: number[]
  busyPorts: PortInfo[]
  portsRange: { min: number; max: number }
  effectiveNodeId: string
  onUpdate: (patch: Partial<{ host: string; container: string }>) => void
  onRemove: () => void
  disabled?: boolean
  isLast?: boolean
}

export default function PortRow({
  row,
  suggestions,
  onUpdate,
  onRemove,
  disabled = false,
  isLast = false,
}: PortRowProps) {
  const currentHost = parsePort(row.host)
  const options = currentHost !== null && !suggestions.includes(currentHost)
    ? [currentHost, ...suggestions]
    : suggestions

  return (
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ alignItems: { sm: 'flex-end' } }}>
      <TextField
        size="small"
        label="Порт контейнера"
        placeholder="80"
        value={row.container}
        onChange={(e) => onUpdate({ container: e.target.value })}
        sx={{ flex: 1 }}
        disabled={disabled}
      />
      <FormControl size="small" sx={{ flex: 1 }}>
        <InputLabel>Хост-порт</InputLabel>
        <Select
          value={row.host}
          label="Хост-порт"
          onChange={(e) => onUpdate({ host: e.target.value })}
          disabled={disabled}
        >
          {row.host === '' && <MenuItem value=""><em>— Выберите —</em></MenuItem>}
          {options.map((port) => (
            <MenuItem key={port} value={String(port)}>{port}</MenuItem>
          ))}
        </Select>
      </FormControl>
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