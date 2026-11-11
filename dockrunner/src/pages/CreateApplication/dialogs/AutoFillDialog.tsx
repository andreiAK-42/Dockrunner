import type { Dispatch, SetStateAction } from 'react'
import { Dialog, DialogTitle, DialogContent, DialogActions, Typography, Box, RadioGroup, FormControlLabel, Radio, Checkbox } from '@mui/material'
import { Button } from '@mui/material'
import type { CheckStatus } from '../../../api/containers'
import { portSourceLabel } from '../../../utils/docker'
import styles from '../CreateApplication.module.scss'

interface AutoFillDialogProps {
  open: boolean
  onClose: () => void
  autoFillCheck: CheckStatus | null
  autoFillBranch: string
  setAutoFillBranch: (v: string) => void
  autoFillEnvKeys: string[]
  setAutoFillEnvKeys: Dispatch<SetStateAction<string[]>>
  autoFillPortIndices: number[]
  setAutoFillPortIndices: Dispatch<SetStateAction<number[]>>
  pendingBuildMode: 'dockerfile' | 'compose'
  setPendingBuildMode: (v: 'dockerfile' | 'compose') => void
  pendingServices: string[]
  setPendingServices: Dispatch<SetStateAction<string[]>>
  portsRange: { min: number; max: number }
  onApply: () => void
}

export function AutoFillDialog({
  open,
  onClose,
  autoFillCheck,
  autoFillBranch,
  setAutoFillBranch,
  autoFillEnvKeys,
  setAutoFillEnvKeys,
  autoFillPortIndices,
  setAutoFillPortIndices,
  pendingBuildMode,
  setPendingBuildMode,
  pendingServices,
  setPendingServices,
  portsRange,
  onApply,
}: AutoFillDialogProps) {
  if (!open || !autoFillCheck) return null

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth
      slotProps={{ paper: { sx: { bgcolor: 'background.paper', color: 'text.primary', backgroundImage: 'none' } } }}>
      <DialogTitle>Автозаполнение из репозитория</DialogTitle>
      <DialogContent>
        <Typography variant="body1" sx={{ mb: 2 }}>
          Репозиторий проверен. Выберите, какие данные применить к форме:
        </Typography>

        {autoFillCheck.branches && autoFillCheck.branches.length > 0 && (
          <Box className={styles.detectPanel}>
            <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: 600 }}>
              Ветка Git:
            </Typography>
            <RadioGroup value={autoFillBranch} onChange={(e) => setAutoFillBranch(e.target.value)}>
              {autoFillCheck.branches.map((branchName) => (
                <FormControlLabel key={branchName} className={styles.detectItem} value={branchName} control={<Radio />} label={branchName} />
              ))}
            </RadioGroup>
          </Box>
        )}

        {/* Build mode selection */}
        {(autoFillCheck.detected_dockerfile || autoFillCheck.compose_file) && (
          <Box className={styles.detectPanel}>
            <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: 600 }}>
              Способ сборки:
            </Typography>
            <RadioGroup value={pendingBuildMode} onChange={(e) => setPendingBuildMode(e.target.value as 'dockerfile' | 'compose')}>
              {autoFillCheck.detected_dockerfile && <FormControlLabel className={styles.detectItem} value="dockerfile" control={<Radio />} label="Dockerfile — один контейнер" />}
              {autoFillCheck.compose_file && <FormControlLabel className={styles.detectItem} value="compose" control={<Radio />} label={`Docker Compose — ${autoFillCheck.compose_file}`} />}
            </RadioGroup>
            {pendingBuildMode === 'compose' && autoFillCheck.services && autoFillCheck.services.length > 0 && (
              <Box sx={{ mt: 2 }}>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                  Выберите сервисы для запуска:
                </Typography>
                <Box className={styles.detectList}>
                  {autoFillCheck.services.map((service) => (
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
          </Box>
        )}

        {autoFillCheck.detected_env_vars && Object.keys(autoFillCheck.detected_env_vars).length > 0 && (
          <Box className={styles.detectPanel}>
            <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: 600 }}>
              Переменные окружения (из .env / .env.example):
            </Typography>
            <Box className={styles.detectList}>
              {Object.entries(autoFillCheck.detected_env_vars).map(([key, value]) => {
                const isSelected = autoFillEnvKeys.includes(key)
                return (
                  <FormControlLabel
                    key={key}
                    className={styles.detectItem}
                    control={
                      <Checkbox
                        checked={isSelected}
                        onChange={(e) =>
                          setAutoFillEnvKeys((prev) =>
                            e.target.checked ? [...prev, key] : prev.filter((k) => k !== key)
                          )
                        }
                      />
                    }
                    label={
                      <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>{key}={value}</Typography>
                        <Typography variant="caption" className={styles.detectMeta}>Найдено в .env / .env.example</Typography>
                      </Box>
                    }
                  />
                )
              })}
            </Box>
            <Typography variant="caption" className={styles.detectMeta} sx={{ mt: 1, display: 'block' }}>
              Выбранные переменные будут добавлены в форму. Можно редактировать перед деплоем.
            </Typography>
          </Box>
        )}

        {autoFillCheck.detected_ports && autoFillCheck.detected_ports.length > 0 && (
          <Box className={styles.detectPanel}>
            <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: 600 }}>
              Порты контейнеров:
            </Typography>
            <Typography variant="caption" className={styles.detectMeta} sx={{ mb: 1, display: 'block' }}>
              Хост-порты будут автоматически подобраны из свободного диапазона ({portsRange.min}–{portsRange.max}).
            </Typography>
            <Box className={styles.detectList}>
              {autoFillCheck.detected_ports
                .filter((port) => {
                  // Filter ports by selected build mode
                  if (pendingBuildMode === 'dockerfile') {
                    return port.source === 'dockerfile'
                  }
                  if (pendingBuildMode === 'compose') {
                    return port.source === 'compose'
                  }
                  return true
                })
                .map((port) => {
                  // Find original index for selection tracking
                  const originalIndex = autoFillCheck.detected_ports!.findIndex(p => p === port)
                  const isSelected = autoFillPortIndices.includes(originalIndex)
                  return (
                    <FormControlLabel
                      key={originalIndex}
                      className={styles.detectItem}
                      control={
                        <Checkbox
                          checked={isSelected}
                          onChange={(e) =>
                            setAutoFillPortIndices((prev) =>
                              e.target.checked ? [...prev, originalIndex] : prev.filter((i) => i !== originalIndex)
                            )
                          }
                        />
                      }
                      label={
                        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                          {port.container_port}/{port.protocol}{' '}
                          <Typography component="span" variant="body2" className={styles.detectMeta}>
                            (из {portSourceLabel(port)})
                          </Typography>
                          {' '}→ авто-порт на хосте
                        </Typography>
                      }
                    />
                  )
                })}
            </Box>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="outlined">Отмена</Button>
        <Button onClick={onApply} variant="contained" color="primary" disabled={!autoFillBranch && autoFillEnvKeys.length === 0 && autoFillPortIndices.length === 0}>
          Применить
        </Button>
      </DialogActions>
    </Dialog>
  )
}