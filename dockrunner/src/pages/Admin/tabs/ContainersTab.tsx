import { useMemo, useState } from 'react'
import { Stack, Typography, Chip, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, FormControl, InputLabel, Select, MenuItem, Card, CardContent, IconButton } from '@mui/material'
import { RefreshCw, Square, Trash2 } from 'lucide-react'
import type { AdminUser, AdminContainer } from '../../../api/admin'
import type { WorkerNode } from '../../../types/workerNode'

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

  const filtered = useMemo(() => {
    return containers.filter((c) => {
      const userMatch = !filterUser || c.owner_login === filterUser
      const agentMatch = !filterAgent || c.node_id === filterAgent
      return userMatch && agentMatch
    })
  }, [containers, filterUser, filterAgent])

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
                      <IconButton size="small" sx={{ color: 'error.main' }} disabled={actingId === adminContainer.id} onClick={() => { if (window.confirm(`Удалить контейнер ${adminContainer.app_name || adminContainer.id}?`)) onAction(adminContainer.id, 'remove') }}><Trash2 size={16} /></IconButton>
                    </Stack>
                  </TableCell>
                </TableRow>
              ))}
              {filtered.length === 0 && <TableRow><TableCell colSpan={8} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет данных — контейнеры появятся после первых деплоев</TableCell></TableRow>}
            </TableBody>
          </Table>
        </TableContainer>
      </CardContent>
    </Card>
  )
}
