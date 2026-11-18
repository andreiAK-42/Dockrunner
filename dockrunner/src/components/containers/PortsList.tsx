import { Box, Button, Card, CardContent, Typography, Chip, Stack, Tooltip } from '@mui/material'
import { ExternalLink } from 'lucide-react'
import type { ContainerSpec } from '../../api/containers'

interface PortsListProps {
  spec: ContainerSpec | null
  record: { node_ip?: string; node_id?: string } | null
}

export default function PortsList({ spec, record }: PortsListProps) {
  const ports = spec?.ports ?? []
  if (ports.length === 0) {
    return (
      <Card>
        <CardContent>
          <Typography variant="h6" sx={{ fontWeight: 650, mb: 2 }}>
            Порты
          </Typography>
          <Typography color="text.secondary">Нет проброшенных портов</Typography>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardContent>
        <Typography variant="h6" sx={{ fontWeight: 650, mb: 2 }}>
          Порты
        </Typography>
        <Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: 'wrap' }}>
          {ports.map((p) => (
            <Box key={`${p.host}-${p.container}`} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Chip size="small" label={`${p.host} → ${p.container}`} color="primary" variant="outlined" />
              {p.host === ports[0].host && (
                <a href={`http://${record?.node_ip || record?.node_id}:${p.host}`} target="_blank" rel="noopener noreferrer">
                  <Tooltip title="Открыть в браузере">
                    <Button size="small" startIcon={<ExternalLink size={14} />} variant="outlined">
                      Открыть
                    </Button>
                  </Tooltip>
                </a>
              )}
            </Box>
          ))}
        </Stack>
      </CardContent>
    </Card>
  )
}