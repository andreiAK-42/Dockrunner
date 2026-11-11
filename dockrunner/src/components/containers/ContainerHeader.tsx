import { Box, Typography, Chip, Stack } from '@mui/material'
import { statusLabel } from '../../utils/status'
import type { MineContainer, ContainerSpec } from '../../api/containers'

interface ContainerHeaderProps {
  record: MineContainer | null
  spec: ContainerSpec | null
  title?: string
  onBack: () => void
  backPath?: string
  backLabel?: string
}

export default function ContainerHeader({
  record,
  spec,
  title,
  onBack,
  backLabel,
}: ContainerHeaderProps) {
  return (
    <Box className="container-header">
      {onBack && (
        <Box sx={{ mb: 2 }}>
          <button onClick={onBack} style={{ display: 'flex', alignItems: 'center', gap: '8px', background: 'none', border: 'none', cursor: 'pointer', color: 'inherit', fontSize: '1rem' }}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="19" y1="12" x2="5" y2="12"></line><polyline points="12 19 5 12 12 5"></polyline></svg>
            {backLabel || 'Назад'}
          </button>
        </Box>
      )}

      <Box className="header-content">
        <Box>
          <Typography variant="h4" className="header-title">
            {title || record?.app_name || '—'}
            {record && (
              <Chip
                size="small"
                color={record.status === 'running' ? 'success' : record.status === 'error' ? 'error' : 'default'}
                label={statusLabel(record.status)}
                sx={{ ml: 1 }}
              />
            )}
          </Typography>
          <Stack direction="row" spacing={1} className="header-meta" sx={{ alignItems: 'center' }}>
            <Typography variant="body2" color="text.secondary">
              {spec?.image || record?.image || '—'} • {record?.node_id || '—'}
            </Typography>
          </Stack>
        </Box>
      </Box>
    </Box>
  )
}