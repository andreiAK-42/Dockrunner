import { useEffect, useState } from 'react'
import { Box, Button, Card, CardContent, Stack, Typography, CircularProgress, IconButton, LinearProgress, Chip, Alert, Divider } from '@mui/material'
import { Database, Copy, KeyRound, Trash2 } from 'lucide-react'
import { fetchDatabases, fetchMyDbUsers, createDatabaseUser, deleteDatabaseUser } from '../../api/databases'
import type { DatabaseInfo, DbId, DbUser, DbUserResult } from '../../api/databases'
import { copyText } from '../../utils/clipboard'
import styles from './Databases.module.scss'

function formatDate(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ru-RU')
}

export default function Databases() {
  const [databases, setDatabases] = useState<DatabaseInfo[]>([])
  const [users, setUsers] = useState<DbUser[]>([])
  const [usageGB, setUsageGB] = useState(0)
  const [limitGB, setLimitGB] = useState(0)
  const [loadingList, setLoadingList] = useState(true)
  const [fresh, setFresh] = useState<Partial<Record<DbId, DbUserResult>>>({})
  const [creating, setCreating] = useState<DbId | null>(null)
  const [error, setError] = useState('')

  const reload = () => {
    fetchDatabases().then(setDatabases).catch(() => setError('Не удалось загрузить список СУБД'))
    fetchMyDbUsers()
      .then((res) => {
        setUsers(res.users)
        setUsageGB(res.usage_gb)
        setLimitGB(res.limit_gb)
      })
      .catch(() => setError('Не удалось загрузить ваших пользователей СУБД'))
      .finally(() => setLoadingList(false))
  }

  useEffect(reload, [])

  const handleCreate = (db: DbId) => {
    setCreating(db)
    setError('')
    createDatabaseUser(db)
      .then((result) => {
        setFresh((prev) => ({ ...prev, [db]: result }))
        return fetchMyDbUsers()
      })
      .then((res) => {
        setUsers(res.users)
        setUsageGB(res.usage_gb)
        setLimitGB(res.limit_gb)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось создать пользователя'))
      .finally(() => setCreating(null))
  }

  const handleDelete = (user: DbUser) => {
    if (!window.confirm(`Удалить пользователя ${user.username} (${user.dbType})? База ${user.dbName} тоже будет удалена.`)) {
      return
    }
    setError('')
    deleteDatabaseUser(user.id)
      .then(() => fetchMyDbUsers())
      .then((res) => {
        setUsers(res.users)
        setUsageGB(res.usage_gb)
        setLimitGB(res.limit_gb)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Не удалось удалить'))
  }

  const limitReached = limitGB > 0 && usageGB >= limitGB
  const usagePercent = limitGB > 0 ? Math.min(100, (usageGB / limitGB) * 100) : 0

  return (
    <Box className={styles.page}>
      <Box className={styles.header}>
        <Typography variant="h4" className={styles.header__title}>Базы данных</Typography>
        <Typography color="text.secondary">Создайте пользователя — логин сгенерируется, пароль покажется один раз, строки подключения соберите в .env приложения.</Typography>
      </Box>

      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Stack direction="row" justifyContent="space-between" alignItems="center">
            <Typography variant="body2" sx={{ fontWeight: 600 }}>Занято {usageGB.toFixed(1)} / {limitGB.toFixed(1)} ГБ</Typography>
            <Chip label={limitReached ? 'Лимит' : `Свободно ${(limitGB - usageGB).toFixed(1)} ГБ`} color={limitReached ? 'error' : 'success'} size="small" />
          </Stack>
          <LinearProgress variant="determinate" value={usagePercent} color={limitReached ? 'error' : 'primary'} sx={{ mt: 1.5, height: 8, borderRadius: 999 }} />
          {limitReached && <Alert severity="warning" sx={{ mt: 1.5 }}>Лимит места исчерпан — новые базы создать нельзя.</Alert>}
        </CardContent>
      </Card>

      {loadingList ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}><CircularProgress /></Box>
      ) : (
        <Box className={styles.grid}>
          {databases.map((db) => {
            const busy = creating === db.id
            const lastCreated = fresh[db.id]
            return (
              <Card key={db.id}>
                <CardContent>
                  <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 1 }}>
                    <Box className={styles.dbIcon}><Database size={20} /></Box>
                    <Box>
                      <Typography variant="h6" sx={{ fontWeight: 700 }}>{db.name}</Typography>
                      <Typography variant="caption" color="text.secondary">Порт: {db.defaultPort}</Typography>
                    </Box>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>{db.description}</Typography>

                  <Button variant="contained" startIcon={<KeyRound size={16} />} disabled={busy || limitReached} onClick={() => handleCreate(db.id)} fullWidth>
                    {busy ? 'Создание...' : 'Создать пользователя'}
                  </Button>

                  {lastCreated && (
                    <Stack spacing={1.5} sx={{ mt: 2 }}>
                      <Alert severity="success">Пользователь создан. Сохраните пароль — он показан один раз.</Alert>
                      <Box className={styles.creds}>
                        {lastCreated.rows.map((row) => (
                          <Box key={row.label} className={styles.creds__row}>
                            <Typography variant="body2" color="text.secondary">{row.label}</Typography>
                            <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: 'monospace' }}>{row.value}</Typography>
                          </Box>
                        ))}
                      </Box>
                      {lastCreated.connectionString && (
                        <Box className={styles.conn}>
                          <Typography variant="caption" color="text.secondary">Строка подключения (в .env приложения)</Typography>
                          <Box className={styles.conn__row}>
                            <Typography variant="body2" sx={{ fontFamily: 'monospace', wordBreak: 'break-all' }}>{lastCreated.connectionString}</Typography>
                            <IconButton size="small" onClick={() => copyText(lastCreated.connectionString || '')}><Copy size={14} /></IconButton>
                          </Box>
                        </Box>
                      )}
                    </Stack>
                  )}
                </CardContent>
              </Card>
            )
          })}
        </Box>
      )}

      <Card sx={{ mt: 3 }}>
        <CardContent>
          <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>Мои пользователи СУБД ({users.length})</Typography>
          {users.length === 0 ? (
            <Typography variant="body2" color="text.secondary">Пока никого — создайте первого пользователя выше.</Typography>
          ) : (
            <Stack spacing={1.5} divider={<Divider />}>
              {users.map((u) => (
                <Stack key={u.id} direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ sm: 'center' }}>
                  <Box sx={{ flex: 1 }}>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>{u.username} <Typography component="span" variant="caption" color="text.secondary">· {u.dbType} · {u.dbName}</Typography></Typography>
                    <Typography variant="caption" color="text.secondary">{u.host}:{u.port} · {u.sizeGB.toFixed(1)} ГБ · создан {formatDate(u.createdAt)}</Typography>
                  </Box>
                  <Button size="small" color="error" startIcon={<Trash2 size={14} />} onClick={() => handleDelete(u)}>Удалить</Button>
                </Stack>
              ))}
            </Stack>
          )}
        </CardContent>
      </Card>
    </Box>
  )
}
