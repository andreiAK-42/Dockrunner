import { Box, Card, CardContent, Stack, Typography, Button, Grid } from '@mui/material'
import { ExternalLink, Terminal, Activity } from 'lucide-react'
import { VICTORIA_LOGS_URL, VICTORIA_METRICS_URL } from '../../../config/appConfig'
import styles from '../Admin.module.scss'

interface LogServiceCardProps {
  title: string
  description: string
  url: string
  icon: React.ReactNode
  color: string
}

function LogServiceCard({ title, description, url, icon, color }: LogServiceCardProps) {
  const handleOpen = () => {
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  return (
    <Card sx={{ height: '100%', border: `2px solid ${color}40`, transition: 'all 0.2s', '&:hover': { borderColor: color, boxShadow: `0 4px 20px ${color}30` } }}>
      <CardContent sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center', mb: 2 }}>
          <Box
            sx={{
              p: 1.5,
              borderRadius: 2,
              bgcolor: `${color}20`,
              color: color,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {icon}
          </Box>
          <Box sx={{ flexGrow: 1 }}>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 0.5 }}>{title}</Typography>
            <Typography variant="body2" color="text.secondary">{description}</Typography>
          </Box>
        </Stack>

        

        <Box sx={{ mt: 'auto' }}>
          <Button
            variant="contained"
            startIcon={<ExternalLink size={16} />}
            fullWidth
            onClick={handleOpen}
            sx={{
              borderRadius: 2,
              py: 1.2,
              fontWeight: 600,
              bgcolor: color,
              '&:hover': { opacity: 0.9 },
            }}
          >
            Открыть Web UI
          </Button>
        </Box>
      </CardContent>
    </Card>
  )
}

export default function LogsTab() {
  return (
    <Box className={styles.page}>
      <Box className={styles.header}>
        <Box>
          <Typography variant="h4" className={styles.header__title}>Логи и метрики</Typography>
          <Typography color="text.secondary">Доступ к веб-интерфейсам VictoriaLogs и VictoriaMetrics для просмотра логов и метрик кластера.</Typography>
        </Box>
      </Box>

      <Grid container spacing={3} sx={{ mt: 2 }}>
        <Grid size={{ xs: 12, md: 6 }}>
          <LogServiceCard
            title="VictoriaLogs"
            description="Централизованное хранение и поиск логов всех контейнеров и сервисов. Поддерживает LogQL, SQL-подобный язык запросов."
            url={VICTORIA_LOGS_URL}
            icon={<Terminal size={24} />}
            color="#e67e22"
          />
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          <LogServiceCard
            title="VictoriaMetrics"
            description="Высокопроизводительная система мониторинга и алертинга. Prometheus-совместимый API, MetricsQL для запросов."
            url={VICTORIA_METRICS_URL}
            icon={<Activity size={24} />}
            color="#27ae60"
          />
        </Grid>
      </Grid>
    </Box>
  )
}