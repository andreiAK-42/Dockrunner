import { useEffect, useState } from 'react'
import { Box, Typography, Button, Card, CardContent, Chip, Stack, CircularProgress, Alert, Tabs, Tab } from '@mui/material'
import { DataGrid } from '@mui/x-data-grid'
import type { GridColDef } from '@mui/x-data-grid'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { fetchMine } from '../../api/containers'
import type { MineContainer } from '../../api/containers'
import { fetchMyDeploymentRequests } from '../../api/deploymentRequests'
import type { DeploymentRequest } from '../../api/deploymentRequests'
import styles from './Applications.module.scss'
import { statusLabel, statusColor } from '../../utils/status'

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
  const [tab, setTab] = useState(0)
  const [rows, setRows] = useState<MineContainer[]>([])
  const [requests, setRequests] = useState<DeploymentRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([fetchMine(), fetchMyDeploymentRequests()])
      .then(([mineRes, reqRes]) => {
        setRows(mineRes.data)
        setRequests(reqRes)
      })
      .catch(() => setError('Не удалось загрузить данные'))
      .finally(() => setLoading(false))
  }, [])

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
          <Stack spacing={0} alignItems="center">
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
        <Button variant="contained" color="primary" startIcon={<Plus size={18} />} onClick={() => navigate('/applications/new')}>
          Создать приложение
        </Button>
      </Box>
      <Card>
        <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ px: 2 }}>
          <Tab label={`Мои приложения (${rows.length})`} />
          <Tab label={`Мои заявки (${requests.length})`} />
        </Tabs>
      </Card>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      <Card>
        <CardContent className={styles.cardContentNoPad}>
          <Box className={styles.gridWrap}>
            {loading ? (
              <Stack alignItems="center" sx={{ py: 6 }}><CircularProgress /></Stack>
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
