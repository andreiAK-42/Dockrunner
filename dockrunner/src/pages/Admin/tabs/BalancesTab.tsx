import { useState } from 'react'
import { Button, Stack, Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, TextField, FormControl, InputLabel, Select, MenuItem, Card, CardContent, Paper } from '@mui/material'
import { Plus, Check } from 'lucide-react'
import { addUserBalance, setUserBalance } from '../../../api/admin'
import type { AdminUser, BalanceTransaction } from '../../../api/admin'

interface BalancesTabProps {
  users: AdminUser[]
  transactions: BalanceTransaction[]
  total: number
  page: number
  onPage: (page: number) => void
  onChanged: () => void
  onError: (message: string) => void
}

export default function BalancesTab({ users, transactions, total, page, onPage, onChanged, onError }: BalancesTabProps) {
  const [balanceLogin, setBalanceLogin] = useState('')
  const [balanceAmount, setBalanceAmount] = useState(0)
  const [balanceReason, setBalanceReason] = useState('')
  const [balanceSetValue, setBalanceSetValue] = useState(0)

  const handleAddBalance = () => {
    addUserBalance(balanceLogin, balanceAmount, balanceReason)
      .then(() => {
        onChanged()
      })
      .catch(() => onError('Не удалось пополнить баланс'))
  }

  const handleSetBalance = () => {
    setUserBalance(balanceLogin, balanceSetValue)
      .then(() => {
        onChanged()
      })
      .catch(() => onError('Не удалось установить баланс'))
  }

  return (
    <Card>
      <CardContent>
        <Stack spacing={2} sx={{ mb: 2 }}>
          <Typography variant="h6" sx={{ fontWeight: 700 }}>Управление балансами</Typography>
          <Typography color="text.secondary">Просмотр транзакций и пополнение балансов пользователей</Typography>
        </Stack>
        <Paper sx={{ p: 2, mb: 2 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>Пополнить баланс</Typography>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems={{ sm: 'center' }}>
            <FormControl sx={{ minWidth: 220 }}>
              <InputLabel>Пользователь</InputLabel>
              <Select value={balanceLogin} label="Пользователь" onChange={(e) => setBalanceLogin(e.target.value)}>
                {users.map((u) => (
                  <MenuItem key={u.id} value={u.login}>{u.login} ({u.role})</MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField type="number" label="Сумма" value={balanceAmount} onChange={(e) => setBalanceAmount(parseFloat(e.target.value) || 0)} placeholder="1000" sx={{ minWidth: 120 }} />
            <TextField label="Причина" value={balanceReason} onChange={(e) => setBalanceReason(e.target.value)} placeholder="Пополнение админом" sx={{ flex: 1 }} />
            <Button variant="contained" startIcon={<Plus size={18} />} disabled={!balanceLogin} onClick={handleAddBalance}>Пополнить</Button>
          </Stack>
        </Paper>
        <Paper sx={{ p: 2, mb: 2 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>Установить баланс</Typography>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems={{ sm: 'center' }}>
            <FormControl sx={{ minWidth: 220 }}>
              <InputLabel>Пользователь</InputLabel>
              <Select value={balanceLogin} label="Пользователь" onChange={(e) => setBalanceLogin(e.target.value)}>
                {users.map((u) => (
                  <MenuItem key={u.id} value={u.login}>{u.login} ({u.role})</MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField type="number" label="Новый баланс" value={balanceSetValue} onChange={(e) => setBalanceSetValue(parseFloat(e.target.value) || 0)} placeholder="5000" sx={{ minWidth: 120 }} />
            <Button variant="contained" startIcon={<Check size={14} />} disabled={!balanceLogin} onClick={handleSetBalance}>Установить</Button>
          </Stack>
        </Paper>
        <Typography variant="h6" sx={{ fontWeight: 700, mb: 1 }}>Все транзакции</Typography>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>ID</TableCell>
                <TableCell>Пользователь</TableCell>
                <TableCell>Сумма</TableCell>
                <TableCell>Баланс после</TableCell>
                <TableCell>Причина</TableCell>
                <TableCell>Дата</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {transactions.map((t) => (
                <TableRow key={t.id} hover>
                  <TableCell>{t.id.slice(0, 8)}</TableCell>
                  <TableCell>{t.ownerLogin}</TableCell>
                  <TableCell>
                    <Typography variant="body2" sx={{ fontWeight: 600, color: t.amount >= 0 ? 'success.main' : 'error.main' }}>
                      {t.amount >= 0 ? '+' : ''}{t.amount.toFixed(2)} руб
                    </Typography>
                  </TableCell>
                  <TableCell>{t.balanceAfter.toFixed(2)} руб</TableCell>
                  <TableCell>{t.reason}</TableCell>
                  <TableCell>{new Date(t.createdAt).toLocaleString('ru-RU')}</TableCell>
                </TableRow>
              ))}
              {transactions.length === 0 && <TableRow><TableCell colSpan={6} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет транзакций</TableCell></TableRow>}
            </TableBody>
          </Table>
        </TableContainer>
        {total > 20 && (
          <Stack direction="row" spacing={1} justifyContent="center" sx={{ mt: 2 }}>
            <Button size="small" disabled={page === 1} onClick={() => onPage(page - 1)}>Назад</Button>
            <Typography variant="body2" sx={{ alignSelf: 'center' }}>Страница {page} из {Math.ceil(total / 20)}</Typography>
            <Button size="small" disabled={page >= Math.ceil(total / 20)} onClick={() => onPage(page + 1)}>Вперёд</Button>
          </Stack>
        )}
      </CardContent>
    </Card>
  )
}
