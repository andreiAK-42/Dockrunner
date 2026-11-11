import { useMemo, useState } from 'react'
import { Stack, Typography, Chip, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, FormControl, InputLabel, Select, MenuItem, Card, CardContent, IconButton, Dialog, DialogTitle, DialogContent, DialogActions, TextField, Button, Alert, Grid } from '@mui/material'
import { RefreshCw, Square, Trash2, Edit, Save } from 'lucide-react'
import type { AdminUser, AdminContainer } from '../../../api/admin'
import type { WorkerNode } from '../../../types/workerNode'
import { adminUpdateContainerParams } from '../../../api/admin'
import EditableList from '../../../components/forms/EditableList'
import PortRow from '../../../components/forms/PortRow'
import VolumeRow, { type VolumeRowData } from '../../../components/forms/VolumeRow'
import EnvRow, { type EnvRowData } from '../../../components/forms/EnvRow'

interface ContainersTabProps {
  containers: AdminContainer[]
  users: AdminUser[]
  nodes: WorkerNode[]
  actingId: string
  onAction: (id: string, action: 'stop' | 'restart' | 'remove') => void
}

interface EditFormState {
  image?: string
  cpu_cores?: number
  memory_gb?: number
  disk_gb?: number
  restart_policy?: string
}

// Type conversions for shared form components
function adminPortsToPortRows(ports: { host_port: number; container_port: number }[]): { host: string; container: string }[] {
  return ports.map(p => ({ host: String(p.host_port), container: String(p.container_port) }))
}

function portRowsToAdminPorts(rows: { host: string; container: string }[]): { host_port: number; container_port: number }[] {
  return rows
    .filter(r => r.host.trim() && r.container.trim())
    .map(r => ({
      host_port: parseInt(r.host, 10),
      container_port: parseInt(r.container, 10),
    }))
    .filter(p => !isNaN(p.host_port) && !isNaN(p.container_port))
}

function adminVolumesToVolumeRows(volumes: { name?: string; host_path?: string; container_path: string }[]): VolumeRowData[] {
  return volumes.map(v => ({
    hostPath: v.name || v.host_path || '',
    containerPath: v.container_path,
  }))
}

function volumeRowsToAdminVolumes(rows: VolumeRowData[]): { name?: string; host_path?: string; container_path: string }[] {
  return rows
    .filter(r => r.containerPath.trim())
    .map(r => {
      const hostPath = r.hostPath.trim()
      const isPath = hostPath.includes('/') || hostPath.includes('\\') || hostPath.startsWith('.')
      if (isPath) {
        return { host_path: hostPath, container_path: r.containerPath.trim() }
      } else {
        return { name: hostPath, container_path: r.containerPath.trim() }
      }
    })
}

function adminEnvToEnvRows(env: Record<string, string>): EnvRowData[] {
  return Object.entries(env).map(([key, value]) => ({ key, value }))
}

function envRowsToAdminEnv(rows: EnvRowData[]): Record<string, string> {
  const env: Record<string, string> = {}
  for (const row of rows) {
    const key = row.key.trim()
    if (key) {
      env[key] = row.value
    }
  }
  return env
}

export default function ContainersTab({ containers, users, nodes, actingId, onAction }: ContainersTabProps) {
  const [filterUser, setFilterUser] = useState('')
  const [filterAgent, setFilterAgent] = useState('')
  const [editingContainer, setEditingContainer] = useState<AdminContainer | null>(null)
  const [editPorts, setEditPorts] = useState<{ host: string; container: string }[]>([])
  const [editVolumes, setEditVolumes] = useState<VolumeRowData[]>([])
  const [editEnv, setEditEnv] = useState<EnvRowData[]>([])
  const [editForm, setEditForm] = useState<EditFormState>({})
  const [editError, setEditError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const filtered = useMemo(() => {
    return containers.filter((c) => {
      const userMatch = !filterUser || c.owner_login === filterUser
      const agentMatch = !filterAgent || c.node_id === filterAgent
      return userMatch && agentMatch
    })
  }, [containers, filterUser, filterAgent])

  const openEditDialog = (container: AdminContainer) => {
    setEditingContainer(container)
    setEditPorts(adminPortsToPortRows(container.ports || []))
    setEditVolumes(adminVolumesToVolumeRows(container.volumes || []))
    setEditEnv(adminEnvToEnvRows(container.env || {}))
    setEditForm({
      image: container.image,
      cpu_cores: container.cpu_cores ?? 0.25,
      memory_gb: container.memory_gb ?? 0.5,
      disk_gb: container.disk_gb ?? 1.0,
      restart_policy: container.restart_policy || 'unless-stopped',
    })
    setEditError(null)
  }

  const closeEditDialog = () => {
    setEditingContainer(null)
    setEditPorts([])
    setEditVolumes([])
    setEditEnv([])
    setEditForm({})
    setEditError(null)
  }

  const handleSave = async () => {
    if (!editingContainer) return
    setSaving(true)
    setEditError(null)
    try {
      await adminUpdateContainerParams(editingContainer.id, {
        image: editForm.image,
        cpu_cores: editForm.cpu_cores,
        memory_gb: editForm.memory_gb,
        disk_gb: editForm.disk_gb,
        ports: portRowsToAdminPorts(editPorts),
        env: envRowsToAdminEnv(editEnv),
        volumes: volumeRowsToAdminVolumes(editVolumes),
        restart_policy: editForm.restart_policy,
      })
      closeEditDialog()
      // The parent component will reload data
      window.dispatchEvent(new CustomEvent('admin-data-reload'))
    } catch (e: unknown) {
      setEditError(e instanceof Error ? e.message : 'Не удалось обновить параметры')
    } finally {
      setSaving(false)
    }
  }

  const handlePortChange = (index: number, patch: Partial<{ host: string; container: string }>) => {
    setEditPorts(prev => {
      const next = [...prev]
      if (index >= 0 && index < next.length) {
        next[index] = { ...next[index], ...patch }
      }
      return next
    })
  }

  const handleVolumeChange = (index: number, patch: Partial<VolumeRowData>) => {
    setEditVolumes(prev => {
      const next = [...prev]
      if (index >= 0 && index < next.length) {
        next[index] = { ...next[index], ...patch }
      }
      return next
    })
  }

  const handleEnvChange = (index: number, patch: Partial<EnvRowData>) => {
    setEditEnv(prev => {
      const next = [...prev]
      if (index >= 0 && index < next.length) {
        next[index] = { ...next[index], ...patch }
      }
      return next
    })
  }

  return (
    <Card>
      <CardContent>
        <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>Все контейнеры кластера</Typography>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mb: 2, alignItems: { sm: 'center' } }}>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <InputLabel>Фильтр по пользователю</InputLabel>
            <Select value={filterUser} label="Фильтр по пользователю" onChange={(e) => setFilterUser(e.target.value)}>
              <MenuItem value=""><em>— Все пользователи —</em></MenuItem>
              {users.map((u) => <MenuItem key={u.id} value={u.login}>{u.login}</MenuItem>)}
            </Select>
          </FormControl>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <InputLabel>Фильтр по агенту</InputLabel>
            <Select value={filterAgent} label="Фильтр по агенту" onChange={(e) => setFilterAgent(e.target.value)}>
              <MenuItem value=""><em>— Все агенты —</em></MenuItem>
              {nodes.map((n) => <MenuItem key={n.id} value={n.id}>{n.name} ({n.city})</MenuItem>)}
            </Select>
          </FormControl>
        </Stack>
        <TableContainer>
          <Table size="small">
            <TableHead><TableRow><TableCell>Контейнер</TableCell><TableCell>Владелец</TableCell><TableCell>Образ</TableCell><TableCell>Статус</TableCell><TableCell>Узел</TableCell><TableCell>Ресурсы</TableCell><TableCell>Порты</TableCell><TableCell>Действия</TableCell></TableRow></TableHead>
            <TableBody>
              {filtered.map((adminContainer) => (
                <TableRow key={adminContainer.id} hover>
                  <TableCell><Typography variant="body2" sx={{ fontWeight: 600 }}>{adminContainer.app_name || adminContainer.id}</Typography><Typography variant="caption" color="text.secondary">{adminContainer.id}</Typography></TableCell>
                  <TableCell>{adminContainer.owner_login || '—'}</TableCell>
                  <TableCell><Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12 }}>{adminContainer.image || '—'}</Typography></TableCell>
                  <TableCell><Chip size="small" label={adminContainer.status} color={adminContainer.status === 'running' ? 'success' : adminContainer.status === 'error' ? 'error' : 'default'} /></TableCell>
                  <TableCell>{adminContainer.node_id || '—'}</TableCell>
                  <TableCell>
                    {(adminContainer.cpu_cores ? `${adminContainer.cpu_cores} ядер` : '—')} · {(adminContainer.memory_gb ? `${adminContainer.memory_gb} ГБ` : '—')} · {(adminContainer.disk_gb ? `${adminContainer.disk_gb} ГБ` : '—')}
                  </TableCell>
                  <TableCell>{(adminContainer.ports || []).length === 0 ? '—' : (adminContainer.ports || []).map((port) => <Chip key={port.host_port} size="small" label={`${port.host_port}→${port.container_port}`} sx={{ mr: 0.5, fontSize: 10 }} />)}</TableCell>
                  <TableCell>
                    <Stack direction="row" spacing={0.5}>
                      <IconButton size="small" sx={{ color: 'text.secondary' }} disabled={actingId === adminContainer.id} onClick={() => onAction(adminContainer.id, 'restart')}>
                        <RefreshCw size={16} />
                      </IconButton>
                      <IconButton size="small" sx={{ color: 'error.main' }} disabled={actingId === adminContainer.id} onClick={() => onAction(adminContainer.id, 'stop')}>
                        <Square size={16} />
                      </IconButton>
                      <IconButton size="small" sx={{ color: 'primary.main' }} disabled={actingId === adminContainer.id} onClick={() => openEditDialog(adminContainer)}>
                        <Edit size={16} />
                      </IconButton>
                      <IconButton size="small" sx={{ color: 'error.main' }} disabled={actingId === adminContainer.id} onClick={() => { if (window.confirm(`Удалить контейнер ${adminContainer.app_name || adminContainer.id}?`)) onAction(adminContainer.id, 'remove') }}>
                        <Trash2 size={16} />
                      </IconButton>
                    </Stack>
                  </TableCell>
                </TableRow>
              ))}
              {filtered.length === 0 && <TableRow><TableCell colSpan={8} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет данных — контейнеры появятся после первых деплоев</TableCell></TableRow>}
            </TableBody>
          </Table>
        </TableContainer>

        {/* Edit Dialog */}
        <Dialog open={!!editingContainer} onClose={closeEditDialog} maxWidth="md" fullWidth>
          <DialogTitle>Редактировать параметры контейнера: {editingContainer?.app_name || editingContainer?.id}</DialogTitle>
          <DialogContent>
            {editError && <Alert severity="error" sx={{ mb: 2 }}>{editError}</Alert>}
            <Grid container spacing={2} sx={{ maxHeight: 600, overflow: 'auto' }}>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField
                  fullWidth
                  size="small"
                  label="Образ"
                  value={editForm.image || ''}
                  onChange={(e) => setEditForm(prev => ({ ...prev, image: e.target.value }))}
                  required
                  sx={{ '& .MuiInputBase-root': { minHeight: 36 }, mt: 1 }}
                />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField
                  fullWidth
                  size="small"
                  type="number"
                  label="CPU Cores"
                  value={editForm.cpu_cores}
                  onChange={(e) => setEditForm(prev => ({ ...prev, cpu_cores: parseFloat(e.target.value) || 0 }))}
                  slotProps={{ htmlInput: { step: 0.01, min: 0.01 } }}
                  sx={{ '& .MuiInputBase-root': { minHeight: 36 }, mt: 1 }}
                />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField
                  fullWidth
                  size="small"
                  type="number"
                  label="Memory (GB)"
                  value={editForm.memory_gb}
                  onChange={(e) => setEditForm(prev => ({ ...prev, memory_gb: parseFloat(e.target.value) || 0 }))}
                  slotProps={{ htmlInput: { step: 0.01, min: 0.01 } }}
                  sx={{ '& .MuiInputBase-root': { minHeight: 36 }, mt: 1 }}
                />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField
                  fullWidth
                  size="small"
                  type="number"
                  label="Disk (GB)"
                  value={editForm.disk_gb}
                  onChange={(e) => setEditForm(prev => ({ ...prev, disk_gb: parseFloat(e.target.value) || 0 }))}
                  slotProps={{ htmlInput: { step: 0.1, min: 0.1 } }}
                  sx={{ '& .MuiInputBase-root': { minHeight: 36 }, mt: 1 }}
                />
              </Grid>
              <Grid size={{ xs: 12 }}>
                <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>Порты</Typography>
                <EditableList
                  items={editPorts}
                  renderItem={(item, index, onUpdate, onRemove) => (
                    <PortRow
                      row={item}
                      index={index}
                      suggestions={[]}
                      busyPorts={[]}
                      portsRange={{ min: 1, max: 65535 }}
                      effectiveNodeId={editingContainer?.node_id || ''}
                      onUpdate={onUpdate}
                      onRemove={onRemove}
                      isLast={index === editPorts.length - 1}
                    />
                  )}
                  onAdd={() => setEditPorts([...editPorts, { host: '', container: '' }])}
                  onRemove={(index) => setEditPorts(editPorts.filter((_, i) => i !== index))}
                  onUpdate={handlePortChange}
                  addButtonLabel="Добавить порт"
                  emptyMessage="Нет портов"
                />
              </Grid>
              <Grid size={{ xs: 12 }}>
                <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>Тома</Typography>
                <EditableList
                  items={editVolumes}
                  renderItem={(item, index, onUpdate, onRemove) => (
                    <VolumeRow
                      row={item}
                      index={index}
                      onUpdate={onUpdate}
                      onRemove={onRemove}
                      isLast={index === editVolumes.length - 1}
                    />
                  )}
                  onAdd={() => setEditVolumes([...editVolumes, { hostPath: '', containerPath: '' }])}
                  onRemove={(index) => setEditVolumes(editVolumes.filter((_, i) => i !== index))}
                  onUpdate={handleVolumeChange}
                  addButtonLabel="Добавить том"
                  emptyMessage="Нет томов"
                />
              </Grid>
              <Grid size={{ xs: 12 }}>
                <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>Переменные окружения</Typography>
                <EditableList
                  items={editEnv}
                  renderItem={(item, index, onUpdate, onRemove) => (
                    <EnvRow
                      row={item}
                      index={index}
                      onUpdate={onUpdate}
                      onRemove={onRemove}
                      isLast={index === editEnv.length - 1}
                    />
                  )}
                  onAdd={() => setEditEnv([...editEnv, { key: '', value: '' }])}
                  onRemove={(index) => setEditEnv(editEnv.filter((_, i) => i !== index))}
                  onUpdate={handleEnvChange}
                  addButtonLabel="Добавить переменную"
                  emptyMessage="Нет переменных окружения"
                />
              </Grid>
              <Grid size={{ xs: 12 }}>
                <FormControl fullWidth size="small" sx={{ mt: 1 }}>
                  <InputLabel>Restart Policy</InputLabel>
                  <Select
                    value={editForm.restart_policy || 'unless-stopped'}
                    label="Restart Policy"
                    onChange={(e) => setEditForm(prev => ({ ...prev, restart_policy: e.target.value }))}
                  >
                    <MenuItem value="unless-stopped">unless-stopped</MenuItem>
                    <MenuItem value="always">always</MenuItem>
                    <MenuItem value="on-failure">on-failure</MenuItem>
                    <MenuItem value="no">no</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
            </Grid>
          </DialogContent>
          <DialogActions>
            <Button onClick={closeEditDialog} disabled={saving}>Отмена</Button>
            <Button variant="contained" onClick={handleSave} disabled={saving}>
              <Save size={16} /> {saving ? 'Сохранение…' : 'Сохранить'}
            </Button>
          </DialogActions>
        </Dialog>
      </CardContent>
    </Card>
  )
}