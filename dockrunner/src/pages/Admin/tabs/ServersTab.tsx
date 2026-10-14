import { Box, Button, Card, CardContent, Stack, Typography, Chip } from '@mui/material'
import { RotateCcw } from 'lucide-react'
import type { WorkerNode } from '../../../types/workerNode'
import { timeAgo } from '../../../utils/time'
import { PingText } from '../../../components/Ping'
import styles from '../Admin.module.scss'

interface ServersTabProps {
  nodes: WorkerNode[]
  loading: boolean
  rebooting: string
  onReboot: (id: string) => void
}

export default function ServersTab({ nodes, loading, rebooting, onReboot }: ServersTabProps) {
  return (
    <Card>
      <CardContent>
        <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>Серверы кластера</Typography>
        {loading && <Typography color="text.secondary">Загрузка серверов...</Typography>}
        {!loading && nodes.length === 0 && (
          <Typography color="text.secondary">Нет подключённых серверов — воркеры шлют heartbeat на централь.</Typography>
        )}
        <Stack spacing={1.5}>
          {nodes.map((node) => (
            <Box key={node.id} className={styles.serverRow}>
              <Box>
                <Typography variant="body2" sx={{ fontWeight: 600 }}>{node.name} <Typography component="span" variant="caption" color="text.secondary">· {node.city}</Typography></Typography>
                {node.description && (
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>{node.description}</Typography>
                )}
                <Typography variant="caption" color="text.secondary">
                  CPU: {node.cpu.total} ядер {node.cpu.mhz ? `· ${node.cpu.mhz} МГц` : ''} · RAM: {node.memory.total} ГБ {node.memory.type ? `· ${node.memory.type}` : ''} {node.memory.speedMhz ? `· ${node.memory.speedMhz} МГц` : ''} · Диск: {node.disk.total} ГБ {node.disk.type ? `· ${node.disk.type}` : ''}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Heartbeat: {timeAgo(node.lastSeen)} · Пинг: <PingText url={node.healthUrl} />
                </Typography>
              </Box>
              <Stack direction="row" spacing={1} alignItems="center">
                <Chip size="small" label={node.status === 'online' ? 'Онлайн' : 'Офлайн'} color={node.status === 'online' ? 'success' : 'default'} />
                <Button size="small" variant="outlined" startIcon={<RotateCcw size={14} />} disabled={rebooting === node.id || node.status !== 'online'} onClick={() => onReboot(node.id)}>
                  {rebooting === node.id ? 'Перезагрузка...' : 'Перезагрузить'}
                </Button>
              </Stack>
            </Box>
          ))}
        </Stack>
      </CardContent>
    </Card>
  )
}
