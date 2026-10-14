import { useState } from 'react'
import { Box, Button, Stack, Typography, Chip, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Dialog, DialogTitle, DialogContent, DialogActions, Card, CardContent, IconButton, Alert } from '@mui/material'
import { Check, X, DollarSign, Eye } from 'lucide-react'
import { approveRequest, rejectRequest, setRequestPrice } from '../../../api/admin'
import type { DeploymentRequest } from '../../../api/admin'
import type { WorkerNode } from '../../../types/workerNode'
import styles from '../Admin.module.scss'

export interface RequestsTabProps {
  pending: DeploymentRequest[]
  nodes: WorkerNode[]
  onChanged: () => void
  onError: (message: string) => void
}

function nodeLabel(nodes: WorkerNode[], nodeId: string) {
  const node = nodes.find((n) => n.id === nodeId)
  if (node) {
    return `${node.name} (${node.city})`
  }
  return nodeId
}

export default function RequestsTab({ pending, nodes, onChanged, onError }: RequestsTabProps) {
  const [approveReq, setApproveReq] = useState<DeploymentRequest | null>(null)
  const [detailsReq, setDetailsReq] = useState<DeploymentRequest | null>(null)

  const handleApprove = () => {
    if (!approveReq) return
    onError('')
    approveRequest(approveReq.id)
      .then(() => {
        setApproveReq(null)
        onChanged()
      })
      .catch((e) => onError(e instanceof Error ? e.message : 'Не удалось одобрить заявку'))
  }

  const handleReject = (req: DeploymentRequest) => {
    const comment = window.confirm('Отклонить заявку? Введите причину в следующем окне.') ? prompt('Причина отказа:') : null
    if (comment !== null) {
      rejectRequest(req.id, comment)
        .then(onChanged)
        .catch(() => onError('Не удалось отклонить заявку'))
    }
  }

  const handleSetPrice = (req: DeploymentRequest) => {
    const price = prompt('Установить цену (руб):', req.price.toString())
    if (price !== null) {
      const priceNum = parseFloat(price)
      if (!isNaN(priceNum)) {
        setRequestPrice(req.id, priceNum)
          .then(onChanged)
          .catch(() => onError('Не удалось установить цену'))
      }
    }
  }

  const containerNames = (req: DeploymentRequest) => {
    if ((req.services || []).length > 0) {
      return req.services.map((s) => `${req.appName}-${s}`).join(', ')
    }
    return req.appName
  }

  return (
    <>
      <Card>
        <CardContent>
          <Stack spacing={2} sx={{ mb: 2 }}>
            <Typography variant="h6" sx={{ fontWeight: 700 }}>Заявки на запуск</Typography>
            <Typography color="text.secondary">Ожидающие одобрения: {pending.length}</Typography>
          </Stack>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Заявка</TableCell>
                  <TableCell>Пользователь</TableCell>
                  <TableCell>Репозиторий</TableCell>
                  <TableCell>Ресурсы</TableCell>
                  <TableCell>Стоимость</TableCell>
                  <TableCell>Статус</TableCell>
                  <TableCell>Действия</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {pending.map((req) => (
                  <TableRow key={req.id} hover>
                    <TableCell>
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>{req.appName}</Typography>
                      <Typography variant="caption" color="text.secondary">{req.id.slice(0, 8)}</Typography>
                      {(req.services || []).length > 0 && (
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                          {req.services.length} конт.: {req.services.join(', ')}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>{req.ownerLogin}</TableCell>
                    <TableCell>
                      {req.repoUrl ? (
                        <a href={req.repoUrl} target="_blank" rel="noopener noreferrer">
                          <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12, color: 'primary.main', textDecoration: 'underline', cursor: 'pointer' }}>
                            {req.repoUrl}
                          </Typography>
                        </a>
                      ) : req.image ? (
                        <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12 }}>{req.image}</Typography>
                      ) : (
                        <Typography variant="body2" color="text.secondary">—</Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      {req.cpuCores} ядер · {req.memoryGB} ГБ RAM · {req.diskGB} ГБ диск · {req.durationHours} ч
                    </TableCell>
                    <TableCell><Typography variant="body2" sx={{ fontWeight: 600 }}>{req.price.toFixed(2)} руб</Typography></TableCell>
                    <TableCell>
                      <Chip size="small" label={req.status} color={req.status === 'pending' ? 'warning' : req.status === 'approved' ? 'success' : 'error'} variant="outlined" />
                    </TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={1}>
                        <IconButton size="small" sx={{ color: 'text.secondary' }} onClick={() => setDetailsReq(req)} title="Подробнее"><Eye size={16} /></IconButton>
                        {req.status === 'pending' && (
                          <>
                            <Button size="small" startIcon={<Check size={14} />} variant="contained" color="success" onClick={() => setApproveReq(req)}>Одобрить</Button>
                            <Button size="small" startIcon={<X size={14} />} color="error" onClick={() => handleReject(req)}>Отклонить</Button>
                          </>
                        )}
                        <Button size="small" startIcon={<DollarSign size={14} />} onClick={() => handleSetPrice(req)}>Цена</Button>
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))}
                {pending.length === 0 && <TableRow><TableCell colSpan={7} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет ожидающих заявок</TableCell></TableRow>}
              </TableBody>
            </Table>
          </TableContainer>
        </CardContent>
      </Card>

      <Dialog open={!!approveReq} onClose={() => setApproveReq(null)} fullWidth maxWidth="sm">
        <DialogTitle>Одобрить заявку</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent} spacing={2}>
            {approveReq && (
              <>
                <Typography><b>{approveReq.appName}</b> · {approveReq.ownerLogin} · узел {nodeLabel(nodes, approveReq.nodeId)}</Typography>
                <Typography variant="body2" color="text.secondary" sx={{ wordBreak: 'break-all' }}>{approveReq.repoUrl || approveReq.image}</Typography>
                <Typography variant="body2">Контейнеров: {(approveReq.services || []).length > 0 ? approveReq.services.length : 1} ({containerNames(approveReq)})</Typography>
                <Typography variant="body2">Лимиты на контейнер: {approveReq.cpuCores} ядер · {approveReq.memoryGB} ГБ RAM · {approveReq.diskGB} ГБ · {approveReq.durationHours} ч</Typography>
                <Alert severity="info">Запустится автоматически, с баланса спишется {approveReq.price.toFixed(2)} руб</Alert>
              </>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setApproveReq(null)}>Отмена</Button>
          <Button variant="contained" color="success" startIcon={<Check size={18} />} onClick={handleApprove}>Одобрить и запустить</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={!!detailsReq} onClose={() => setDetailsReq(null)} fullWidth maxWidth="md">
        <DialogTitle>Заявка: {detailsReq?.appName}</DialogTitle>
        <DialogContent>
          {detailsReq && (
            <Stack spacing={2} sx={{ mt: 1 }}>
              <Stack direction="row" spacing={1} alignItems="center">
                <Chip size="small" label={detailsReq.status} color={detailsReq.status === 'pending' ? 'warning' : detailsReq.status === 'approved' ? 'success' : 'error'} variant="outlined" />
                <Typography variant="body2" color="text.secondary">от {detailsReq.ownerLogin} · создана {new Date(detailsReq.createdAt).toLocaleString('ru-RU')}</Typography>
              </Stack>

              <Box>
                <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 0.5 }}>Где запустить</Typography>
                <Typography variant="body2">Узел: <b>{nodeLabel(nodes, detailsReq.nodeId)}</b></Typography>
                <Typography variant="body2" color="text.secondary">ID узла: {detailsReq.nodeId} · ID заявки: {detailsReq.id}</Typography>
              </Box>

              <Box>
                <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 0.5 }}>Источник</Typography>
                {detailsReq.repoUrl ? (
                  <a href={detailsReq.repoUrl} target="_blank" rel="noopener noreferrer">
                    <Typography variant="body2" sx={{ fontFamily: 'monospace', color: 'primary.main', textDecoration: 'underline', wordBreak: 'break-all' }}>{detailsReq.repoUrl}</Typography>
                  </a>
                ) : (
                  <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>Образ: {detailsReq.image || '—'}</Typography>
                )}
                <Typography variant="body2" color="text.secondary">Имя контейнера: {detailsReq.containerName || detailsReq.appName}</Typography>
                {(detailsReq.services || []).length > 0 && (
                  <Typography variant="body2">Сервисы compose ({detailsReq.services.length}): {containerNames(detailsReq)}</Typography>
                )}
              </Box>

              <Box>
                <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 0.5 }}>Ресурсы (на каждый контейнер)</Typography>
                <Typography variant="body2">{detailsReq.cpuCores} ядер · {detailsReq.memoryGB} ГБ RAM · {detailsReq.diskGB} ГБ диск · {detailsReq.durationHours} ч</Typography>
                <Typography variant="body2" sx={{ fontWeight: 600 }}>Итого: {detailsReq.price.toFixed(2)} руб</Typography>
              </Box>

              <Box>
                <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 0.5 }}>Порты ({(detailsReq.ports || []).length})</Typography>
                {(detailsReq.ports || []).length === 0 ? (
                  <Typography variant="body2" color="text.secondary">—</Typography>
                ) : (
                  <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                    {detailsReq.ports.map((p) => <Chip key={p.host_port} size="small" label={`${p.host_port}→${p.container_port}`} />)}
                  </Stack>
                )}
              </Box>

              <Box>
                <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 0.5 }}>Тома ({(detailsReq.volumes || []).length})</Typography>
                {(detailsReq.volumes || []).length === 0 ? (
                  <Typography variant="body2" color="text.secondary">—</Typography>
                ) : (
                  <Stack spacing={0.5}>
                    {detailsReq.volumes.map((v, i) => (
                      <Typography key={i} variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12 }}>
                        {v.name || v.host_path || '(анонимный)'} → {v.container_path}
                      </Typography>
                    ))}
                  </Stack>
                )}
              </Box>

              <Box>
                <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 0.5 }}>Переменные окружения ({Object.keys(detailsReq.env || {}).length})</Typography>
                {Object.keys(detailsReq.env || {}).length === 0 ? (
                  <Typography variant="body2" color="text.secondary">—</Typography>
                ) : (
                  <Stack spacing={0.5}>
                    {Object.entries(detailsReq.env || {}).map(([k, v]) => (
                      <Typography key={k} variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12 }}>{k}={v}</Typography>
                    ))}
                  </Stack>
                )}
              </Box>

              {(detailsReq.comment || detailsReq.decidedBy || detailsReq.deploymentId) && (
                <Box>
                  <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 0.5 }}>Решение</Typography>
                  {detailsReq.decidedBy && <Typography variant="body2">Решил: {detailsReq.decidedBy}{detailsReq.decidedAt ? ` · ${new Date(detailsReq.decidedAt).toLocaleString('ru-RU')}` : ''}</Typography>}
                  {detailsReq.comment && <Typography variant="body2">Комментарий: {detailsReq.comment}</Typography>}
                  {detailsReq.deploymentId && <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12 }}>Deployment: {detailsReq.deploymentId}</Typography>}
                </Box>
              )}
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          {detailsReq?.status === 'pending' && (
            <>
              <Button color="error" startIcon={<X size={14} />} onClick={() => { const r = detailsReq; setDetailsReq(null); if (r) handleReject(r) }}>Отклонить</Button>
              <Button variant="contained" color="success" startIcon={<Check size={14} />} onClick={() => { const r = detailsReq; setDetailsReq(null); if (r) setApproveReq(r) }}>Одобрить</Button>
            </>
          )}
          <Button onClick={() => setDetailsReq(null)}>Закрыть</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
