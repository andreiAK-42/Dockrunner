import { Box, FormControl, InputLabel, Select, MenuItem, Typography } from '@mui/material'
import { toDockerEnvArgs } from '../../types/envConfig'
import type { EnvConfig } from '../../types/envConfig'

interface EnvConfigSelectorProps {
  configs: EnvConfig[]
  selectedEnvConfig: EnvConfig | undefined
  envConfigId: string
  onEnvConfigChange: (id: string) => void
  spec: { env: Record<string, string> } | null
}

export default function EnvConfigSelector({
  configs,
  selectedEnvConfig,
  envConfigId,
  onEnvConfigChange,
  spec,
}: EnvConfigSelectorProps) {
  return (
    <Box>
      <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
        Env-конфигурация
      </Typography>
      <FormControl fullWidth size="small">
        <InputLabel>Выбрать сохранённый env-конфиг</InputLabel>
        <Select value={envConfigId} label="Выбрать сохранённый env-конфиг" onChange={(e) => onEnvConfigChange(e.target.value)}>
          <MenuItem value=""><em>— Переменные контейнера —</em></MenuItem>
          {configs.map((c) => (
            <MenuItem key={c.id} value={c.id}>
              {c.name} ({c.vars.length} перем.)
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      {selectedEnvConfig ? (
        <Box sx={{ p: 1.5, borderRadius: 2, bgcolor: '#0f172a', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, whiteSpace: 'pre-wrap' }}>
          {toDockerEnvArgs(selectedEnvConfig)}
        </Box>
      ) : (
        <Box>
          {!spec || Object.keys(spec.env).length === 0 ? (
            <Typography variant="body2" color="text.secondary">Нет env</Typography>
          ) : (
            Object.entries(spec.env).map(([k, v]) => (
              <Box key={k} sx={{ display: 'flex', justifyContent: 'space-between', py: 0.7, borderBottom: '1px solid', borderColor: 'divider' }}>
                <Typography variant="body2" sx={{ fontWeight: 500 }}>{k}</Typography>
                <Typography variant="caption">{v}</Typography>
              </Box>
            ))
          )}
        </Box>
      )}
    </Box>
  )
}