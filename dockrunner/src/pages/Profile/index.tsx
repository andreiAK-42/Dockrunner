import { Box, Button, Card, CardContent, Stack, Typography, Avatar, Chip, Divider } from '@mui/material'
import { User as UserIcon, LogOut, Crown, HardDrive } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { useAuth } from '../../auth/AuthContext'
import { TARIFF } from '../../config/tariff'
import { fetchStorageUsage } from '../../api/storage'
import type { StorageUsage } from '../../api/storage'
import UserQuotas from '../../components/UserQuotas'
import styles from './Profile.module.scss'

function getInitial(name: string) {
  if (!name) {
    return 'Г'
  }
  const firstLetter = name[0]?.toUpperCase()
  if (firstLetter) {
    return firstLetter
  }
  return 'Г'
}

export default function Profile() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [storage, setStorage] = useState<StorageUsage | null>(null)

  const userName = user?.name || 'Гость'
  const userLogin = user?.login || 'не авторизован'

  useEffect(() => {
    fetchStorageUsage()
      .then((res) => setStorage(res.data))
      .catch(() => setStorage(null))
  }, [])

  const handleLogout = () => {
    logout()
    navigate('/')
  }

  return (
    <Box className={styles.page}>
      <Box className={styles.header}>
        <Typography variant="h4" className={styles.header__title}>Профиль</Typography>
        <Typography color="text.secondary">Информация о пользователе и тарифном плане.</Typography>
      </Box>

      <Box className={styles.grid}>
        <Card>
          <CardContent>
            <Stack direction="row" spacing={2} alignItems="center" sx={{ mb: 2 }}>
              <Avatar sx={{ width: 56, height: 56 }}>{getInitial(userName)}</Avatar>
              <Box>
                <Typography variant="h6" sx={{ fontWeight: 700, display: 'flex', alignItems: 'center', gap: 1 }}>
                  <UserIcon size={18} /> {userName}
                </Typography>
                <Typography variant="body2" color="text.secondary">{userLogin}</Typography>
              </Box>
            </Stack>
            <Divider sx={{ my: 2 }} />
            <Button variant="outlined" color="error" startIcon={<LogOut size={16} />} onClick={handleLogout} fullWidth>
              Выйти
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
              <Crown size={18} /> Тарифный план
            </Typography>
            <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
              <Chip label={TARIFF.name} color="primary" />
              <Typography variant="body2" color="text.secondary">{TARIFF.price}</Typography>
            </Stack>
            <Stack spacing={1}>
              <Typography variant="body2">Приложений: до {TARIFF.apps}</Typography>
              <Typography variant="body2">Доменов: до {TARIFF.domains}</Typography>
              <Typography variant="body2">Портов: до {TARIFF.ports}</Typography>
              <Typography variant="body2">Env-конфигов: до {TARIFF.envConfigs}</Typography>
              <Typography variant="body2">Место под базы данных: до {TARIFF.dbStorageGB} ГБ</Typography>
            </Stack>
          </CardContent>
        </Card>
      </Box>

      <Card>
        <CardContent>
          <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>Лимиты профиля</Typography>
          <UserQuotas />
        </CardContent>
      </Card>

      {storage && (
        <Card>
          <CardContent>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
              <HardDrive size={18} /> Занятое место
            </Typography>
            <Typography variant="h5" sx={{ fontWeight: 700 }}>{storage.total_gb.toFixed(1)} ГБ</Typography>
            <Typography variant="body2" color="text.secondary">
              Контейнеры: {storage.containers_gb.toFixed(1)} ГБ · Личные папки: {storage.files_gb.toFixed(1)} ГБ
            </Typography>
          </CardContent>
        </Card>
      )}
    </Box>
  )
}
