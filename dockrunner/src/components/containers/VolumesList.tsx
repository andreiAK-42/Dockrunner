import { Box, Card, CardContent, Typography, Stack } from '@mui/material'
import type { ContainerSpec } from '../../api/containers'

interface VolumesListProps {
  spec: ContainerSpec | null
}

export default function VolumesList({ spec }: VolumesListProps) {
  if (!spec || spec.volumes.length === 0) {
    return (
      <Card>
        <CardContent>
          <Typography variant="h6" sx={{ fontWeight: 650, mb: 2 }}>
            Тома
          </Typography>
          <Typography color="text.secondary">Нет томов у этого приложения</Typography>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardContent>
        <Typography variant="h6" sx={{ fontWeight: 650, mb: 2 }}>
          Тома
        </Typography>
        <Stack spacing={1}>
          {spec.volumes.map((v, i) => (
            <Box key={i} sx={{ display: 'flex', justifyContent: 'space-between', py: 0.7, borderBottom: '1px solid', borderColor: 'divider' }}>
              <Typography variant="body2" sx={{ fontWeight: 500 }}>
                {v.name || v.host_path}
              </Typography>
              <Typography variant="caption">{v.container_path}</Typography>
            </Box>
          ))}
        </Stack>
      </CardContent>
    </Card>
  )
}