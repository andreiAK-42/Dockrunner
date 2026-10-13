import { useState, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Box, Button, Card, CardContent, Stack, Typography, Chip, Tabs, Tab, Divider, IconButton, TextField, MenuItem, Select, FormControl, InputLabel } from '@mui/material'
import { ArrowLeft, Play, Square, RotateCcw, Terminal, Copy, Download, Trash2, HardDrive, Globe, Box as BoxIcon } from 'lucide-react'
import { getContainerById } from '../../types/container'
import type { ContainerVolume, Container } from '../../types/container'
import { copyText } from '../../utils/clipboard'
import { useEnvConfigs } from '../../hooks/useEnvConfigs'
import { toDockerEnvArgs } from '../../types/envConfig'
import type { EnvConfig } from '../../types/envConfig'
import styles from './ContainerDetails.module.scss'

const DEFAULT_NEW_VOLUME_SIZE_GB = 5

function filterLogsBySearch(logs: string[], search: string): string[] {
  const query = search.trim().toLowerCase()
  if (!query) {
    return logs
  }
  return logs.filter((line) => line.toLowerCase().includes(query))
}

function buildInlineEnvArgs(container: Container): string {
  const args = container.env.map((envVar) => `--env ${envVar.key}="${envVar.value}"`)
  return args.join(' ')
}

function buildDockerCommand(container: Container, volumes: ContainerVolume[], selectedEnv: EnvConfig | undefined): string {
  const portArgs = container.ports.map((port) => `-p ${port.hostPort}:${port.containerPort}/${port.protocol}`).join(' ')
  const volumeArgs = volumes.map((volume) => `-v ${volume.name}:${volume.mountPath}`).join(' ')

  let envArgs = buildInlineEnvArgs(container)
  if (selectedEnv) {
    envArgs = toDockerEnvArgs(selectedEnv)
  }

  let restartArg = ''
  if (container.restartPolicy !== 'no') {
    restartArg = `--restart ${container.restartPolicy}`
  }

  const command = container.command || ''
  const fullCommand = `docker run -d --name ${container.name} ${portArgs} ${volumeArgs} ${envArgs} ${restartArg} ${container.image} ${command}`

  return fullCommand.replace(/\s+/g, ' ').trim()
}

function downloadTextFile(fileName: string, text: string) {
  const blob = new Blob([text], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.click()
  URL.revokeObjectURL(url)
}

function getStatusColor(status: string): 'success' | 'error' | 'default' {
  if (status === 'running') {
    return 'success'
  }
  if (status === 'error') {
    return 'error'
  }
  return 'default'
}

export default function ContainerDetails() {
  const { id, containerId } = useParams()
  const realId = containerId || id
  const navigate = useNavigate()
  const container = getContainerById(realId || '')
  const { configs } = useEnvConfigs()

  const [tab, setTab] = useState(0)
  const [follow, setFollow] = useState(true)
  const [search, setSearch] = useState('')
  const [envConfigId, setEnvConfigId] = useState('')
  const [volumes, setVolumes] = useState<ContainerVolume[]>([])
  const [volumesFor, setVolumesFor] = useState<string | undefined>(undefined)
  const [newVolName, setNewVolName] = useState('')
  const [newVolPath, setNewVolPath] = useState('')

  // Сбрасываем локальные правки томов и env при переходе на другой контейнер.
  // Сравнение во время рендера — штатный приём React для такого сброса.
  if (volumesFor !== realId) {
    setVolumesFor(realId)
    setVolumes(container?.volumes || [])
    setEnvConfigId(container?.envConfigId || '')
  }

  const filteredLogs = useMemo(() => {
    const allLogs = container?.logs || []
    return filterLogsBySearch(allLogs, search)
  }, [container, search])

  const selectedEnv = configs.find((config) => config.id === envConfigId)
  const dockerCmd = useMemo(() => {
    if (!container) {
      return ''
    }
    return buildDockerCommand(container, volumes, selectedEnv)
  }, [container, volumes, selectedEnv])

  const copyDocker = () => {
    copyText(dockerCmd)
  }

  const copyFilteredLogs = () => {
    copyText(filteredLogs.join('\n'))
  }

  const downloadLogs = () => {
    if (!container) {
      return
    }
    const logText = filteredLogs.join('\n')
    downloadTextFile(`${container.name}.log`, logText)
  }

  const handleAddVolume = () => {
    const volumeName = newVolName.trim()
    const mountPath = newVolPath.trim()
    if (!volumeName || !mountPath) {
      return
    }

    const newVolume: ContainerVolume = {
      id: `vol-${Date.now()}`,
      name: volumeName,
      mountPath,
      sizeGB: DEFAULT_NEW_VOLUME_SIZE_GB,
    }
    setVolumes([...volumes, newVolume])
    setNewVolName('')
    setNewVolPath('')
  }

  const handleRemoveVolume = (volumeId: string) => {
    setVolumes(volumes.filter((volume) => volume.id !== volumeId))
  }

  const handleToggleFollow = () => {
    setFollow(!follow)
  }

  if (!container) {
    return (
      <Box className={styles.page}>
        <Button startIcon={<ArrowLeft size={16} />} onClick={() => navigate(-1)}>Назад</Button>
        <Card><CardContent>Контейнер не найден: {realId}</CardContent></Card>
      </Box>
    )
  }

  const statusColor = getStatusColor(container.status)
  const followLabel = follow ? 'Слежение: вкл' : 'Слежение: выкл'
  const followColor = follow ? 'success' : 'default'
  const volumeArgsPreview = volumes.map((volume) => `-v ${volume.name}:${volume.mountPath}`).join(' ') || '# нет томов'
  const canAddVolume = !newVolName.trim() || !newVolPath.trim()

  return (
    <Box className={styles.page}>
      <Button startIcon={<ArrowLeft size={16} />} onClick={() => navigate(-1)} className={styles.back}>Назад</Button>

      <Box className={styles.header}>
        <Box>
          <Typography variant="h4" className={styles.header__title}>
            {container.name}
            <Chip size="small" color={statusColor} label={container.status} />
            <Chip size="small" variant="outlined" label={container.image} />
          </Typography>
          <Box className={styles.header__meta}>
            <Typography variant="body2">{container.nodeId} • {container.id}</Typography>
            <Typography variant="caption">{new Date(container.createdAt).toLocaleString()}</Typography>
          </Box>
        </Box>
        <Stack direction="row" spacing={1}>
          <Button variant="outlined" startIcon={<RotateCcw size={16} />}>Перезапустить</Button>
          <Button variant="contained" color="success" startIcon={<Play size={16} />}>Старт</Button>
          <Button variant="contained" color="error" startIcon={<Square size={16} />}>Стоп</Button>
        </Stack>
      </Box>

      <Card className={styles.tabsCard}>
        <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ px: 2 }}>
          <Tab label="Обзор" />
          <Tab label="Логи" />
          <Tab label="Параметры запуска" />
          <Tab label="Тома" />
        </Tabs>
        <Divider />
      </Card>

      {tab === 0 && (
        <Box className={styles.grid}>
          <Stack spacing={2}>
            <Card><CardContent>
              <Typography variant="h6" sx={{ fontWeight: 650, mb: 1 }}>Команда запуска</Typography>
              <Box className={styles.dockerPreview}>{dockerCmd}</Box>
              <Button size="small" sx={{ mt: 1 }} startIcon={<Copy size={14} />} onClick={copyDocker}>Копировать команду</Button>
            </CardContent></Card>
            <Card><CardContent>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>Порты</Typography>
              <Box className={styles.portList}>
                {container.ports.length === 0 ? (
                  <Typography variant="body2" color="text.secondary">Нет проброшенных портов</Typography>
                ) : (
                  container.ports.map((port) => (
                    <Chip
                      key={port.hostPort}
                      icon={<Globe size={14} />}
                      label={`${port.hostPort} → ${port.containerPort}/${port.protocol}`}
                      size="small"
                      color="primary"
                      variant="outlined"
                    />
                  ))
                )}
              </Box>
            </CardContent></Card>
          </Stack>
          <Stack spacing={2}>
            <Card><CardContent>
              <Stack className={styles.stat} direction="row"><Box className={`${styles.stat__icon} ${styles['stat__icon--primary']}`}><BoxIcon size={18} /></Box><Box><Typography variant="body2" color="text.secondary">Образ</Typography><Typography sx={{ fontWeight: 600 }}>{container.image}</Typography></Box></Stack>
              <Divider sx={{ my: 1.5 }} />
              <Box className={styles.kv}>
                <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Рестарт</span><span className={styles.kv__rowValue}>{container.restartPolicy}</span></Box>
                <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Узел</span><span className={styles.kv__rowValue}>{container.nodeId}</span></Box>
                <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Env-конфиг</span><span className={styles.kv__rowValue}>{selectedEnv?.name || 'встроенные env'}</span></Box>
              </Box>
            </CardContent></Card>
          </Stack>
        </Box>
      )}

      {tab === 1 && (
        <Card>
          <Box className={styles.logs}>
            <Box className={styles.logs__toolbar}>
              <Terminal size={16} />
              <Typography variant="caption" sx={{ fontWeight: 600 }}>Логи контейнера</Typography>
              <input className={styles.logs__search} placeholder="Поиск по логам..." value={search} onChange={e => setSearch(e.target.value)} />
              <Chip size="small" label={followLabel} onClick={handleToggleFollow} color={followColor} sx={{ cursor: 'pointer' }} />
              <IconButton size="small" onClick={downloadLogs} sx={{ color: '#e2e8f0' }}><Download size={16} /></IconButton>
              <IconButton size="small" onClick={copyFilteredLogs} sx={{ color: '#e2e8f0' }}><Copy size={14} /></IconButton>
            </Box>
            <Box className={styles.logs__body}>
              {filteredLogs.length === 0 ? (
                <Typography variant="body2" sx={{ opacity: .6 }}>Нет логов по фильтру</Typography>
              ) : (
                filteredLogs.map((line, lineIndex) => <div key={lineIndex} className={styles.logs__line}>{line}</div>)
              )}
              {follow && <Typography variant="caption" sx={{ opacity: .5, display: 'block', mt: 1 }}>— режим слежения, новые строки появятся автоматически —</Typography>}
            </Box>
          </Box>
          <CardContent>
            <Typography variant="caption" color="text.secondary">Логи получаются через `docker logs {container.name}`. В реальном бэкенде — стриминг по WebSocket.</Typography>
          </CardContent>
        </Card>
      )}

      {tab === 2 && (
        <Stack spacing={2}>
          <Card><CardContent>
            <Typography variant="h6" sx={{ fontWeight: 650, mb: 2 }}>Параметры запуска</Typography>
            <Box className={styles.kv}>
              <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Образ</span><span className={styles.kv__rowValue}>{container.image}</span></Box>
              <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Команда</span><span className={styles.kv__rowValue}>{container.command || '— (по умолчанию)'}</span></Box>
              <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Политика рестарта</span><span className={styles.kv__rowValue}>{container.restartPolicy}</span></Box>
              <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Создан</span><span className={styles.kv__rowValue}>{new Date(container.createdAt).toLocaleString()}</span></Box>
            </Box>

            <Typography variant="subtitle1" sx={{ fontWeight: 600, mt: 3, mb: 1 }}>Env-конфигурация</Typography>
            <FormControl fullWidth size="small" sx={{ mb: 1 }}>
              <InputLabel>Выбрать сохранённый env-конфиг</InputLabel>
              <Select value={envConfigId} label="Выбрать сохранённый env-конфиг" onChange={e => setEnvConfigId(e.target.value)}>
                <MenuItem value=""><em>— Использовать встроенные env —</em></MenuItem>
                {configs.map((config) => <MenuItem key={config.id} value={config.id}>{config.name} ({config.vars.length} перем.)</MenuItem>)}
              </Select>
            </FormControl>
            {selectedEnv ? (
              <Box className={styles.dockerPreview}>{toDockerEnvArgs(selectedEnv)}</Box>
            ) : (
              <Box className={styles.kv}>
                {container.env.length === 0 ? (
                  <Typography variant="body2" color="text.secondary">Нет env</Typography>
                ) : (
                  container.env.map((envVar) => (
                    <Box key={envVar.key} className={styles.kv__row}>
                      <span className={styles.kv__rowLabel}>{envVar.key}</span>
                      <span className={styles.kv__rowValue}>{envVar.value}</span>
                    </Box>
                  ))
                )}
              </Box>
            )}
            <Box className={styles.dockerPreview} sx={{ mt: 2 }}>{dockerCmd}</Box>
            <Button size="small" startIcon={<Copy size={14} />} sx={{ mt: 1 }} onClick={copyDocker}>Копировать docker run</Button>
          </CardContent></Card>
        </Stack>
      )}

      {tab === 3 && (
        <Card><CardContent>
          <Typography variant="h6" sx={{ fontWeight: 650, mb: 1, display: 'flex', alignItems: 'center', gap: 1 }}><HardDrive size={18} /> Тома (в настройках контейнера)</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Тома управляются внутри контейнера. Добавляйте маунты — они попадут в `docker run -v`.</Typography>
          <Box className={styles.volumeList}>
            {volumes.map((volume) => {
              const readOnlySuffix = volume.readOnly ? '• только чтение' : ''
              return (
                <Box key={volume.id} sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', p: 1.5, border: '1px solid', borderColor: 'divider', borderRadius: 2 }}>
                  <Box>
                    <Typography sx={{ fontWeight: 600 }}>{volume.name}</Typography>
                    <Typography variant="caption" color="text.secondary">{volume.mountPath} • {volume.sizeGB} ГБ {readOnlySuffix}</Typography>
                  </Box>
                  <IconButton size="small" color="error" onClick={() => handleRemoveVolume(volume.id)}><Trash2 size={16} /></IconButton>
                </Box>
              )
            })}
            {volumes.length === 0 && <Typography variant="body2" color="text.secondary">Нет томов</Typography>}
          </Box>

          <Divider sx={{ my: 2 }} />
          <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>Добавить том</Typography>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
            <TextField size="small" label="Имя тома" value={newVolName} onChange={e => setNewVolName(e.target.value)} placeholder="my-data" sx={{ flex: 1 }} />
            <TextField size="small" label="Путь монтирования" value={newVolPath} onChange={e => setNewVolPath(e.target.value)} placeholder="/data" sx={{ flex: 1 }} />
            <Button variant="contained" onClick={handleAddVolume} disabled={canAddVolume}>Добавить</Button>
          </Stack>

          <Box className={styles.dockerPreview} sx={{ mt: 2 }}>
            {volumeArgsPreview}
          </Box>
        </CardContent></Card>
      )}
    </Box>
  )
}
