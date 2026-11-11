import { useEffect, useState, useCallback, useRef } from 'react'
import { Box, Typography, Button, Card, CardContent, Chip, Stack, CircularProgress, Alert, Tabs, Tab, Snackbar } from '@mui/material'
import { DataGrid } from '@mui/x-data-grid'
import type { GridColDef } from '@mui/x-data-grid'
import { useNavigate } from 'react-router-dom'
import { Plus, RefreshCw, AlertCircle } from 'lucide-react'
import { fetchMine, fetchContainerSpec, refreshContainerByApp } from '../../api/containers'
import styles from './Applications.module.scss'
import { statusLabel, statusColor } from '../../utils/status'
import { useRealtimeStatus } from '../../hooks/useRealtime'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { containerStatusPatched, fetchMineContainers } from '../../store/slices/containersSlice'
import { fetchMineRequests } from '../../store/slices/deploymentRequestsSlice'

function formatDate(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ru-RU')
}

const dataGridSx = {
  border: 'none',
  '& .MuiDataGrid-cell': { borderColor: 'divider', display: 'flex', alignItems: 'center' },
  '& .MuiDataGrid-columnHeaders': { bgcolor: 'background.default', borderColor: 'divider' },
  '& .MuiDataGrid-columnHeader': { alignItems: 'center' },
  '& .MuiDataGrid-row': { cursor: 'pointer' },
}

export default function Applications() {
  const navigate = useNavigate()
  const dispatch = useAppDispatch()
  const [tab, setTab] = useState(0)
  const rows = useAppSelector((state) => state.containers.mine)
  const requests = useAppSelector((state) => state.deploymentRequests.mine)
  const loading = useAppSelector(
    (state) => state.containers.mineStatus === 'idle' || state.containers.mineStatus === 'loading',
  )
  const error = useAppSelector((state) => state.containers.mineError)
  const [syncing, setSyncing] = useState(false)
  const [syncError, setSyncError] = useState<string | null>(null)
  const [syncSuccess, setSyncSuccess] = useState<string | null>(null)
  const connected = useRealtimeStatus()

  useEffect(() => {
    void dispatch(fetchMineContainers())
    void dispatch(fetchMineRequests())
  }, [dispatch])

  // Verify actual container status for all containers
  const verifyContainerStatuses = useCallback(async () => {
    setSyncing(true)
    setSyncError(null)
    setSyncSuccess(null)

    try {
      // Fetch fresh container list from backend to avoid stale closure
      const mineRes = await fetchMine()
      const currentRows = mineRes.data

      if (currentRows.length === 0) {
        setSyncSuccess('Статусы актуальны, изменений нет')
        setSyncing(false)
        return
      }

      // Fetch all container specs in parallel
      const specResults = await Promise.allSettled(
        currentRows.map(container => fetchContainerSpec(container.id))
      )

      let correctedCount = 0
      let refreshedCount = 0

      for (let i = 0; i < currentRows.length; i++) {
        const container = currentRows[i]
        const specResult = specResults[i]

        if (specResult.status === 'fulfilled') {
          const actualStatus = specResult.value.data.status
          if (actualStatus !== container.status) {
            dispatch(containerStatusPatched({ id: container.id, status: actualStatus }))
            correctedCount++
          }
        } else {
          // Handle 404 - backend DB out of sync with Docker on agent
          // Try to trigger a refresh from the agent
          const err = specResult.reason
          if (err?.status === 404 && container.app_name) {
            try {
              await refreshContainerByApp(container.app_name)
              refreshedCount++
            } catch (refreshErr) {
              console.warn(`Failed to refresh container ${container.id}:`, refreshErr)
            }
          } else {
            console.warn(`Failed to verify status for container ${container.id}:`, err)
          }
        }
      }

      const messages: string[] = []
      if (correctedCount > 0) messages.push(`исправлено ${correctedCount}`)
      if (refreshedCount > 0) messages.push(`запрошено обновление: ${refreshedCount}`)
      if (messages.length > 0) {
        setSyncSuccess(`Синхронизация: ${messages.join(', ')}`)
      } else {
        setSyncSuccess('Статусы актуальны, изменений нет')
      }
    } catch {
      setSyncError('Не удалось синхронизировать статусы')
    } finally {
      setSyncing(false)
    }
  }, [dispatch])

  // Auto-verify status on initial load for all containers (only once)
  const verifiedRef = useRef(false)
  useEffect(() => {
    if (!verifiedRef.current && rows.length > 0) {
      verifiedRef.current = true
      void verifyContainerStatuses()
    }
  }, [rows.length, verifyContainerStatuses])

  // Fallback status sync: only while the WebSocket is down.
  useEffect(() => {
    if (connected) return
    const interval = setInterval(() => {
      if (rows.length > 0) {
        verifyContainerStatuses()
      }
    }, 30000)

    return () => clearInterval(interval)
  }, [rows, connected, verifyContainerStatuses])

  const columns: GridColDef[] = [
    { field: 'app_name', headerName: 'Название приложения', flex: 1, minWidth: 200, align: 'center', headerAlign: 'center' },
    { field: 'image', headerName: 'Образ', width: 220, align: 'center', headerAlign: 'center' },
    {
      field: 'status',
      headerName: 'Статус',
      width: 140,
      align: 'center',
      headerAlign: 'center',
      renderCell: (params) => (
        <Chip label={statusLabel(params.value)} size="small" color={statusColor(params.value)} variant="outlined" sx={{ fontWeight: 600 }} />
      ),
    },
    { field: 'node_id', headerName: 'Узел', width: 140, align: 'center', headerAlign: 'center' },
    {
      field: 'updated_at',
      headerName: 'Обновлён',
      width: 180,
      align: 'center',
      headerAlign: 'center',
      renderCell: (params) => <Typography variant="body2">{formatDate(params.value)}</Typography>,
    },
  ]

  const requestColumns: GridColDef[] = [
    {
      field: 'appName',
      headerName: 'Название',
      flex: 1,
      minWidth: 200,
      align: 'center',
      headerAlign: 'center',
      renderCell: (params) => {
        const services = (params.row.services || []) as string[]
        return (
          <Stack spacing={0} sx={{ alignItems: 'center' }}>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>{params.value}</Typography>
            {services.length > 0 && (
              <Typography variant="caption" color="text.secondary">{services.length} конт.: {services.join(', ')}</Typography>
            )}
          </Stack>
        )
      },
    },
    { field: 'repoUrl', headerName: 'Репозиторий', width: 300, align: 'center', headerAlign: 'center', renderCell: (params) => params.value ? (
        <a href={params.value} target="_blank" rel="noopener noreferrer">
          <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12, color: 'primary.main', textDecoration: 'underline', cursor: 'pointer' }}>
            {params.value}
          </Typography>
        </a>
      ) : <Typography variant="body2" color="text.secondary">—</Typography> },
    {
      field: 'status',
      headerName: 'Статус',
      width: 150,
      align: 'center',
      headerAlign: 'center',
      renderCell: (params) => {
        const label = params.value === 'pending' ? 'На рассмотрении' : params.value
        const color = params.value === 'pending' ? 'warning' : params.value === 'approved' ? 'success' : 'error'
        return <Chip label={label} size="small" color={color} variant="outlined" sx={{ fontWeight: 600 }} />
      },
    },
    { field: 'cpuCores', headerName: 'CPU', width: 90, align: 'center', headerAlign: 'center', renderCell: (params) => <Typography variant="body2">{params.value} ядер</Typography> },
    { field: 'memoryGB', headerName: 'RAM', width: 90, align: 'center', headerAlign: 'center', renderCell: (params) => <Typography variant="body2">{params.value} ГБ</Typography> },
    { field: 'durationHours', headerName: 'Период', width: 90, align: 'center', headerAlign: 'center', renderCell: (params) => <Typography variant="body2">{params.value} ч</Typography> },
    { field: 'price', headerName: 'Стоимость', width: 110, align: 'center', headerAlign: 'center', renderCell: (params) => <Typography variant="body2" sx={{ fontWeight: 600 }}>{params.value.toFixed(2)} руб</Typography> },
    { field: 'createdAt', headerName: 'Создана', width: 180, align: 'center', headerAlign: 'center', renderCell: (params) => <Typography variant="body2">{formatDate(params.value)}</Typography> },
  ]

  return (
    <Box className={styles.page}>
      <Box className={styles.header}>
        <Typography variant="h5" className={styles.header__title}>Приложения</Typography>
        <Stack direction="row" spacing={1}>
          <Button
            variant="outlined"
            startIcon={<RefreshCw size={18} />}
            onClick={verifyContainerStatuses}
            disabled={syncing || rows.length === 0}
          >
            {syncing ? 'Синхронизация...' : 'Синхронизировать статусы'}
          </Button>
          <Button variant="contained" color="primary" startIcon={<Plus size={18} />} onClick={() => navigate('/applications/new')}>
            Создать приложение
          </Button>
        </Stack>
      </Box>
      <Card>
        <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ px: 2 }}>
          <Tab label={`Мои приложения (${rows.length})`} />
          <Tab label={`Мои заявки (${requests.length})`} />
        </Tabs>
      </Card>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      <Snackbar open={!!syncError} autoHideDuration={6000} onClose={() => setSyncError(null)}>
        <Alert severity="error" onClose={() => setSyncError(null)} sx={{ flexGrow: 1 }}>
          <AlertCircle size={18} style={{ marginRight: 8 }} />
          {syncError}
        </Alert>
      </Snackbar>
      <Snackbar open={!!syncSuccess} autoHideDuration={6000} onClose={() => setSyncSuccess(null)}>
        <Alert severity="success" onClose={() => setSyncSuccess(null)} sx={{ flexGrow: 1 }}>
          {syncSuccess}
        </Alert>
      </Snackbar>
      <Card>
        <CardContent className={styles.cardContentNoPad}>
          <Box className={styles.gridWrap}>
            {loading ? (
              <Stack sx={{ alignItems: 'center', py: 6 }}><CircularProgress /></Stack>
            ) : tab === 0 ? (
              <DataGrid
                rows={rows}
                columns={columns}
                disableRowSelectionOnClick
                onRowClick={(p) => navigate(`/applications/${p.row.app_name}`)}
                className={styles.dataGrid}
                sx={dataGridSx}
                localeText={{ noRowsLabel: 'Нет приложений — создайте первое' }}
              />
            ) : (
              <DataGrid
                rows={requests}
                columns={requestColumns}
                disableRowSelectionOnClick
                className={styles.dataGrid}
                sx={dataGridSx}
                localeText={{ noRowsLabel: 'Нет заявок — создайте первую при создании приложения' }}
              />
            )}
          </Box>
        </CardContent>
      </Card>
    </Box>
  )
}
