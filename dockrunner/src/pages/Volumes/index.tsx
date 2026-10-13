import { useEffect, useState } from 'react'
import { Box, Button, Card, CardContent, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Stack, TextField, Typography, Alert, FormControl, InputLabel, Select, MenuItem, CircularProgress } from '@mui/material'
import { Database, Plus, Trash2 } from 'lucide-react'
import { fetchVolumes, createVolume, deleteVolume } from '../../api/volumes'
import type { ApiVolume } from '../../api/volumes'
import { useAgents } from '../../hooks/useAgents'
import styles from './Volumes.module.scss'

const DEFAULT_VOLUME_SIZE_GB = 20

export default function Volumes() {
  const [volumes, setVolumes] = useState<ApiVolume[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [size, setSize] = useState(String(DEFAULT_VOLUME_SIZE_GB))
  const [node, setNode] = useState('')
  const [busy, setBusy] = useState(false)
  const { nodes } = useAgents()
  const online = nodes.filter((worker) => worker.status === 'online')

  let activeNode = node
  if (!activeNode && online.length > 0) {
    activeNode = online[0].id
  }

  const reload = () => {
    fetchVolumes()
      .then((res) => setVolumes(res.data))
      .catch(() => setError('Не удалось загрузить тома'))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    reload()
  }, [])

  const allocated = volumes.reduce((totalSize, volume) => totalSize + volume.size_gb, 0)
  const usedNodeIds = new Set(volumes.map((volume) => volume.node_id))
  const usedNodesCount = usedNodeIds.size

  const handleCreate = () => {
    const trimmedName = name.trim()
    if (!trimmedName || !activeNode) {
      return
    }
    setBusy(true)
    const sizeGb = Number(size) || DEFAULT_VOLUME_SIZE_GB
    const targetNode = activeNode
    createVolume({ name: trimmedName, node_id: targetNode, size_gb: sizeGb, container_path: '' })
      .then(() => {
        setOpen(false)
        setName('')
        setSize(String(DEFAULT_VOLUME_SIZE_GB))
        reload()
      })
      .catch(() => setError('Не удалось создать том'))
      .finally(() => setBusy(false))
  }

  const handleDelete = (volumeNode: string, volumeName: string) => {
    deleteVolume(volumeNode, volumeName)
      .then(reload)
      .catch(() => setError('Не удалось удалить том'))
  }

  return (
    <div className={styles.page}>
      <Box className={styles.header}>
        <Box><Typography variant="h4" className={styles.header__title}>Тома</Typography><Typography color="text.secondary">Именованные тома на воркерах. При создании приложения указывайте их по имени.</Typography></Box>
        <Button variant="contained" startIcon={<Plus size={18} />} onClick={() => setOpen(true)}>Создать том</Button>
      </Box>
      {error && <Alert severity="error">{error}</Alert>}
      <Box className={styles.statsGrid}>
        <Card><CardContent><Typography color="text.secondary">Всего томов</Typography><Typography variant="h5" className={styles.header__title}>{volumes.length}</Typography></CardContent></Card>
        <Card><CardContent><Typography color="text.secondary">Выделено</Typography><Typography variant="h5" className={styles.header__title}>{allocated.toFixed(1)} ГБ</Typography></CardContent></Card>
        <Card><CardContent><Typography color="text.secondary">Узлов</Typography><Typography variant="h5" className={styles.header__title}>{usedNodesCount}</Typography></CardContent></Card>
      </Box>
      <Card><CardContent><Typography variant="h6" className={styles.sectionTitle}>Постоянные тома</Typography>
        {loading ? (
          <Stack alignItems="center" sx={{ py: 4 }}><CircularProgress /></Stack>
        ) : volumes.length === 0 ? (
          <Typography color="text.secondary" sx={{ textAlign: 'center', py: 3 }}>Нет томов — создайте первый</Typography>
        ) : (
          <Stack className={styles.volumeList}>{volumes.map((volume) => (
            <Box key={`${volume.node_id}/${volume.name}`} className={styles.volumeItem}>
              <Stack direction={{ xs: 'column', md: 'row' }} alignItems={{ md: 'center' }} gap={2}>
                <Database size={21} />
                <Box className={styles.volumeItem__main}>
                  <Typography className={styles.volumeItem__name}>{volume.name}</Typography>
                  <Typography variant="caption" color="text.secondary">{volume.node_id}{volume.container_path ? ` · ${volume.container_path}` : ''}</Typography>
                </Box>
                <Chip size="small" label={volume.status === 'attached' ? 'Подключён' : volume.status} color={volume.status === 'attached' ? 'success' : 'default'} />
                <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'nowrap' }}>{volume.size_gb} ГБ</Typography>
                <Button color="error" size="small" onClick={() => handleDelete(volume.node_id, volume.name)} startIcon={<Trash2 size={16} />}>Удалить</Button>
              </Stack>
            </Box>
          ))}</Stack>
        )}
      </CardContent></Card>
      <Dialog open={open} onClose={() => setOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>Создать постоянный том</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent}>
            <TextField label="Название тома" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
            <FormControl fullWidth>
              <InputLabel>Узел</InputLabel>
              <Select value={activeNode} label="Узел" onChange={(e) => setNode(e.target.value)}>
                {online.map((worker) => <MenuItem key={worker.id} value={worker.id}>{worker.name} ({worker.city})</MenuItem>)}
              </Select>
            </FormControl>
            <TextField label="Размер (ГБ)" type="number" value={size} onChange={(e) => setSize(e.target.value)} helperText="Квота для учёта в профиле." />
          </Stack>
        </DialogContent>
        <DialogActions><Button onClick={() => setOpen(false)}>Отмена</Button><Button variant="contained" onClick={handleCreate} disabled={!name.trim() || !activeNode || busy}>Создать</Button></DialogActions>
      </Dialog>
    </div>
  )
}
