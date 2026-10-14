import { useState } from 'react'
import { Box, Button, Stack, Typography, Chip, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Dialog, DialogTitle, DialogContent, DialogActions, TextField, FormControl, InputLabel, Select, MenuItem, Card } from '@mui/material'
import { Plus, Ban, CheckCircle2, Crown, KeyRound } from 'lucide-react'
import { createAdminUser, setUserBlocked, setUserPassword, setUserPlan } from '../../../api/admin'
import type { AdminUser, TariffPlan } from '../../../api/admin'
import { useAuth } from '../../../auth/AuthContext'
import { formatExpiry, planName, todayPlusYear } from '../helpers'
import styles from '../Admin.module.scss'

interface UsersTabProps {
  users: AdminUser[]
  plans: TariffPlan[]
  onChanged: () => void
  onError: (message: string) => void
}

export default function UsersTab({ users, plans, onChanged, onError }: UsersTabProps) {
  const { user: currentUser } = useAuth()
  const isSuperuser = !!currentUser?.isSuperuser

  const [createOpen, setCreateOpen] = useState(false)
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('user')
  const [planId, setPlanId] = useState('student')
  const [busy, setBusy] = useState(false)

  const [planOpen, setPlanOpen] = useState(false)
  const [planUserId, setPlanUserId] = useState('')
  const [planExpires, setPlanExpires] = useState(todayPlusYear())

  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordUserId, setPasswordUserId] = useState('')
  const [passwordUserLogin, setPasswordUserLogin] = useState('')
  const [newPassword, setNewPassword] = useState('')

  const canManageUser = (user: AdminUser) => {
    if (user.isSuperuser) {
      return isSuperuser && user.login === currentUser?.login
    }
    if (user.role === 'admin') {
      return isSuperuser
    }
    return true
  }

  const handleCreate = () => {
    if (!login.trim() || !password) return
    setBusy(true)
    createAdminUser({ login: login.trim(), password, role, planId })
      .then(() => {
        setCreateOpen(false)
        setLogin('')
        setPassword('')
        onChanged()
      })
      .catch(() => onError('Не удалось создать пользователя'))
      .finally(() => setBusy(false))
  }

  const handleBlock = (user: AdminUser) => {
    setUserBlocked(user.id, !user.blocked)
      .then(onChanged)
      .catch((e) => onError(e instanceof Error ? e.message : 'Не удалось изменить статус'))
  }

  const openPlanDialog = (user: AdminUser) => {
    setPlanUserId(user.id)
    setPlanId(user.planId)
    setPlanExpires(user.planExpires ? user.planExpires.slice(0, 10) : todayPlusYear())
    setPlanOpen(true)
  }

  const handleSavePlan = () => {
    setUserPlan(planUserId, planId, planExpires || null)
      .then(() => {
        setPlanOpen(false)
        onChanged()
      })
      .catch(() => onError('Не удалось сменить тариф'))
  }

  const openPasswordDialog = (user: AdminUser) => {
    setPasswordUserId(user.id)
    setPasswordUserLogin(user.login)
    setNewPassword('')
    setPasswordOpen(true)
  }

  const handleSavePassword = () => {
    if (newPassword.length < 6) {
      onError('Пароль должен быть минимум 6 символов')
      return
    }
    setUserPassword(passwordUserId, newPassword)
      .then(() => {
        setPasswordOpen(false)
        setNewPassword('')
        onChanged()
      })
      .catch((e) => onError(e instanceof Error ? e.message : 'Не удалось сменить пароль'))
  }

  return (
    <>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 2 }}>
        <Button variant="contained" startIcon={<Plus size={18} />} onClick={() => setCreateOpen(true)}>Создать пользователя</Button>
      </Box>
      <Card>
        <TableContainer>
          <Table size="small">
            <TableHead><TableRow><TableCell>Пользователь</TableCell><TableCell>Роль</TableCell><TableCell>Тариф</TableCell><TableCell>Действует до</TableCell><TableCell>Статус</TableCell><TableCell>Действия</TableCell></TableRow></TableHead>
            <TableBody>
              {users.map((adminUser) => {
                const manageable = canManageUser(adminUser)
                const blockDisabled = !manageable || adminUser.isSuperuser
                const blockHint = adminUser.isSuperuser
                  ? 'Главного администратора заблокировать нельзя'
                  : !manageable
                    ? 'Администраторов блокирует только главный'
                    : undefined
                return (
                  <TableRow key={adminUser.id} hover>
                    <TableCell><Typography variant="body2" sx={{ fontWeight: 600 }}>{adminUser.login}</Typography><Typography variant="caption" color="text.secondary">{adminUser.id.slice(0, 8)}</Typography></TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.5}>
                        <Chip size="small" label={adminUser.role} variant="outlined" />
                        {adminUser.isSuperuser && <Chip size="small" label="Главный" color="warning" />}
                      </Stack>
                    </TableCell>
                    <TableCell><Chip size="small" icon={<Crown size={12} />} label={planName(plans, adminUser.planId)} color="primary" variant="outlined" /></TableCell>
                    <TableCell>{formatExpiry(adminUser.planExpires)}</TableCell>
                    <TableCell>{adminUser.blocked ? <Chip size="small" label="Заблокирован" color="error" /> : <Chip size="small" label="Активен" color="success" />}</TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={1}>
                        <Button size="small" onClick={() => openPlanDialog(adminUser)}>Тариф</Button>
                        <Button size="small" startIcon={<KeyRound size={14} />} disabled={!manageable} title={manageable ? 'Сменить пароль' : 'Пароли администраторов меняет только главный'} onClick={() => openPasswordDialog(adminUser)}>Пароль</Button>
                        <Button size="small" color={adminUser.blocked ? 'success' : 'error'} startIcon={adminUser.blocked ? <CheckCircle2 size={14} /> : <Ban size={14} />} disabled={blockDisabled} title={blockHint} onClick={() => handleBlock(adminUser)}>
                          {adminUser.blocked ? 'Разблокировать' : 'Заблокировать'}
                        </Button>
                      </Stack>
                    </TableCell>
                  </TableRow>
                )
              })}
              {users.length === 0 && <TableRow><TableCell colSpan={6} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет данных</TableCell></TableRow>}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>

      <Dialog open={createOpen} onClose={() => setCreateOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>Новый пользователь</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent}>
            <TextField label="Логин" value={login} onChange={(e) => setLogin(e.target.value)} placeholder="ivan" autoFocus />
            <TextField label="Пароль" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Минимум 6 символов" />
            <FormControl fullWidth>
              <InputLabel>Роль</InputLabel>
              <Select value={role} label="Роль" onChange={(e) => setRole(e.target.value)}>
                <MenuItem value="user">Пользователь</MenuItem>
                <MenuItem value="viewer">Наблюдатель</MenuItem>
                <MenuItem value="admin">Администратор</MenuItem>
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <InputLabel>Тариф</InputLabel>
              <Select value={planId} label="Тариф" onChange={(e) => setPlanId(e.target.value)}>
                {plans.map((t) => <MenuItem key={t.id} value={t.id}>{t.name} — {t.price}</MenuItem>)}
              </Select>
            </FormControl>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateOpen(false)}>Отмена</Button>
          <Button variant="contained" disabled={!login.trim() || !password || busy} onClick={handleCreate}>Создать</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={planOpen} onClose={() => setPlanOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>Тарифный план</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent}>
            <FormControl fullWidth>
              <InputLabel>Тариф</InputLabel>
              <Select value={planId} label="Тариф" onChange={(e) => setPlanId(e.target.value)}>
                {plans.map((t) => <MenuItem key={t.id} value={t.id}>{t.name} — {t.price}</MenuItem>)}
              </Select>
            </FormControl>
            <TextField label="Действует до" type="date" value={planExpires} onChange={(e) => setPlanExpires(e.target.value)} InputLabelProps={{ shrink: true }} helperText="Пусто — бессрочно" />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPlanOpen(false)}>Отмена</Button>
          <Button variant="contained" onClick={handleSavePlan}>Сохранить</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={passwordOpen} onClose={() => setPasswordOpen(false)} fullWidth maxWidth="xs">
        <DialogTitle>Новый пароль: {passwordUserLogin}</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent}>
            <TextField label="Новый пароль" type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="Минимум 6 символов" autoFocus fullWidth sx={{ mt: 1 }} />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPasswordOpen(false)}>Отмена</Button>
          <Button variant="contained" disabled={newPassword.length < 6} onClick={handleSavePassword}>Сохранить</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
