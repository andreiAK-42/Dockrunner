import { useEffect, useState } from 'react'
import { Box, Button, Card, CardContent, Stack, Typography, CircularProgress, IconButton, LinearProgress, Chip, Alert, Divider } from '@mui/material'
import { Database, Copy, KeyRound, Trash2, Puzzle, RotateCcw } from 'lucide-react'
import {
  fetchIntegrations,
  fetchMyIntegrationAccounts,
  createIntegrationAccount,
  deleteIntegrationAccount,
  resetIntegrationPassword,
} from '../../api/integrations'
import type { IntegrationKind, IntegrationId, IntegrationAccount, IntegrationResult } from '../../api/integrations'
import { copyText } from '../../utils/clipboard'
import styles from './Integrations.module.scss'

function formatDate(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ru-RU')
}

function Credentials({ result }: { result: IntegrationResult }) {
  return (
    <Stack spacing={1.5} sx={{ mt: 2 }}>
      <Alert severity="success">Сохраните секрет — он показан один раз. Потом его можно только сбросить.</Alert>
      <Box className={styles.creds}>
        {result.rows.map((row) => (
          <Box key={row.label} className={styles.creds__row}>
            <Typography variant="body2" color="text.secondary">{row.label}</Typography>
            <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: 'monospace', wordBreak: 'break-all' }}>{row.value}</Typography>
          </Box>
        ))}
      </Box>
      {result.connection_string && (
        <Box className={styles.conn}>
          <Typography variant="caption" color="text.secondary">Строка подключения</Typography>
          <Box className={styles.conn__row}>
            <Typography variant="body2" sx={{ fontFamily: 'monospace', wordBreak: 'break-all' }}>{result.connection_string}</Typography>
            <IconButton size="small" onClick={() => copyText(result.connection_string || '')}><Copy size={14} /></IconButton>
          </Box>
        </Box>
      )}
    </Stack>
  )
}

export default function Integrations() {
  const [kinds, setKinds] = useState<IntegrationKind[]>([])
  const [users, setUsers] = useState<IntegrationAccount[]>([])
  const [usageGB, setUsageGB] = useState(0)
  const [limitGB, setLimitGB] = useState(0)
  const [loadingList, setLoadingList] = useState(true)
  const [fresh, setFresh] = useState<Partial<Record<IntegrationId, IntegrationResult>>>({})
  const [creating, setCreating] = useState<IntegrationId | null>(null)
  const [resetting, setResetting] = useState<string | null>(null)
  const [error, setError] = useState('')

  const reload = () => {
    fetchIntegrations().then(setKinds).catch(() => setError('Не удалось загрузить список интеграций'))
    fetchMyIntegrationAccounts()
      .then((res) => {
        setUsers(res.users)
        setUsageGB(res.usage_gb)
        setLimitGB(res.limit_gb)
      })
      .catch(() => setError('Не удалось загрузить ваши учётки'))
      .finally(() => setLoadingList(false))
  }

  useEffect(reload, [])

  const accountFor = (id: IntegrationId) => users.find((u) => u.kind === id)

  const handleCreate = (id: IntegrationId) => {
    setCreating(id)
    setError('')
    createIntegrationAccount(id)
      .then((result) => {
        setFresh((prev) => ({ ...prev, [id]: result }))
        return fetchMyIntegrationAccounts()
      })
      .then((res) => {
        setUsers(res.users)
        setUsageGB(res.usage_gb)
        setLimitGB(res.limit_gb)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось создать учётку'))
      .finally(() => setCreating(null))
  }

  const handleReset = (user: IntegrationAccount) => {
    if (!window.confirm(`Сбросить пароль для ${user.username}? Старый перестанет работать.`)) {
      return
    }
    setResetting(user.id)
    setError('')
    resetIntegrationPassword(user.id)
      .then((result) => {
        setFresh((prev) => ({ ...prev, [user.kind]: result }))
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось сбросить пароль'))
      .finally(() => setResetting(null))
  }

  const handleDelete = (user: IntegrationAccount) => {
    if (!window.confirm(`Удалить учётку ${user.username} (${user.kind})? Данные интеграции будут стёрты.`)) {
      return
    }
    setError('')
    deleteIntegrationAccount(user.id)
      .then(() => {
        setFresh((prev) => {
          const next = { ...prev }
          delete next[user.kind]
          return next
        })
        return fetchMyIntegrationAccounts()
      })
      .then((res) => {
        setUsers(res.users)
        setUsageGB(res.usage_gb)
        setLimitGB(res.limit_gb)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось удалить'))
  }

  const limitReached = limitGB > 0 && usageGB >= limitGB
  const usagePercent = limitGB > 0 ? Math.min(100, (usageGB / limitGB) * 100) : 0
  const databases = kinds.filter((k) => k.group === 'database')
  const others = kinds.filter((k) => k.group !== 'database')

  const renderCard = (item: IntegrationKind) => {
    const busy = creating === item.id
    const account = accountFor(item.id)
    const lastCreated = fresh[item.id]
    const quotaBlocks = item.hasQuota && limitReached && !account
    return (
      <Card key={item.id}>
        <CardContent>
          <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 1 }}>
            <Box className={styles.dbIcon}>{item.group === 'database' ? <Database size={20} /> : <Puzzle size={20} />}</Box>
            <Box>
              <Typography variant="h6" sx={{ fontWeight: 700 }}>{item.name}</Typography>
              <Typography variant="caption" color="text.secondary">Порт: {item.defaultPort}</Typography>
            </Box>
          </Stack>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>{item.description}</Typography>

          {!account ? (
            <Button
              variant="contained"
              startIcon={<KeyRound size={16} />}
              disabled={busy || quotaBlocks || !item.enabled}
              onClick={() => handleCreate(item.id)}
              fullWidth
            >
              {busy ? 'Создание...' : item.enabled ? 'Создать доступ' : 'Не настроено на сервере'}
            </Button>
          ) : (
            <Stack spacing={1.5}>
              <Box className={styles.creds}>
                <Box className={styles.creds__row}>
                  <Typography variant="body2" color="text.secondary">Адрес</Typography>
                  <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: 'monospace' }}>{account.host}</Typography>
                </Box>
                <Box className={styles.creds__row}>
                  <Typography variant="body2" color="text.secondary">Порт</Typography>
                  <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: 'monospace' }}>{account.port}</Typography>
                </Box>
                {item.group === 'database' && item.id !== 'redis' && (
                  <Box className={styles.creds__row}>
                    <Typography variant="body2" color="text.secondary">База</Typography>
                    <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: 'monospace' }}>{account.resource}</Typography>
                  </Box>
                )}
                <Box className={styles.creds__row}>
                  <Typography variant="body2" color="text.secondary">Логин</Typography>
                  <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: 'monospace' }}>{account.username}</Typography>
                </Box>
                {item.hasQuota && (
                  <Box className={styles.creds__row}>
                    <Typography variant="body2" color="text.secondary">Занято</Typography>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>{account.sizeGB.toFixed(1)} ГБ</Typography>
                  </Box>
                )}
              </Box>
              <Typography variant="caption" color="text.secondary">Создано {formatDate(account.createdAt)}. Пароль хранится только у вас — при утере нажмите «Новый пароль».</Typography>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
                <Button size="small" variant="outlined" startIcon={<RotateCcw size={14} />} disabled={resetting === account.id} onClick={() => handleReset(account)}>
                  {resetting === account.id ? 'Сброс...' : 'Новый пароль'}
                </Button>
                <Button size="small" color="error" startIcon={<Trash2 size={14} />} onClick={() => handleDelete(account)}>Удалить</Button>
              </Stack>
            </Stack>
          )}

          {lastCreated && <Credentials result={lastCreated} />}
        </CardContent>
      </Card>
    )
  }

  return (
    <Box className={styles.page}>
      <Box className={styles.header}>
        <Typography variant="h4" className={styles.header__title}>Интеграции</Typography>
        <Typography color="text.secondary">
          Доступ к общим сервисам платформы. Если учётки с вашим логином ещё нет — создайте её, адрес и пароль появятся на карточке.
        </Typography>
      </Box>

      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Stack direction="row" justifyContent="space-between" alignItems="center">
            <Typography variant="body2" sx={{ fontWeight: 600 }}>Базы данных: {usageGB.toFixed(1)} / {limitGB.toFixed(1)} ГБ</Typography>
            <Chip label={limitReached ? 'Лимит' : `Свободно ${(limitGB - usageGB).toFixed(1)} ГБ`} color={limitReached ? 'error' : 'success'} size="small" />
          </Stack>
          <LinearProgress variant="determinate" value={usagePercent} color={limitReached ? 'error' : 'primary'} sx={{ mt: 1.5, height: 8, borderRadius: 999 }} />
          {limitReached && <Alert severity="warning" sx={{ mt: 1.5 }}>Лимит места тарифа исчерпан — новые базы создать нельзя.</Alert>}
        </CardContent>
      </Card>

      {loadingList ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}><CircularProgress /></Box>
      ) : (
        <>
          <Typography variant="h6" className={styles.sectionTitle}>Базы данных</Typography>
          <Box className={styles.grid}>{databases.map(renderCard)}</Box>

          {others.length > 0 && (
            <>
              <Typography variant="h6" className={styles.sectionTitle} sx={{ mt: 4 }}>Другие подключения</Typography>
              <Box className={styles.grid}>{others.map(renderCard)}</Box>
            </>
          )}
        </>
      )}

      {users.length > 1 && (
        <Card sx={{ mt: 3 }}>
          <CardContent>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>Все учётки ({users.length})</Typography>
            <Stack spacing={1.5} divider={<Divider />}>
              {users.map((u) => (
                <Stack key={u.id} direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ sm: 'center' }}>
                  <Box sx={{ flex: 1 }}>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>
                      {u.username}{' '}
                      <Typography component="span" variant="caption" color="text.secondary">· {u.kind} · {u.resource}</Typography>
                    </Typography>
                    <Typography variant="caption" color="text.secondary">{u.host}:{u.port} · создан {formatDate(u.createdAt)}</Typography>
                  </Box>
                </Stack>
              ))}
            </Stack>
          </CardContent>
        </Card>
      )}
    </Box>
  )
}
