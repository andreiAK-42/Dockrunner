import { useEffect, useState } from 'react'
import { Box, Typography, Card, Tabs, Tab, Divider, Alert } from '@mui/material'
import { DollarSign, CreditCard } from 'lucide-react'
import { fetchAdminUsers, fetchTariffPlans, fetchAdminContainers, adminContainerAction, rebootServer, fetchPendingRequests, fetchPricing, fetchAllTransactions } from '../../api/admin'
import type { AdminUser, AdminContainer, TariffPlan, DeploymentRequest, PricingRate, BalanceTransaction } from '../../api/admin'
import { useAgents } from '../../hooks/useAgents'
import UsersTab from './tabs/UsersTab'
import ContainersTab from './tabs/ContainersTab'
import ServersTab from './tabs/ServersTab'
import RequestsTab from './tabs/RequestsTab'
import PricingTab from './tabs/PricingTab'
import BalancesTab from './tabs/BalancesTab'
import styles from './Admin.module.scss'

type AdminData = [AdminUser[], { data: TariffPlan[] }, AdminContainer[], DeploymentRequest[], PricingRate[], { data: BalanceTransaction[]; total: number }]

export default function Admin() {
  const [tab, setTab] = useState(0)
  const [users, setUsers] = useState<AdminUser[]>([])
  const [plans, setPlans] = useState<TariffPlan[]>([])
  const [containers, setContainers] = useState<AdminContainer[]>([])
  const [pendingRequests, setPendingRequests] = useState<DeploymentRequest[]>([])
  const [pricing, setPricing] = useState<PricingRate[]>([])
  const [transactions, setTransactions] = useState<BalanceTransaction[]>([])
  const [transactionsTotal, setTransactionsTotal] = useState(0)
  const [transactionsPage, setTransactionsPage] = useState(1)
  const [error, setError] = useState('')
  const [rebooting, setRebooting] = useState('')
  const [actingId, setActingId] = useState('')
  const { nodes, loading: nodesLoading } = useAgents()

  const loadData = () => {
    return Promise.all([
      fetchAdminUsers(),
      fetchTariffPlans(),
      fetchAdminContainers(),
      fetchPendingRequests(),
      fetchPricing(),
      fetchAllTransactions(1, 20),
    ])
  }

  const applyData = ([loadedUsers, plansResponse, loadedContainers, loadedPending, loadedPricing, loadedTransactions]: AdminData) => {
    setUsers(loadedUsers)
    setPlans(plansResponse.data)
    setContainers(loadedContainers)
    setPendingRequests(loadedPending)
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
    let active = true
    loadData().then((data) => { if (active) applyData(data) }).catch(failLoad)
    const timer = window.setInterval(() => {
      fetchAdminContainers().then((latest) => {
        if (active) setContainers(latest)
      }).catch(() => {
        // Не показываем ошибку при единичном сбое фонового обновления.
      })
    }, 5000)

    // Listen for custom reload event from container edit dialog
    const handleReload = () => {
      if (active) reloadAll()
    }
    window.addEventListener('admin-data-reload', handleReload)

    return () => {
      active = false
      window.clearInterval(timer)
      window.removeEventListener('admin-data-reload', handleReload)
    }
  }, [])

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
          <Typography color="text.secondary">Пользователи, контейнеры, серверы, заявки, тарифы и балансы.</Typography>
        </Box>
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
        <UsersTab users={users} plans={plans} onChanged={reloadAll} onError={setError} />
      )}

      {tab === 1 && (
        <ContainersTab containers={containers} users={users} nodes={nodes} actingId={actingId} onAction={handleContainerAction} />
      )}

      {tab === 2 && (
        <ServersTab nodes={nodes} loading={nodesLoading} rebooting={rebooting} onReboot={handleReboot} />
      )}

      {tab === 3 && (
        <RequestsTab pending={pendingRequests} nodes={nodes} onChanged={reloadAll} onError={setError} />
      )}

      {tab === 4 && (
        <PricingTab rates={pricing} nodes={nodes} onChanged={reloadAll} onError={setError} />
      )}

      {tab === 5 && (
        <BalancesTab
          users={users}
          transactions={transactions}
          total={transactionsTotal}
          page={transactionsPage}
          onPage={loadMoreTransactions}
          onChanged={reloadAll}
          onError={setError}
        />
      )}
    </Box>
  )
}
