import { useState } from 'react'
import { Button, Stack, Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Dialog, DialogTitle, DialogContent, DialogActions, TextField, FormControl, InputLabel, Select, MenuItem, Card, CardContent } from '@mui/material'
import { Settings } from 'lucide-react'
import { setPricing } from '../../../api/admin'
import type { PricingRate } from '../../../api/admin'
import type { WorkerNode } from '../../../types/workerNode'
import styles from '../Admin.module.scss'

interface PricingTabProps {
  rates: PricingRate[]
  nodes: WorkerNode[]
  onChanged: () => void
  onError: (message: string) => void
}

export default function PricingTab({ rates, nodes, onChanged, onError }: PricingTabProps) {
  const [pricingOpen, setPricingOpen] = useState(false)
  const [pricingIsNew, setPricingIsNew] = useState(false)
  const [pricingAgentId, setPricingAgentId] = useState('')
  const [pricingCpu, setPricingCpu] = useState(0)
  const [pricingMem, setPricingMem] = useState(0)
  const [pricingDisk, setPricingDisk] = useState(0)

  const openPricingDialog = (agent?: PricingRate) => {
    if (agent) {
      setPricingIsNew(false)
      setPricingAgentId(agent.agent_id)
      setPricingCpu(agent.cpu_per_core_hour)
      setPricingMem(agent.mem_per_gb_hour)
      setPricingDisk(agent.disk_per_gb_hour)
    } else {
      setPricingIsNew(true)
      const withoutPricing = nodes.find((n) => !rates.some((p) => p.agent_id === n.id))
      setPricingAgentId(withoutPricing?.id || nodes[0]?.id || '')
      setPricingCpu(0)
      setPricingMem(0)
      setPricingDisk(0)
    }
    setPricingOpen(true)
  }

  const handleSavePricing = () => {
    if (!pricingAgentId.trim()) return
    setPricing({ agent_id: pricingAgentId, cpu_per_core_hour: pricingCpu, mem_per_gb_hour: pricingMem, disk_per_gb_hour: pricingDisk })
      .then(() => {
        setPricingOpen(false)
        onChanged()
      })
      .catch(() => onError('Не удалось сохранить тарифы'))
  }

  return (
    <>
      <Card>
        <CardContent>
          <Stack spacing={2} sx={{ mb: 2 }}>
            <Typography variant="h6" sx={{ fontWeight: 700 }}>Тарифы почасово</Typography>
            <Typography color="text.secondary">Настройте стоимость ресурсов для каждого агента (руб/час)</Typography>
          </Stack>
          <Button variant="contained" startIcon={<Settings size={18} />} onClick={() => openPricingDialog()}>Добавить тариф</Button>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Агент</TableCell>
                  <TableCell>CPU (за ядро/час)</TableCell>
                  <TableCell>RAM (за ГБ/час)</TableCell>
                  <TableCell>Диск (за ГБ/час)</TableCell>
                  <TableCell>Обновлено</TableCell>
                  <TableCell>Действия</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {rates.map((p) => (
                  <TableRow key={p.agent_id} hover>
                    <TableCell>{p.agent_id}</TableCell>
                    <TableCell>{p.cpu_per_core_hour} руб</TableCell>
                    <TableCell>{p.mem_per_gb_hour} руб</TableCell>
                    <TableCell>{p.disk_per_gb_hour} руб</TableCell>
                    <TableCell>{new Date(p.updated_at).toLocaleString('ru-RU')}</TableCell>
                    <TableCell>
                      <Button size="small" startIcon={<Settings size={14} />} onClick={() => openPricingDialog(p)}>Изменить</Button>
                    </TableCell>
                  </TableRow>
                ))}
                {rates.length === 0 && <TableRow><TableCell colSpan={6} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет тарифов — добавьте для каждого агента</TableCell></TableRow>}
              </TableBody>
            </Table>
          </TableContainer>
        </CardContent>
      </Card>

      <Dialog open={pricingOpen} onClose={() => setPricingOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>{pricingIsNew ? 'Новый тариф' : 'Изменить тариф'}</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent} spacing={2}>
            {nodes.length > 0 ? (
              <FormControl fullWidth>
                <InputLabel>Сервер</InputLabel>
                <Select value={pricingAgentId} label="Сервер" onChange={(e) => setPricingAgentId(e.target.value)} disabled={!pricingIsNew}>
                  {nodes.map((n) => <MenuItem key={n.id} value={n.id}>{n.name} ({n.city}) — {n.id}</MenuItem>)}
                </Select>
              </FormControl>
            ) : (
              <TextField label="ID агента" value={pricingAgentId} onChange={(e) => setPricingAgentId(e.target.value)} placeholder="worker-01" helperText="Серверы пока не на связи — впишите ID вручную" />
            )}
            <TextField type="number" label="CPU (руб/ядро/час)" value={pricingCpu} onChange={(e) => setPricingCpu(parseFloat(e.target.value) || 0)} placeholder="1" InputProps={{ step: 0.01 }} />
            <TextField type="number" label="RAM (руб/ГБ/час)" value={pricingMem} onChange={(e) => setPricingMem(parseFloat(e.target.value) || 0)} placeholder="0.5" InputProps={{ step: 0.01 }} />
            <TextField type="number" label="Диск (руб/ГБ/час)" value={pricingDisk} onChange={(e) => setPricingDisk(parseFloat(e.target.value) || 0)} placeholder="0.1" InputProps={{ step: 0.01 }} />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPricingOpen(false)}>Отмена</Button>
          <Button variant="contained" disabled={!pricingAgentId.trim()} onClick={handleSavePricing}>Сохранить</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
