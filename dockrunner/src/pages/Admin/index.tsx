import { useEffect, useState, useMemo } from 'react'
import { Box, Button, Card, CardContent, Stack, Typography, Chip, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Dialog, DialogTitle, DialogContent, DialogActions, TextField, FormControl, InputLabel, Select, MenuItem, Tabs, Tab, Divider, IconButton, Alert, Paper } from '@mui/material'
import { Plus, Ban, CheckCircle2, Crown, Square, RefreshCw, RotateCcw, Trash2, DollarSign, CreditCard, Receipt, Settings, Check, X, Eye, KeyRound } from 'lucide-react'
import { fetchAdminUsers, createAdminUser, setUserBlocked, setUserPassword, setUserPlan, fetchTariffPlans, fetchAdminContainers, adminContainerAction, rebootServer, fetchPendingRequests, fetchAllRequests, approveRequest, rejectRequest, setRequestPrice, fetchPricing, setPricing, fetchUserBalance, addUserBalance, setUserBalance, fetchAllTransactions } from '../../api/admin'
import type { AdminUser, AdminContainer, TariffPlan, DeploymentRequest, PricingRate, BalanceTransaction } from '../../api/admin'
import { useAuth } from '../../auth/AuthContext'
import { useAgents } from '../../hooks/useAgents'
import { timeAgo } from '../../utils/time'
import { PingText } from '../../components/Ping'
import styles from './Admin.module.scss'

function formatExpiry(iso: string | null) {
  if (!iso) return 'Бессрочно'
  return new Date(iso).toLocaleDateString('ru-RU')
}

function planName(plans: TariffPlan[], planId: string) {
  const foundPlan = plans.find((plan) => plan.id === planId)
  if (foundPlan) {
    return foundPlan.name
  }
  return planId
}

function todayPlusYear() {
  const d = new Date()
  d.setFullYear(d.getFullYear() + 1)
  return d.toISOString().slice(0, 10)
}

type AdminData = [AdminUser[], { data: TariffPlan[] }, AdminContainer[], DeploymentRequest[], DeploymentRequest[], PricingRate[], { data: BalanceTransaction[]; total: number }]

export default function Admin() {
  const [tab, setTab] = useState(0)
  const [users, setUsers] = useState<AdminUser[]>([])
  const [plans, setPlans] = useState<TariffPlan[]>([])
  const [containers, setContainers] = useState<AdminContainer[]>([])
  const [pendingRequests, setPendingRequests] = useState<DeploymentRequest[]>([])
  const [allRequests, setAllRequests] = useState<DeploymentRequest[]>([])
  const [pricing, setPricing] = useState<PricingRate[]>([])
  const [transactions, setTransactions] = useState<BalanceTransaction[]>([])
  const [transactionsTotal, setTransactionsTotal] = useState(0)
  const [transactionsPage, setTransactionsPage] = useState(1)
  const [error, setError] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [planOpen, setPlanOpen] = useState(false)
  const [pricingOpen, setPricingOpen] = useState(false)
  const [balanceOpen, setBalanceOpen] = useState(false)
  const [approveOpen, setApproveOpen] = useState(false)
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('user')
  const [planId, setPlanId] = useState('student')
  const [planUserId, setPlanUserId] = useState('')
  const [planExpires, setPlanExpires] = useState(todayPlusYear())
  const [busy, setBusy] = useState(false)
  const [rebooting, setRebooting] = useState('')
  const [actingId, setActingId] = useState('')
  const [approveReqId, setApproveReqId] = useState('')
  const [detailsReq, setDetailsReq] = useState<DeploymentRequest | null>(null)
  const [rejectReqId, setRejectReqId] = useState('')
  const [rejectComment, setRejectComment] = useState('')
  const [priceReqId, setPriceReqId] = useState('')
  const [priceValue, setPriceValue] = useState(0)
  const [pricingAgentId, setPricingAgentId] = useState('')
  const [pricingCpu, setPricingCpu] = useState(0)
  const [pricingMem, setPricingMem] = useState(0)
  const [pricingDisk, setPricingDisk] = useState(0)
  const [balanceLogin, setBalanceLogin] = useState('')
  const [balanceAmount, setBalanceAmount] = useState(0)
  const [balanceReason, setBalanceReason] = useState('')
  const [balanceSetValue, setBalanceSetValue] = useState(0)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordUserId, setPasswordUserId] = useState('')
  const [passwordUserLogin, setPasswordUserLogin] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const { user: currentUser } = useAuth()
  const isSuperuser = !!currentUser?.isSuperuser
  const { nodes, loading: nodesLoading } = useAgents()
  const [containerFilterUser, setContainerFilterUser] = useState('')
  const [containerFilterAgent, setContainerFilterAgent] = useState('')

  const filteredContainers = useMemo(() => {
    return containers.filter((c) => {
      const userMatch = !containerFilterUser || c.owner_login === containerFilterUser
      const agentMatch = !containerFilterAgent || c.node_id === containerFilterAgent
      return userMatch && agentMatch
    })
  }, [containers, containerFilterUser, containerFilterAgent])

  const loadData = () => {
    return Promise.all([
      fetchAdminUsers(), 
      fetchTariffPlans(), 
      fetchAdminContainers(),
      fetchPendingRequests(),
      fetchAllRequests(),
      fetchPricing(),
      fetchAllTransactions(1, 20)
    ])
  }

  const applyData = ([loadedUsers, plansResponse, loadedContainers, loadedPending, loadedAll, loadedPricing, loadedTransactions]: AdminData) => {
    setUsers(loadedUsers)
    setPlans(plansResponse.data)
    setContainers(loadedContainers)
    setPendingRequests(loadedPending)
    setAllRequests(loadedAll)
    setPricing(loadedPricing)
    setTransactions(loadedTransactions.data)
    setTransactionsTotal(loadedTransactions.total)
  }

  const failLoad = () => {
    setError('Не удалось загрузить данные. Проверьте соединение с сервером.')
  }

  const reloadAll = () => {
    setError('')
    loadData().then(applyData).catch(failLoad)
  }

  useEffect(() => {
    loadData().then(applyData).catch(failLoad)
  }, [])

  const handleCreate = () => {
    if (!login.trim() || !password) return
    setBusy(true)
    createAdminUser({ login: login.trim(), password, role, planId })
      .then(() => {
        setCreateOpen(false)
        setLogin('')
        setPassword('')
        reloadAll()
      })
      .catch(() => setError('Не удалось создать пользователя'))
      .finally(() => setBusy(false))
  }

  const handleBlock = (user: AdminUser) => {
    setUserBlocked(user.id, !user.blocked)
      .then(reloadAll)
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось изменить статус'))
  }

  const canManageUser = (user: AdminUser) => {
    if (user.isSuperuser) {
      return isSuperuser && user.login === currentUser?.login
    }
    if (user.role === 'admin') {
      return isSuperuser
    }
    return true
  }

  const openPasswordDialog = (user: AdminUser) => {
    setPasswordUserId(user.id)
    setPasswordUserLogin(user.login)
    setNewPassword('')
    setPasswordOpen(true)
  }

  const handleSavePassword = () => {
    if (newPassword.length < 6) {
      setError('Пароль должен быть минимум 6 символов')
      return
    }
    setUserPassword(passwordUserId, newPassword)
      .then(() => {
        setPasswordOpen(false)
        setNewPassword('')
        reloadAll()
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось сменить пароль'))
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
        reloadAll()
      })
      .catch(() => setError('Не удалось сменить тариф'))
  }

  const handleContainerAction = (id: string, action: 'stop' | 'restart' | 'remove') => {
    if (actingId) return
    setActingId(id)
    setError('')
    adminContainerAction(id, action)
      .then(reloadAll)
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось отправить задачу'))
      .finally(() => setActingId(''))
  }

  const handleReboot = (id: string) => {
    setRebooting(id)
    rebootServer(id)
      .catch(() => setError('Не удалось отправить задачу'))
      .finally(() => setRebooting(''))
  }

  // Deployment Requests handlers
  const openApproveDialog = (req: DeploymentRequest) => {
    setApproveReqId(req.id)
    setApproveOpen(true)
  }

  const handleApprove = () => {
    setError('')
    approveRequest(approveReqId)
      .then(() => {
        setApproveOpen(false)
        reloadAll()
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось одобрить заявку'))
  }

  const openRejectDialog = (req: DeploymentRequest) => {
    setRejectReqId(req.id)
    setRejectComment('')
    // We need a separate dialog or inline - using a prompt for now
    const comment = window.confirm('Отклонить заявку? Введите причину в следующем окне.') ? prompt('Причина отказа:') : null
    if (comment !== null) {
      rejectRequest(req.id, comment)
        .then(reloadAll)
        .catch(() => setError('Не удалось отклонить заявку'))
    }
  }

  const openSetPriceDialog = (req: DeploymentRequest) => {
    setPriceReqId(req.id)
    setPriceValue(req.price)
    // Using prompt for simplicity
    const price = prompt('Установить цену (руб):', req.price.toString())
    if (price !== null) {
      const priceNum = parseFloat(price)
      if (!isNaN(priceNum)) {
        setRequestPrice(req.id, priceNum)
          .then(reloadAll)
          .catch(() => setError('Не удалось установить цену'))
      }
    }
  }

  // Pricing handlers
  const openPricingDialog = (agent?: PricingRate) => {
    if (agent) {
      setPricingAgentId(agent.agentId)
      setPricingCpu(agent.cpuPerCoreHour)
      setPricingMem(agent.memPerGbHour)
      setPricingDisk(agent.diskPerGbHour)
    } else {
      setPricingAgentId('')
      setPricingCpu(0)
      setPricingMem(0)
      setPricingDisk(0)
    }
    setPricingOpen(true)
  }

  const handleSavePricing = () => {
    if (!pricingAgentId.trim()) return
    setPricing({ agentId: pricingAgentId, cpuPerCoreHour: pricingCpu, memPerGbHour: pricingMem, diskPerGbHour: pricingDisk })
      .then(() => {
        setPricingOpen(false)
        reloadAll()
      })
      .catch(() => setError('Не удалось сохранить тарифы'))
  }

  // Balance handlers
  const openBalanceDialog = (user: AdminUser) => {
    setBalanceLogin(user.login)
    setBalanceAmount(0)
    setBalanceReason('Пополнение баланса админом')
    setBalanceSetValue(0)
    setBalanceOpen(true)
  }

  const handleAddBalance = () => {
    addUserBalance(balanceLogin, balanceAmount, balanceReason)
      .then(() => {
        setBalanceOpen(false)
        reloadAll()
      })
      .catch(() => setError('Не удалось пополнить баланс'))
  }

  const handleSetBalance = () => {
    setUserBalance(balanceLogin, balanceSetValue)
      .then(() => {
        setBalanceOpen(false)
        reloadAll()
      })
      .catch(() => setError('Не удалось установить баланс'))
  }

  const loadMoreTransactions = (page: number) => {
    setTransactionsPage(page)
    fetchAllTransactions(page, 20)
      .then(({ data, total }) => {
        setTransactions(data)
        setTransactionsTotal(total)
      })
      .catch(() => setError('Не удалось загрузить транзакции'))
  }

  return (
    <Box className={styles.page}>
      <Box className={styles.header}>
        <Box>
          <Typography variant="h4" className={styles.header__title}>Администрирование</Typography>
          <Typography color="text.secondary">Пользователи, чужие контейнеры и серверы.</Typography>
        </Box>
        {tab === 0 && (
          <Button variant="contained" startIcon={<Plus size={18} />} onClick={() => setCreateOpen(true)}>Создать пользователя</Button>
        )}
      </Box>

      {error && <Alert severity="error">{error}</Alert>}

      <Card>
        <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ px: 2 }}>
          <Tab label={`Пользователи (${users.length})`} />
          <Tab label={`Контейнеры (${containers.length})`} />
          <Tab label="Серверы" />
          <Tab label={`Заявки (${pendingRequests.length})`} />
          <Tab label={<>Тарифы <DollarSign size={14} /></>} />
          <Tab label={<>Балансы <CreditCard size={14} /></>} />
        </Tabs>
        <Divider />
      </Card>

      {tab === 0 && (
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
      )}

      {tab === 1 && (
        <Card>
          <CardContent>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>Все контейнеры кластера</Typography>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mb: 2 }} alignItems={{ sm: 'center' }}>
              <FormControl size="small" sx={{ minWidth: 200 }}>
                <InputLabel>Фильтр по пользователю</InputLabel>
                <Select value={containerFilterUser} label="Фильтр по пользователю" onChange={(e) => setContainerFilterUser(e.target.value)}>
                  <MenuItem value=""><em>— Все пользователи —</em></MenuItem>
                  {users.map((u) => <MenuItem key={u.id} value={u.login}>{u.login}</MenuItem>)}
                </Select>
              </FormControl>
              <FormControl size="small" sx={{ minWidth: 200 }}>
                <InputLabel>Фильтр по агенту</InputLabel>
                <Select value={containerFilterAgent} label="Фильтр по агенту" onChange={(e) => setContainerFilterAgent(e.target.value)}>
                  <MenuItem value=""><em>— Все агенты —</em></MenuItem>
                  {nodes.map((n) => <MenuItem key={n.id} value={n.id}>{n.name} ({n.city})</MenuItem>)}
                </Select>
              </FormControl>
            </Stack>
            <TableContainer>
              <Table size="small">
                <TableHead><TableRow><TableCell>Контейнер</TableCell><TableCell>Владелец</TableCell><TableCell>Образ</TableCell><TableCell>Статус</TableCell><TableCell>Узел</TableCell><TableCell>Ресурсы</TableCell><TableCell>Порты</TableCell><TableCell>Действия</TableCell></TableRow></TableHead>
                <TableBody>
                  {filteredContainers.map((adminContainer) => (
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
                          <IconButton size="small" sx={{ color: 'text.secondary' }} disabled={actingId === adminContainer.id} onClick={() => handleContainerAction(adminContainer.id, 'restart')}><RefreshCw size={16} /></IconButton>
                          <IconButton size="small" sx={{ color: 'error.main' }} disabled={actingId === adminContainer.id} onClick={() => handleContainerAction(adminContainer.id, 'stop')}><Square size={16} /></IconButton>
                          <IconButton size="small" sx={{ color: 'error.main' }} disabled={actingId === adminContainer.id} onClick={() => { if (window.confirm(`Удалить контейнер ${adminContainer.app_name || adminContainer.id}?`)) handleContainerAction(adminContainer.id, 'remove') }}><Trash2 size={16} /></IconButton>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  ))}
                  {filteredContainers.length === 0 && <TableRow><TableCell colSpan={8} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет данных — контейнеры появятся после первых деплоев</TableCell></TableRow>}
                </TableBody>
              </Table>
            </TableContainer>
          </CardContent>
        </Card>
      )}

      {tab === 2 && (
        <Card>
          <CardContent>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>Серверы кластера</Typography>
            {nodesLoading && <Typography color="text.secondary">Загрузка серверов...</Typography>}
            {!nodesLoading && nodes.length === 0 && (
              <Typography color="text.secondary">Нет подключённых серверов — воркеры шлют heartbeat на централь.</Typography>
            )}
            <Stack spacing={1.5}>
              {nodes.map((node) => (
                <Box key={node.id} className={styles.serverRow}>
                  <Box>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>{node.name} <Typography component="span" variant="caption" color="text.secondary">· {node.city}</Typography></Typography>
                    {node.description && (
                      <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>{node.description}</Typography>
                    )}
                    <Typography variant="caption" color="text.secondary">
                      CPU: {node.cpu.total} ядер {node.cpu.mhz ? `· ${node.cpu.mhz} МГц` : ''} · RAM: {node.memory.total} ГБ {node.memory.type ? `· ${node.memory.type}` : ''} {node.memory.speedMhz ? `· ${node.memory.speedMhz} МГц` : ''} · Диск: {node.disk.total} ГБ {node.disk.type ? `· ${node.disk.type}` : ''}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Heartbeat: {timeAgo(node.lastSeen)} · Пинг: <PingText url={node.healthUrl} />
                    </Typography>
                  </Box>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Chip size="small" label={node.status === 'online' ? 'Онлайн' : 'Офлайн'} color={node.status === 'online' ? 'success' : 'default'} />
                    <Button size="small" variant="outlined" startIcon={<RotateCcw size={14} />} disabled={rebooting === node.id || node.status !== 'online'} onClick={() => handleReboot(node.id)}>
                      {rebooting === node.id ? 'Перезагрузка...' : 'Перезагрузить'}
                    </Button>
                  </Stack>
                </Box>
              ))}
            </Stack>
          </CardContent>
        </Card>
      )}

      {tab === 3 && (
        <Card>
          <CardContent>
            <Stack spacing={2} sx={{ mb: 2 }}>
              <Typography variant="h6" sx={{ fontWeight: 700 }}>Заявки на запуск</Typography>
              <Typography color="text.secondary">Ожидающие одобрения: {pendingRequests.length}</Typography>
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
                  {pendingRequests.map((req) => (
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
                              <Button size="small" startIcon={<Check size={14} />} variant="contained" color="success" onClick={() => openApproveDialog(req)}>Одобрить</Button>
                              <Button size="small" startIcon={<X size={14} />} color="error" onClick={() => openRejectDialog(req)}>Отклонить</Button>
                            </>
                          )}
                          <Button size="small" startIcon={<DollarSign size={14} />} onClick={() => openSetPriceDialog(req)}>Цена</Button>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  ))}
                  {pendingRequests.length === 0 && <TableRow><TableCell colSpan={7} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет ожидающих заявок</TableCell></TableRow>}
                </TableBody>
              </Table>
            </TableContainer>
          </CardContent>
        </Card>
      )}

      {tab === 4 && (
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
                  {pricing.map((p) => (
                    <TableRow key={p.agentId} hover>
                      <TableCell>{p.agentId}</TableCell>
                      <TableCell>{p.cpuPerCoreHour} руб</TableCell>
                      <TableCell>{p.memPerGbHour} руб</TableCell>
                      <TableCell>{p.diskPerGbHour} руб</TableCell>
                      <TableCell>{new Date(p.updatedAt).toLocaleString('ru-RU')}</TableCell>
                      <TableCell>
                        <Button size="small" startIcon={<Settings size={14} />} onClick={() => openPricingDialog(p)}>Изменить</Button>
                      </TableCell>
                    </TableRow>
                  ))}
                  {pricing.length === 0 && <TableRow><TableCell colSpan={6} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Нет тарифов — добавьте для каждого агента</TableCell></TableRow>}
                </TableBody>
              </Table>
            </TableContainer>
          </CardContent>
        </Card>
      )}

      {tab === 5 && (
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
            {transactionsTotal > 20 && (
              <Stack direction="row" spacing={1} justifyContent="center" sx={{ mt: 2 }}>
                <Button size="small" disabled={transactionsPage === 1} onClick={() => loadMoreTransactions(transactionsPage - 1)}>Назад</Button>
                <Typography variant="body2" sx={{ alignSelf: 'center' }}>Страница {transactionsPage} из {Math.ceil(transactionsTotal / 20)}</Typography>
                <Button size="small" disabled={transactionsPage >= Math.ceil(transactionsTotal / 20)} onClick={() => loadMoreTransactions(transactionsPage + 1)}>Вперёд</Button>
              </Stack>
            )}
          </CardContent>
        </Card>
      )}

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

      <Dialog open={approveOpen} onClose={() => setApproveOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>Одобрить заявку</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent} spacing={2}>
            {(() => {
              const req = allRequests.find((r) => r.id === approveReqId) || pendingRequests.find((r) => r.id === approveReqId)
              if (!req) {
                return <Typography color="text.secondary">Заявка не найдена</Typography>
              }
              const names = (req.services || []).length > 0 ? req.services.map((s) => `${req.appName}-${s}`).join(', ') : req.appName
              return (
                <>
                  <Typography><b>{req.appName}</b> · {req.ownerLogin} · узел {req.nodeId}</Typography>
                  <Typography variant="body2" color="text.secondary" sx={{ wordBreak: 'break-all' }}>{req.repoUrl || req.image}</Typography>
                  <Typography variant="body2">Контейнеров: {(req.services || []).length > 0 ? req.services.length : 1} ({names})</Typography>
                  <Typography variant="body2">Лимиты на контейнер: {req.cpuCores} ядер · {req.memoryGB} ГБ RAM · {req.diskGB} ГБ · {req.durationHours} ч</Typography>
                  <Alert severity="info">Запустится автоматически, с баланса спишется {req.price.toFixed(2)} руб</Alert>
                </>
              )
            })()}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setApproveOpen(false)}>Отмена</Button>
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
                <Typography variant="body2">
                  Узел: <b>{(() => { const n = nodes.find((x) => x.id === detailsReq.nodeId); return n ? `${n.name} (${n.city})` : detailsReq.nodeId })()}</b>
                </Typography>
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
                  <Typography variant="body2">Сервисы compose ({detailsReq.services.length}): {detailsReq.services.map((s) => `${detailsReq.appName}-${s}`).join(', ')}</Typography>
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
              <Button color="error" startIcon={<X size={14} />} onClick={() => { const r = detailsReq; setDetailsReq(null); if (r) openRejectDialog(r) }}>Отклонить</Button>
              <Button variant="contained" color="success" startIcon={<Check size={14} />} onClick={() => { const r = detailsReq; setDetailsReq(null); if (r) openApproveDialog(r) }}>Одобрить</Button>
            </>
          )}
          <Button onClick={() => setDetailsReq(null)}>Закрыть</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={pricingOpen} onClose={() => setPricingOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>{pricingAgentId ? 'Изменить тариф' : 'Новый тариф'}</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent} spacing={2}>
            <TextField label="ID агента" value={pricingAgentId} onChange={(e) => setPricingAgentId(e.target.value)} placeholder="worker-01" disabled={!!pricingAgentId} />
            <TextField type="number" label="CPU (руб/ядро/час)" value={pricingCpu} onChange={(e) => setPricingCpu(parseFloat(e.target.value) || 0)} placeholder="1" InputProps={{ step: 0.01 }} />
            <TextField type="number" label="RAM (руб/ГБ/час)" value={pricingMem} onChange={(e) => setPricingMem(parseFloat(e.target.value) || 0)} placeholder="0.5" InputProps={{ step: 0.01 }} />
            <TextField type="number" label="Диск (руб/ГБ/час)" value={pricingDisk} onChange={(e) => setPricingDisk(parseFloat(e.target.value) || 0)} placeholder="0.1" InputProps={{ step: 0.01 }} />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPricingOpen(false)}>Отмена</Button>
          <Button variant="contained" onClick={handleSavePricing}>Сохранить</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
