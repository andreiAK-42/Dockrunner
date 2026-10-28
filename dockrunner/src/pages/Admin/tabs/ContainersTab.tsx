import { useMemo, useState } from 'react'
import { Stack, Typography, Chip, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, FormControl, InputLabel, Select, MenuItem, Card, CardContent, IconButton, Dialog, DialogTitle, DialogContent, DialogActions, TextField, Button, Alert } from '@mui/material'
import Grid from '@mui/material/Grid'
import { RefreshCw, Square, Trash2, Edit, Save, X } from 'lucide-react'
import type { AdminUser, AdminContainer } from '../../../api/admin'
import type { WorkerNode } from '../../../types/workerNode'
import { adminUpdateContainerParams } from '../../../api/admin'

interface ContainersTabProps {
  containers: AdminContainer[]
  users: AdminUser[]
  nodes: WorkerNode[]
  actingId: string
  onAction: (id: string, action: 'stop' | 'restart' | 'remove') => void
}

export default function ContainersTab({ containers, users, nodes, actingId, onAction }: ContainersTabProps) {
  const [filterUser, setFilterUser] = useState('')
  const [filterAgent, setFilterAgent] = useState('')
  const [editingContainer, setEditingContainer] = useState<AdminContainer | null>(null)
  const [editForm, setEditForm] = useState<Record<string, any>>({})
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
    setEditForm({
      image: container.image,
      cpu_cores: container.cpu_cores ?? 0.25,
      memory_gb: container.memory_gb ?? 0.5,
      disk_gb: container.disk_gb ?? 1.0,
      ports: container.ports || [],
      env: container.env || {},
      volumes: container.volumes || [],
      restart_policy: container.restart_policy || 'unless-stopped',
    })
    setEditError(null)
  }

  const closeEditDialog = () => {
    setEditingContainer(null)
    setEditForm({})
    setEditError(null)
  }

  const handleSave = async () => {
    if (!editingContainer) return
    setSaving(true)
    setEditError(null)
    try {
      await adminUpdateContainerParams(editingContainer.id, editForm)
      closeEditDialog()
      // The parent component will reload data
      window.dispatchEvent(new CustomEvent('admin-data-reload'))
    } catch (e: any) {
      setEditError(e.message || 'Не удалось обновить параметры')
    } finally {
      setSaving(false)
    }
  }

  const addPort = () => {
    setEditForm(prev => ({
      ...prev,
      ports: [...(prev.ports || []), { host_port: '', container_port: '' }]
    }))
  }

  const removePort = (index: number) => {
    setEditForm(prev => ({
      ...prev,
      ports: prev.ports.filter((_: any, i: number) => i !== index)
    }))
  }

  const addEnv = () => {
    setEditForm(prev => ({
      ...prev,
      env: { ...prev.env, ['']: '' }
    }))
  }

  const removeEnv = (key: string) => {
    setEditForm(prev => {
      const newEnv = { ...prev.env }
      delete newEnv[key]
      return { ...prev, env: newEnv }
    })
  }

  const handlePortChange = (index: number, field: 'host_port' | 'container_port', value: string) => {
    setEditForm(prev => ({
      ...prev,
      ports: prev.ports.map((p: any, i: number) => i === index ? { ...p, [field]: value } : p)
    }))
  }

  const handleEnvChange = (key: string, value: string) => {
    setEditForm(prev => ({
      ...prev,
      env: { ...prev.env, [key]: value }
    }))
  }

  return (
    <Card>
      <CardContent>
        <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>Все контейнеры кластера</Typography>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mb: 2 }} alignItems={{ sm: 'center' }}>
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
                      <IconButton size="small" sx={{ color: 'text.secondary' }} disabled={actingId === adminContainer.id} onClick={() => onAction(adminContainer.id, 'restart')}><RefreshCw size={16} /></IconButton>
                      <IconButton size="small" sx={{ color: 'error.main' }} disabled={actingId === adminContainer.id} onClick={() => onAction(adminContainer.id, 'stop')}><Square size={16} /></IconButton>
                      <IconButton size="small" sx={{ color: 'primary.main' }} disabled={actingId === adminContainer.id} onClick={() => openEditDialog(adminContainer)}><Edit size={16} /></IconButton>
                      <IconButton size="small" sx={{ color: 'error.main' }} disabled={actingId === adminContainer.id} onClick={() => { if (window.confirm(`Удалить контейнер ${adminContainer.app_name || adminContainer.id}?`)) onAction(adminContainer.id, 'remove') }}><Trash2 size={16} /></IconButton>
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
                  label="Образ"
                  value={editForm.image || ''}
                  onChange={(e) => setEditForm(prev => ({ ...prev, image: e.target.value }))}
                  required
                />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField
                  fullWidth
                  type="number"
                  label="CPU Cores"
                  value={editForm.cpu_cores}
                  onChange={(e) => setEditForm(prev => ({ ...prev, cpu_cores: parseFloat(e.target.value) || 0 }))}
                  inputProps={{ step: 0.01, min: 0.01 }}
                />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField
                  fullWidth
                  type="number"
                  label="Memory (GB)"
                  value={editForm.memory_gb}
                  onChange={(e) => setEditForm(prev => ({ ...prev, memory_gb: parseFloat(e.target.value) || 0 }))}
                  inputProps={{ step: 0.01, min: 0.01 }}
                />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField
                  fullWidth
                  type="number"
                  label="Disk (GB)"
                  value={editForm.disk_gb}
                  onChange={(e) => setEditForm(prev => ({ ...prev, disk_gb: parseFloat(e.target.value) || 0 }))}
                  inputProps={{ step: 0.1, min: 0.1 }}
                />
              </Grid>
              <Grid size={{ xs: 12 }}>
                <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>Порты</Typography>
                {(editForm.ports || []).map((port: any, index: number) => (
                  <Grid key={index} container spacing={1} size={{ xs: 12 }}>
                    <Grid size={{ xs: 5 }}>
                      <TextField
                        fullWidth
                        size="small"
                        type="number"
                        label="Host Port"
                        value={port.host_port || ''}
                        onChange={(e) => handlePortChange(index, 'host_port', e.target.value)}
                        inputProps={{ min: 1, max: 65535 }}
                      />
                    </Grid>
                    <Grid size={{ xs: 5 }}>
                      <TextField
                        fullWidth
                        size="small"
                        type="number"
                        label="Container Port"
                        value={port.container_port || ''}
                        onChange={(e) => handlePortChange(index, 'container_port', e.target.value)}
                        inputProps={{ min: 1, max: 65535 }}
                      />
                    </Grid>
                    <Grid size={{ xs: 2 }}>
                      <Button size="small" variant="outlined" color="error" onClick={() => removePort(index)} sx={{ mt: '21px' }}><X size={16} /></Button>
                    </Grid>
                  </Grid>
                ))}
                <Button size="small" startIcon={<Save size={14} />} onClick={addPort} sx={{ mt: 1 }}>Добавить порт</Button>
              </Grid>
              <Grid size={{ xs: 12 }}>
                <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>Переменные окружения</Typography>
                {Object.entries(editForm.env || {}).map(([key, value], index) => (
                  <Grid key={index} container spacing={1} size={{ xs: 12 }}>
                    <Grid size={{ xs: 5 }}>
                      <TextField
                        fullWidth
                        size="small"
                        label="Ключ"
                        value={key}
                        onChange={(e) => {
                          const newKey = e.target.value
                          if (newKey !== key) {
                            setEditForm(prev => {
                              const newEnv = { ...prev.env }
                              delete newEnv[key]
                              newEnv[newKey] = prev.env[key]
                              return { ...prev, env: newEnv }
                            })
                          }
                        }}
                      />
                    </Grid>
                    <Grid size={{ xs: 5 }}>
                      <TextField
                        fullWidth
                        size="small"
                        label="Значение"
                        value={value as string}
                        onChange={(e) => handleEnvChange(key, e.target.value)}
                      />
                    </Grid>
                    <Grid size={{ xs: 2 }}>
                      <Button size="small" variant="outlined" color="error" onClick={() => removeEnv(key)} sx={{ mt: '21px' }}><X size={16} /></Button>
                    </Grid>
                  </Grid>
                ))}
                <Button size="small" startIcon={<Save size={14} />} onClick={addEnv} sx={{ mt: 1 }}>Добавить переменную</Button>
              </Grid>
              <Grid size={{ xs: 12 }}>
                <TextField
                  fullWidth
                  label="Restart Policy"
                  value={editForm.restart_policy || 'unless-stopped'}
                  onChange={(e) => setEditForm(prev => ({ ...prev, restart_policy: e.target.value }))}
                  select
                  SelectProps={{ native: true }}
                >
                  <option value="unless-stopped">unless-stopped</option>
                  <option value="always">always</option>
                  <option value="on-failure">on-failure</option>
                  <option value="no">no</option>
                </TextField>
              </Grid>
            </Grid>
          </DialogContent>
          <DialogActions>
            <Button onClick={closeEditDialog} disabled={saving}>Отмена</Button>
            <Button variant="contained" onClick={handleSave} disabled={saving}><Save size={16} /> {saving ? 'Сохранение…' : 'Сохранить'}</Button>
          </DialogActions>
        </Dialog>
      </CardContent>
    </Card>
  )
}
