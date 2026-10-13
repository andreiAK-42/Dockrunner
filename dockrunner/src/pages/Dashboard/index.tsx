import { useEffect, useState } from 'react'
import { Box, Grid, Card, CardContent, Typography, Stack, Alert, Chip, FormControl, InputLabel, Select, MenuItem, Button, CircularProgress } from '@mui/material'
import { RefreshCw } from 'lucide-react'
import { LineChart, Gauge, gaugeClasses } from '@mui/x-charts'
import { Box as BoxIcon, Server, Wallet, HardDrive } from 'lucide-react'
import ServerMap from '../../components/ServerMap'
import { useAgents } from '../../hooks/useAgents'
import { fetchMine, fetchContainerStats } from '../../api/containers'
import type { MineContainer, ContainerStats } from '../../api/containers'
import { fetchResources } from '../../api/overview'
import type { ResourceDay } from '../../api/overview'
import { fetchStorageUsage } from '../../api/storage'
import type { StorageUsage } from '../../api/storage'
import { fetchMyBalance } from '../../api/balances'
import { TARIFF } from '../../config/tariff'
import { timeAgo } from '../../utils/time'
import { statusLabel, statusColor } from '../../utils/status'
import UserQuotas from '../../components/UserQuotas'
import PortQuota from '../../components/PortQuota'
import styles from './Dashboard.module.scss'

function distinctApps(containers: MineContainer[]) {
  const appNames = containers.map((container) => container.app_name)
  return new Set(appNames).size
}

function sortByUpdatedDesc(containers: MineContainer[]) {
  const sorted = [...containers].sort((first, second) => {
    const firstTime = new Date(first.updated_at).getTime()
    const secondTime = new Date(second.updated_at).getTime()
    return secondTime - firstTime
  })
  return sorted
}

const RESOURCES_DAYS = 14
const ACTIVITY_LIMIT = 8

export default function Dashboard() {
  const { nodes, loading: nodesLoading, error: nodesError } = useAgents()
  const [containers, setContainers] = useState<MineContainer[]>([])
  const [resources, setResources] = useState<ResourceDay[]>([])
  const [storage, setStorage] = useState<StorageUsage | null>(null)
  const [balance, setBalance] = useState<number | null>(null)
  const [error, setError] = useState('')
  const [statsContainerId, setStatsContainerId] = useState('')
  const [stats, setStats] = useState<ContainerStats | null>(null)
  const [statsLoading, setStatsLoading] = useState(false)
  const [statsError, setStatsError] = useState('')

  useEffect(() => {
    Promise.allSettled([fetchMine(), fetchResources(RESOURCES_DAYS), fetchStorageUsage(), fetchMyBalance()]).then(([mineResult, resourcesResult, usageResult, balanceResult]) => {
      if (mineResult.status === 'fulfilled') {
        setContainers(mineResult.value.data)
      } else {
        setError('Не удалось загрузить контейнеры')
      }

      if (resourcesResult.status === 'fulfilled') {
        setResources(resourcesResult.value.data)
      }

      if (usageResult.status === 'fulfilled') {
        setStorage(usageResult.value.data)
      }

      if (balanceResult.status === 'fulfilled') {
        setBalance(balanceResult.value.data)
      }
    })
  }, [])

  const onlineNodes = nodes.filter((node) => node.status === 'online')
  const usedGB = storage?.total_gb || 0
  const gaugeValue = Math.min(100, (usedGB / TARIFF.dbStorageGB) * 100)
  const myCpu = containers.reduce((sum, c) => sum + (c.cpu_cores || 0), 0)
  const myRam = containers.reduce((sum, c) => sum + (c.memory_gb || 0), 0)

  const sortedContainers = sortByUpdatedDesc(containers)
  const activity = sortedContainers.slice(0, ACTIVITY_LIMIT)
  const statsContainer = containers.find((c) => c.id === statsContainerId) || null

  const loadStats = (id: string) => {
    if (!id) {
      return
    }
    setStatsLoading(true)
    setStatsError('')
    fetchContainerStats(id)
      .then((res) => setStats(res.data))
      .catch(() => {
        setStats(null)
        setStatsError('Не удалось получить статистику — контейнер остановлен или узел недоступен')
      })
      .finally(() => setStatsLoading(false))
  }

  const handleSelectStatsContainer = (id: string) => {
    setStatsContainerId(id)
    setStats(null)
    loadStats(id)
  }

  const cards = [
    { title: 'Мои приложения', value: String(distinctApps(containers)), icon: <BoxIcon size={24} />, color: '#3b82f6' },
    { title: 'Мой баланс', value: balance === null ? '…' : `${balance.toFixed(2)} ₽`, icon: <Wallet size={24} />, color: '#8b5cf6' },
    { title: 'Узлов онлайн', value: String(onlineNodes.length), icon: <Server size={24} />, color: '#10b981' },
    { title: 'Моё место', value: `${usedGB.toFixed(1)} ГБ`, icon: <HardDrive size={24} />, color: '#f59e0b' },
  ]

  return (
    <div className={styles.page}>
      {error && <Alert severity="error">{error}</Alert>}
      <Grid container spacing={3}>
        {cards.map((card) => (
          <Grid key={card.title} size={{ xs: 12, sm: 6, md: 3 }}>
            <Card sx={{ height: '100%' }}>
              <CardContent>
                <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
                  <Box sx={{ p: 1.5, borderRadius: 2, bgcolor: `${card.color}20`, color: card.color, display: 'flex' }}>{card.icon}</Box>
                  <Box>
                    <Typography color="text.secondary" variant="body2" sx={{ fontWeight: 500, mb: 0.5 }}>{card.title}</Typography>
                    <Typography variant="h4" sx={{ fontWeight: 700 }}>{card.value}</Typography>
                  </Box>
                </Stack>
              </CardContent>
            </Card>
          </Grid>
        ))}
      </Grid>

      <Grid container spacing={3}>
        <Grid size={{ xs: 12, lg: 8 }}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>Использование ресурсов кластера</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>Среднесуточная загрузка всех серверов за 14 дней — общая, не только ваши контейнеры</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Моё сейчас: {containers.length} конт. · CPU {myCpu.toFixed(1)} ядер · RAM {myRam.toFixed(1)} ГБ</Typography>
              {resources.length === 0 ? (
                <Typography color="text.secondary">Пока нет данных — метрики появятся после первых heartbeat воркеров.</Typography>
              ) : (
                <div className={styles.chart}>
                  <LineChart
                    series={[
                      { data: resources.map((r) => r.cpu), label: 'CPU %', color: '#3b82f6', curve: 'linear' },
                      { data: resources.map((r) => r.ram), label: 'RAM %', color: '#10b981', curve: 'linear' },
                    ]}
                    xAxis={[{ scaleType: 'point', data: resources.map((r) => r.day) }]}
                    margin={{ top: 20, bottom: 30, left: 40, right: 20 }}
                    sx={{ '.MuiLineElement-root': { strokeWidth: 3 }, '.MuiMarkElement-root': { display: 'none' } }}
                  />
                </div>
              )}
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, lg: 4 }}>
          <Card sx={{ height: '100%' }}>
            <CardContent sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
              <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>Моё место</Typography>
              <Box sx={{ flexGrow: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Gauge
                  width={200}
                  height={200}
                  value={gaugeValue}
                  startAngle={-110}
                  endAngle={110}
                  sx={{
                    [`& .${gaugeClasses.valueText}`]: { fontSize: 40, fontWeight: 700 },
                    [`& .${gaugeClasses.valueArc}`]: { fill: '#8b5cf6' },
                  }}
                  text={() => `${gaugeValue.toFixed(0)}%`}
                />
              </Box>
              <Stack direction="row" sx={{ justifyContent: 'space-between', mt: 2, px: 2 }}>
                <Box><Typography variant="body2" color="text.secondary">Использовано</Typography><Typography variant="h6" sx={{ fontWeight: 600 }}>{usedGB.toFixed(1)} ГБ</Typography></Box>
                <Box sx={{ textAlign: 'right' }}><Typography variant="body2" color="text.secondary">Тариф</Typography><Typography variant="h6" sx={{ fontWeight: 600 }}>{TARIFF.dbStorageGB} ГБ</Typography></Box>
              </Stack>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      <Card>
        <CardContent>
          <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>Статистика контейнера</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Живая нагрузка вашего контейнера — выберите из списка</Typography>
          {containers.length === 0 ? (
            <Typography color="text.secondary">Нет контейнеров — создайте первое приложение.</Typography>
          ) : (
            <Stack spacing={2}>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems={{ sm: 'center' }}>
                <FormControl sx={{ minWidth: 240, flex: 1 }}>
                  <InputLabel>Контейнер</InputLabel>
                  <Select value={statsContainerId} label="Контейнер" onChange={(e) => handleSelectStatsContainer(e.target.value)}>
                    {containers.map((c) => (
                      <MenuItem key={c.id} value={c.id}>{c.app_name} · {c.status} · {c.node_id}</MenuItem>
                    ))}
                  </Select>
                </FormControl>
                <Button variant="outlined" startIcon={<RefreshCw size={16} />} disabled={!statsContainerId || statsLoading} onClick={() => loadStats(statsContainerId)}>
                  Обновить
                </Button>
              </Stack>
              {!statsContainerId && (
                <Typography variant="body2" color="text.secondary">Выберите контейнер, чтобы увидеть CPU и RAM в реальном времени.</Typography>
              )}
              {statsLoading && (
                <Stack direction="row" spacing={1.5} alignItems="center"><CircularProgress size={18} /><Typography variant="body2" color="text.secondary">Опрос воркера…</Typography></Stack>
              )}
              {statsError && <Alert severity="warning">{statsError}</Alert>}
              {stats && statsContainer && !statsLoading && (
                <Stack direction={{ xs: 'column', sm: 'row' }} spacing={3}>
                  <Box>
                    <Typography variant="body2" color="text.secondary">CPU сейчас</Typography>
                    <Typography variant="h5" sx={{ fontWeight: 700 }}>{stats.cpu_percent.toFixed(1)}%</Typography>
                    <Typography variant="caption" color="text.secondary">Лимит: {statsContainer.cpu_cores ?? '—'} ядер</Typography>
                  </Box>
                  <Box>
                    <Typography variant="body2" color="text.secondary">RAM сейчас</Typography>
                    <Typography variant="h5" sx={{ fontWeight: 700 }}>{stats.mem_mb.toFixed(1)} МБ</Typography>
                    <Typography variant="caption" color="text.secondary">Лимит: {statsContainer.memory_gb ? `${(statsContainer.memory_gb * 1024).toFixed(0)} МБ` : '—'} · Узел: {statsContainer.node_id}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="body2" color="text.secondary">Статус</Typography>
                    <Chip size="small" label={statusLabel(statsContainer.status)} color={statusColor(statsContainer.status)} sx={{ mt: 0.5 }} />
                  </Box>
                </Stack>
              )}
            </Stack>
          )}
        </CardContent>
      </Card>

      <Grid container spacing={3} alignItems="stretch">
        <Grid size={{ xs: 12, lg: 8 }}>
          <Card sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
            <CardContent sx={{ flex: 1, display: 'flex', flexDirection: 'column', p: 2.5, '&:last-child': { pb: 2.5 } }}>
              <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600, flexShrink: 0 }}>Расположение серверов</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Общая карта кластера — нагрузка на ней тоже общая</Typography>
              {nodesLoading && <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>Загрузка серверов...</Typography>}
              {nodesError && <Typography variant="body2" color="error" sx={{ mb: 1 }}>{nodesError}</Typography>}
              <Box sx={{ flex: 1, minHeight: 320, display: 'flex', overflow: 'hidden', borderRadius: 2 }}>
                <div className={styles.map}><ServerMap nodes={nodes} /></div>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, lg: 4 }}>
          <Stack spacing={3} sx={{ height: '100%' }}>
            <Card sx={{ height: '100%' }}>
              <CardContent>
                <Typography variant="h6" sx={{ mb: 3, fontWeight: 600 }}>Лимиты профиля</Typography>
                <Box sx={{ py: 1 }}><UserQuotas /></Box>
              </CardContent>
            </Card>
            <PortQuota sx={{ flexGrow: 1 }} />
          </Stack>
        </Grid>
      </Grid>

      <Card>
        <CardContent>
          <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>Последняя активность</Typography>
          {activity.length === 0 ? (
            <Typography color="text.secondary">Пока пусто — активность появится после первых деплоев.</Typography>
          ) : (
            <Stack spacing={1.5}>
              {activity.map((container) => (
                <Box key={container.id} className={styles.event}>
                  <Typography variant="caption" className={styles.event__time}>{timeAgo(container.updated_at)}</Typography>
                  <Box>
                    <Typography variant="body2" sx={{ fontWeight: 700 }}>{container.app_name}</Typography>
                    <Typography variant="body2" color="text.secondary">{statusLabel(container.status)} · {container.node_id}</Typography>
                  </Box>
                  <Chip size="small" label={statusLabel(container.status)} color={statusColor(container.status)} sx={{ ml: 'auto', alignSelf: 'center' }} />
                </Box>
              ))}
            </Stack>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
