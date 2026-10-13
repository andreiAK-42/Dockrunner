import { useState } from 'react'
import { Box, Button, Card, CardContent, TextField, Typography, Stack, Alert, InputAdornment, IconButton } from '@mui/material'
import { Server, Eye, EyeOff, LogIn } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../../auth/AuthContext'
import styles from './Login.module.scss'

export default function Login() {
  const [show, setShow] = useState(false)
  const [loginText, setLoginText] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()
  const { login } = useAuth()

  const passwordInputType = show ? 'text' : 'password'

  const handleToggleShow = () => {
    setShow(!show)
  }

  const handleLogin = async () => {
    const trimmedLogin = loginText.trim()
    if (!trimmedLogin || !password) {
      setError('Заполните все поля')
      return
    }

    setBusy(true)
    setError('')

    try {
      await login(trimmedLogin, password)
      navigate('/dashboard')
    } catch (loginError) {
      if (loginError instanceof Error) {
        setError(loginError.message)
      } else {
        setError('Не удалось войти')
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Box className={styles.page}>
      <Card className={styles.card}>
        <CardContent className={styles.card__content}>
          <Stack alignItems="center" spacing={1.5} className={styles.brand}>
            <Link to="/" aria-label="На главную">
              <Box className={styles.brand__logo}><Server size={24} /></Box>
            </Link>
            <Typography variant="h5" className={styles.brand__title}>Добро пожаловать</Typography>
          </Stack>

          {error && <Alert severity="error" className={styles.alert}>{error}</Alert>}

          <Stack className={styles.form}>
            <TextField label="Логин" value={loginText} onChange={e => setLoginText(e.target.value)} fullWidth placeholder="admin" />
            <TextField
              label="Пароль"
              type={passwordInputType}
              value={password}
              onChange={e => setPassword(e.target.value)}
              fullWidth
              InputProps={{
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton onClick={handleToggleShow} edge="end" size="small">{show ? <EyeOff size={18} /> : <Eye size={18} />}</IconButton>
                  </InputAdornment>
                ),
              } as any}
            />
            <Button variant="contained" size="large" fullWidth startIcon={<LogIn size={18} />} disabled={busy} onClick={handleLogin} className={styles.submitBtn}>Войти</Button>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
