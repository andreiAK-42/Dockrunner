import { Box, Card, CardContent, Typography, Stack, Chip, LinearProgress, Tooltip } from '@mui/material'
import type { SxProps, Theme } from '@mui/material'
import { Globe, AlertTriangle } from 'lucide-react'
import { usePorts } from '../hooks/usePorts'
import type { PortInfo } from '../hooks/usePorts'
import { useAgents } from '../hooks/useAgents'
import type { WorkerNode } from '../types/workerNode'

function findNodeIp(nodes: WorkerNode[], nodeId: string): string {
  const foundNode = nodes.find((node) => node.id === nodeId)
  if (foundNode && foundNode.ip) {
    return foundNode.ip
  }
  return ''
}

function formatPortAddress(nodeIp: string, port: number): string {
  if (nodeIp) {
    return `${nodeIp}:${port}`
  }
  return `:${port}`
}

function getProgressColor(percentage: number, isFull: boolean): 'error' | 'warning' | 'primary' {
  if (isFull) {
    return 'error'
  }
  if (percentage > 80) {
    return 'warning'
  }
  return 'primary'
}

function getPortTooltip(nodeIp: string, portInfo: PortInfo): string {
  if (nodeIp) {
    return `${nodeIp}:${portInfo.port} (${portInfo.containerName})`
  }
  return `Занят: ${portInfo.port} (${portInfo.containerName})`
}

export default function PortQuota({ compact = false, sx }: { compact?: boolean; sx?: SxProps<Theme> }) {
  const { ports, usedCount, freeCount, limit, isFull } = usePorts()
  const { nodes } = useAgents()

  let usagePercent = 0
  if (limit > 0) {
    usagePercent = (usedCount / limit) * 100
  }
  const progressColor = getProgressColor(usagePercent, isFull)

  if (compact) {
    const statusLabel = isFull ? 'Лимит' : `Свободно ${freeCount}`
    const statusColor = isFull ? 'error' : 'success'

    return (
      <Box sx={{ p: 1.5, border: '1px solid', borderColor: isFull ? 'error.main' : 'divider', borderRadius: 2, bgcolor: 'background.paper' }}>
        <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center', gap: 2 }}>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>Порты {usedCount}/{limit}</Typography>
          <Chip size="small" label={statusLabel} color={statusColor} />
        </Stack>
        <LinearProgress variant="determinate" value={usagePercent} color={progressColor} sx={{ mt: 1, height: 6, borderRadius: 999 }} />
        {ports.length > 0 && (
          <Box sx={{ display: 'flex', gap: 1, mt: 1, flexWrap: 'wrap' }}>
            {ports.map((portInfo) => {
              const nodeIp = findNodeIp(nodes, portInfo.nodeId)
              const tooltip = getPortTooltip(nodeIp, portInfo)
              return (
                <Tooltip key={`${portInfo.containerId}-${portInfo.port}`} title={tooltip}>
                  <Chip size="small" icon={<Globe size={12} />} label={`${portInfo.port}`} color="primary" sx={{ fontSize: 11, height: 22 }} />
                </Tooltip>
              )
            })}
          </Box>
        )}
      </Box>
    )
  }

  const headerChipLabel = `${usedCount} / ${limit} занято`
  const headerChipColor = isFull ? 'error' : 'primary'

  return (
    <Card sx={sx}>
      <CardContent>
        <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center', mb: 1.5 }}>
          <Typography variant="h6" sx={{ fontWeight: 650, display: 'flex', alignItems: 'center', gap: 1 }}><Globe size={18} /> Порты пользователя</Typography>
          <Chip label={headerChipLabel} color={headerChipColor} size="small" sx={{ margin: '10px' }} />
        </Stack>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>Лимит — {limit} пробросов на пользователя. Каждый порт занимает hostPort на воркере.</Typography>
        <LinearProgress variant="determinate" value={usagePercent} color={progressColor} sx={{ height: 8, borderRadius: 999, mb: 2 }} />
        {ports.length === 0 ? (
          <Typography variant="body2" color="text.secondary">Нет занятых портов</Typography>
        ) : (
          <Stack spacing={1}>
            {ports.map((portInfo) => {
              const nodeIp = findNodeIp(nodes, portInfo.nodeId)
              const address = formatPortAddress(nodeIp, portInfo.port)
              return (
                <Box key={`${portInfo.containerId}-${portInfo.port}`} sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 2, py: 0.7, borderBottom: '1px solid', borderColor: 'divider' }}>
                  <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: 'monospace' }}>{address}</Typography>
                  <Typography variant="caption" color="text.secondary">{portInfo.containerName}</Typography>
                </Box>
              )
            })}
          </Stack>
        )}
        {isFull && (
          <Stack direction="row" gap={1} sx={{ alignItems: 'center', justifyContent: 'flex-start', mt: 1.5, color: 'error.main' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}><AlertTriangle size={14} /></Box>
            <Typography variant="caption" sx={{ lineHeight: 1.4 }}>Лимит исчерпан — освободите порт перед созданием контейнера.</Typography>
          </Stack>
        )}
        <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center', mt: 1.5, gap: 3, px: 0.5 }}>
          <Typography variant="caption" color="text.secondary">Занято: {usedCount}</Typography>
          <Typography variant="caption" color="text.secondary">Свободно: {freeCount}</Typography>
        </Stack>
      </CardContent>
    </Card>
  )
}
