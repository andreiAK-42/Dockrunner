import type { Dispatch, SetStateAction } from 'react'
import { Dialog, DialogTitle, DialogContent, DialogActions, Typography, Box, RadioGroup, FormControlLabel, Radio, Checkbox } from '@mui/material'
import { Button } from '@mui/material'
import type { CheckStatus } from '../../../api/containers'
import styles from '../CreateApplication.module.scss'

interface BuildModeDialogProps {
  open: boolean
  onClose: () => void
  check: CheckStatus | null
  pendingBuildMode: 'dockerfile' | 'compose'
  setPendingBuildMode: (v: 'dockerfile' | 'compose') => void
  pendingServices: string[]
  setPendingServices: Dispatch<SetStateAction<string[]>>
  portsRange: { min: number; max: number }
  onConfirm: () => void
}

export function BuildModeDialog({
  open,
  onClose,
  check,
  pendingBuildMode,
  setPendingBuildMode,
  pendingServices,
  setPendingServices,
  portsRange,
  onConfirm,
}: BuildModeDialogProps) {
  if (!open) return null

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth
      slotProps={{ paper: { sx: { bgcolor: 'background.paper', color: 'text.primary', backgroundImage: 'none' } } }}>
      <DialogTitle>Выбор способа сборки и запуска</DialogTitle>
      <DialogContent>
        <Typography variant="body1" sx={{ mb: 2 }}>
          Репозиторий проверен. Выберите способ сборки:
        </Typography>

        {check?.detected_env_vars && Object.keys(check.detected_env_vars).length > 0 && (
          <Box className={styles.detectPanel}>
            <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: 600 }}>
              Обнаруженные переменные окружения (из .env / .env.example):
            </Typography>
            <Box className={styles.detectList}>
              {Object.entries(check.detected_env_vars).map(([key, value]) => (
                <Typography key={key} variant="body2" sx={{ fontFamily: 'monospace' }}>
                  {key}={value}
                </Typography>
              ))}
            </Box>
            <Typography variant="caption" className={styles.detectMeta} sx={{ mt: 1, display: 'block' }}>
              Эти переменные будут автоматически подставлены в форму. Их можно изменить перед деплоем.
            </Typography>
          </Box>
        )}

        {check?.detected_ports && check.detected_ports.length > 0 && (
          <Box className={styles.detectPanel}>
            <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: 600 }}>
              Обнаруженные порты контейнеров:
            </Typography>
            <Typography variant="caption" className={styles.detectMeta} sx={{ mb: 1, display: 'block' }}>
              Хост-порты будут автоматически подобраны из свободного диапазона ({portsRange.min}–{portsRange.max}) при деплое.
            </Typography>
            <Box className={styles.detectList}>
              {check.detected_ports.map((port, index) => (
                <Typography key={index} variant="body2" sx={{ fontFamily: 'monospace' }}>
                  {port.container_port}/{port.protocol}{' '}
                  <Typography component="span" variant="body2" className={styles.detectMeta}>
                    (из {port.source === 'dockerfile' ? 'Dockerfile EXPOSE' : port.service ? `docker-compose: ${port.service}` : 'docker-compose'})
                  </Typography>
                  {' '}→ авто-порт на хосте
                </Typography>
              ))}
            </Box>
          </Box>
        )}

        <RadioGroup value={pendingBuildMode} onChange={(e) => setPendingBuildMode(e.target.value as 'dockerfile' | 'compose')}>
          {check?.detected_dockerfile && <FormControlLabel className={styles.detectItem} value="dockerfile" control={<Radio />} label="Dockerfile — один контейнер" />}
          {!!check?.compose_file && <FormControlLabel className={styles.detectItem} value="compose" control={<Radio />} label={`Docker Compose — ${check.compose_file}`} />}
        </RadioGroup>
        {pendingBuildMode === 'compose' && pendingServices.length > 0 && (
          <Box sx={{ mt: 2 }}>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              Выберите сервисы, которые нужно запустить:
            </Typography>
            <Box className={styles.detectList}>
              {pendingServices.map((service) => (
                <FormControlLabel
                  key={service}
                  className={styles.detectItem}
                  control={
                    <Checkbox
                      checked={pendingServices.includes(service)}
                      onChange={(e) =>
                        setPendingServices((prev) =>
                          e.target.checked ? [...prev, service] : prev.filter((item) => item !== service)
                        )
                      }
                    />
                  }
                  label={service}
                />
              ))}
            </Box>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="outlined">Отмена</Button>
        <Button onClick={onConfirm} variant="contained" color="primary" disabled={pendingBuildMode === 'compose' && pendingServices.length === 0}>
          Подтвердить
        </Button>
      </DialogActions>
    </Dialog>
  )
}