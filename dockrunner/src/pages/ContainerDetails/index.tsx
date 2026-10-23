import { useState, useEffect, useMemo, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Box, Button, Card, CardContent, Stack, Typography, Chip, Tabs, Tab, Divider, IconButton, TextField, MenuItem, Select, FormControl, InputLabel, Alert } from '@mui/material'
import { ArrowLeft, Play, Square, RotateCcw, Terminal, Copy, Download, Trash2, HardDrive, Globe, Box as BoxIcon, ExternalLink } from 'lucide-react'
import { fetchMine, fetchContainerSpec, fetchContainerLogs, startContainer, stopContainer, removeContainer, fetchBuildLog, type MineContainer, type ContainerSpec } from '../../api/containers'
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

function buildInlineEnvArgs(env: Record<string, string>): string {
  const args = Object.entries(env).map(([key, value]) => `--env ${key}="${value}"`)
  return args.join(' ')
}

function buildDockerCommand(spec: ContainerSpec, selectedEnv: EnvConfig | undefined): string {
  const portArgs = spec.ports.map((port) => `-p ${port.host}:${port.container}`).join(' ')
  const volumeArgs = spec.volumes.map((volume) => `-v ${volume.name || volume.host_path}:${volume.container_path}`).join(' ')

  let envArgs = buildInlineEnvArgs(spec.env)
  if (selectedEnv) {
    envArgs = toDockerEnvArgs(selectedEnv)
  }

  const fullCommand = `docker run -d --name ${spec.name} ${portArgs} ${volumeArgs} ${envArgs} ${spec.image}`

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

function getAppUrl(nodeId: string, ports: { host: number; container: number }[]): string | null {
  if (ports.length === 0) return null
  const port = ports[0].host
  return `http://${nodeId}:${port}`
}

export default function ContainerDetails() {
  const { id, containerId } = useParams()
  const realId = containerId || id
  const navigate = useNavigate()
  const { configs } = useEnvConfigs()

  const [tab, setTab] = useState(0)
  const [follow, setFollow] = useState(true)
  const [search, setSearch] = useState('')
  const [envConfigId, setEnvConfigId] = useState('')
  const [volumes, setVolumes] = useState<Array<{ id: string; name: string; mountPath: string; sizeGB: number; readOnly?: boolean }>>([])
  const [volumesFor, setVolumesFor] = useState<string | undefined>(undefined)
  const [newVolName, setNewVolName] = useState('')
  const [newVolPath, setNewVolPath] = useState('')
  const [record, setRecord] = useState<MineContainer | null>(null)
  const [spec, setSpec] = useState<ContainerSpec | null>(null)
  const [logs, setLogs] = useState<string[] | null>(null)
  const [logsLoadedFor, setLogsLoadedFor] = useState('')
  const [buildLog, setBuildLog] = useState<string | null>(null)
  const [buildLogError, setBuildLogError] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const buildLogBoxRef = useRef<HTMLDivElement | null>(null)
  const cancelWaitRef = useRef(false)

  useEffect(() => {
    cancelWaitRef.current = false
    return () => {
      cancelWaitRef.current = true
    }
  }, [])

  useEffect(() => {
    const loadRecord = async () => {
      try {
        const res = await fetchMine()
        const found = res.data.find((c) => c.id === realId) || null
        setRecord(found)
        if (!found) {
          setError('Контейнер не найден')
        }
      } catch {
        setError('Ошибка загрузки контейнера')
      }
    }
    loadRecord()
    const timer = setInterval(loadRecord, 5000)
    return () => clearInterval(timer)
  }, [realId])

  useEffect(() => {
    if (!record) {
      return
    }
    fetchContainerSpec(record.id)
      .then((res) => setSpec(res.data))
      .catch(() => setSpec(null))
  }, [record])

  const wantLogs = tab === 1 && record ? record.id : ''
  const isStaleLogs = logsLoadedFor !== wantLogs
  const logsLoading = wantLogs !== '' && (logs === null || isStaleLogs)

  useEffect(() => {
    if (!wantLogs) {
      return
    }
    const loadingFor = wantLogs
    let alive = true
    fetchContainerLogs(loadingFor)
      .then((res) => {
        if (!alive) return
        setLogs(res.data.split('\n'))
        setLogsLoadedFor(loadingFor)
      })
      .catch(() => {
        if (!alive) return
        setLogs([])
        setLogsLoadedFor(loadingFor)
      })
    return () => {
      alive = false
    }
  }, [wantLogs])

  const isBuilding = record?.status === 'building'
  const recordId = record?.id

  useEffect(() => {
    if (!isBuilding || !recordId) {
      setBuildLog(null)
      setBuildLogError('')
      return
    }

    let alive = true
    let socket: WebSocket | null = null
    let pollTimer: ReturnType<typeof setInterval> | null = null
    let gotData = false

    const appendLine = (line: string) => {
      if (!alive) return
      gotData = true
      setBuildLogError('')
      setBuildLog((prev) => {
        if (prev === null || prev === '') {
          return line
        }
        return `${prev}\n${line}`
      })
    }

    const loadBuildLogOnce = () => {
      fetchBuildLog(recordId)
        .then((res) => {
          if (!alive) return
          setBuildLog(res.data)
          setBuildLogError('')
        })
        .catch((loadError) => {
          if (!alive) return
          if (loadError instanceof Error && 'status' in loadError && (loadError as any).status === 404) {
            setBuildLogError((loadError as any).message)
          }
        })
    }

    const startPollingFallback = () => {
      if (pollTimer !== null) return
      loadBuildLogOnce()
      pollTimer = setInterval(loadBuildLogOnce, 2000)
    }

    const openStream = () => {
      try {
        const httpBase = (import.meta.env.VITE_API_BASE || '').replace(/\/+$/, '')
        const wsBase = httpBase.replace(/^http/, 'ws')
        const token = encodeURIComponent(localStorage.getItem('token') || '')
        socket = new WebSocket(`${wsBase}/api/v1/containers/${recordId}/build-log/stream?token=${token}`)
      } catch {
        startPollingFallback()
        return
      }

      socket.onmessage = (event) => appendLine(String(event.data))
      socket.onerror = () => {
        if (alive && !gotData) {
          startPollingFallback()
        }
      }
    }

    openStream()

    return () => {
      alive = false
      if (pollTimer !== null) {
        clearInterval(pollTimer)
      }
      try {
        socket?.close()
      } catch {
        // уже закрыт
      }
    }
  }, [isBuilding, recordId])

  useEffect(() => {
    const logBox = buildLogBoxRef.current
    if (logBox) {
      logBox.scrollTop = logBox.scrollHeight
    }
  }, [buildLog])

  const handleCopyLogs = () => {
    const allLogs = (logs || []).join('\n')
    copyText(allLogs)
  }

  const handleDownloadLogs = () => {
    if (!record) return
    const logText = (logs || []).join('\n')
    downloadTextFile(`${record.id}.log`, logText)
  }

  const handleAddVolume = () => {
    const volumeName = newVolName.trim()
    const mountPath = newVolPath.trim()
    if (!volumeName || !mountPath) return

    const newVolume = {
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

  const handleOp = async (action: 'start' | 'stop' | 'restart' | 'remove') => {
    if (!record || busy) return

    setBusy(true)
    setError(null)

    try {
      if (action === 'start' || action === 'restart') {
        await startContainer(record.id)
      } else if (action === 'stop') {
        await stopContainer(record.id)
      } else if (action === 'remove') {
        await removeContainer(record.id)
        navigate('/applications')
        return
      }

      // Poll for status update
      const startTime = Date.now()
      const targetStatus = action === 'stop' ? 'stopped' : 'running'
      while (Date.now() - startTime < 90000) {
        if (cancelWaitRef.current) return
        await new Promise((resolve) => setTimeout(resolve, 2000))
        try {
          const res = await fetchMine()
          const found = res.data.find((c) => c.id === record.id)
          if (found) {
            setRecord(found)
            if (found.status === targetStatus || found.status === 'error') break
          }
        } catch {
          // continue polling
        }
      }
    } catch (e: any) {
      setError(e.message || 'Операция не удалась')
    } finally {
      setBusy(false)
    }
  }

  if (!record) {
    return (
      <Box className={styles.page}>
        <Button startIcon={<ArrowLeft size={16} />} onClick={() => navigate(-1)}>Назад</Button>
        <Card><CardContent>{error || `Контейнер не найден: ${realId}`}</CardContent></Card>
      </Box>
    )
  }

  const selectedEnv = configs.find((config) => config.id === envConfigId)
  const dockerCmd = useMemo(() => {
    if (!spec) return ''
    return buildDockerCommand(spec, selectedEnv)
  }, [spec, selectedEnv])

  const copyDocker = () => {
    copyText(dockerCmd)
  }

  const copyFilteredLogs = () => {
    copyText(filterLogsBySearch(logs || [], search).join('\n'))
  }

  const statusColor = getStatusColor(record.status)
  const followLabel = follow ? 'Слежение: вкл' : 'Слежение: выкл'
  const followColor = follow ? 'success' : 'default'
  const canAddVolume = !newVolName.trim() || !newVolPath.trim()
  const appUrl = getAppUrl(record.node_id, spec?.ports || [])

  return (
    <Box className={styles.page}>
      <Button startIcon={<ArrowLeft size={16} />} onClick={() => navigate(-1)} className={styles.back}>Назад</Button>

      <Box className={styles.header}>
        <Box>
          <Typography variant="h4" className={styles.header__title}>
            {record.app_name}
            <Chip size="small" color={statusColor} label={record.status} />
            <Chip size="small" variant="outlined" label={record.image} />
          </Typography>
          <Box className={styles.header__meta}>
            <Typography variant="body2">{record.node_id} • {record.id}</Typography>
            <Typography variant="caption">{new Date(record.updated_at).toLocaleString()}</Typography>
          </Box>
        </Box>
        <Stack direction="row" spacing={1}>
          <Button variant="outlined" startIcon={<RotateCcw size={16} />} disabled={busy} onClick={() => handleOp('restart')}>
            {busy ? 'Перезапуск...' : 'Перезапустить'}
          </Button>
          <Button variant="contained" color="success" startIcon={<Play size={16} />} disabled={busy || record.status === 'running'} onClick={() => handleOp('start')}>
            {busy ? 'Запуск...' : 'Старт'}
          </Button>
          <Button variant="contained" color="error" startIcon={<Square size={16} />} disabled={busy || record.status === 'stopped'} onClick={() => handleOp('stop')}>
            {busy ? 'Остановка...' : 'Стоп'}
          </Button>
          <Button variant="outlined" color="error" startIcon={<Trash2 size={16} />} disabled={busy} onClick={() => handleOp('remove')}>
            {busy ? 'Удаление...' : 'Удалить'}
          </Button>
        </Stack>
      </Box>

      {error && <Alert severity="error">{error}</Alert>}

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
              <Box className={styles.dockerPreview}>{dockerCmd || 'Загрузка...'}</Box>
              <Button size="small" sx={{ mt: 1 }} startIcon={<Copy size={14} />} onClick={copyDocker} disabled={!dockerCmd}>Копировать команду</Button>
            </CardContent></Card>
            <Card><CardContent>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>Порты</Typography>
              <Box className={styles.portList}>
                {spec?.ports.length === 0 ? (
                  <Typography variant="body2" color="text.secondary">Нет проброшенных портов</Typography>
                ) : (
                  <>
                    {spec?.ports.map((port) => (
                      <Box key={port.host} sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                        <Chip
                          icon={<Globe size={14} />}
                          label={`${port.host} → ${port.container}`}
                          size="small"
                          color="primary"
                          variant="outlined"
                        />
                        {appUrl && port.host === spec.ports[0].host && (
                          <a href={appUrl} target="_blank" rel="noopener noreferrer" sx={{ textDecoration: 'none' }}>
                            <Button size="small" startIcon={<ExternalLink size={14} />} variant="outlined">Открыть приложение</Button>
                          </a>
                        )}
                      </Box>
                    ))}
                  </>
                )}
              </Box>
            </CardContent></Card>
          </Stack>
          <Stack spacing={2}>
            <Card><CardContent>
              <Stack className={styles.stat} direction="row"><Box className={`${styles.stat__icon} ${styles['stat__icon--primary']}`}><BoxIcon size={18} /></Box><Box><Typography variant="body2" color="text.secondary">Образ</Typography><Typography sx={{ fontWeight: 600 }}>{spec?.image || record.image}</Typography></Box></Stack>
              <Divider sx={{ my: 1.5 }} />
              <Box className={styles.kv}>
                <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Рестарт</span><span className={styles.kv__rowValue}>unless-stopped</span></Box>
                <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Узел</span><span className={styles.kv__rowValue}>{record.node_id}</span></Box>
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
              <IconButton size="small" onClick={handleDownloadLogs} sx={{ color: '#e2e8f0' }}><Download size={16} /></IconButton>
              <IconButton size="small" onClick={handleCopyLogs} sx={{ color: '#e2e8f0' }}><Copy size={14} /></IconButton>
            </Box>
            <Box className={styles.logs__body}>
              {isBuilding && (
                <Box ref={buildLogBoxRef} sx={{ bgcolor: '#0b1220', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, p: 2, borderRadius: 2, maxHeight: 380, overflowY: 'auto', whiteSpace: 'pre-wrap' }}>
                  {buildLog === null ? (buildLogError || 'Подключение к логу сборки...') : buildLog === '' ? 'Пока пусто — первые шаги появятся через пару секунд...' : buildLog}
                </Box>
              )}
              {!isBuilding && logsLoading && <Typography color="text.secondary">Загрузка логов...</Typography>}
              {!isBuilding && !logsLoading && logs !== null && (
                <Box sx={{ bgcolor: '#0b1220', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, p: 2, borderRadius: 2, maxHeight: 380, overflowY: 'auto', whiteSpace: 'pre-wrap' }}>
                  {logs.length === 0 ? '— нет логов —' : logs.map((line, lineIndex) => <div key={lineIndex} className={styles.logs__line}>{line}</div>)}
                </Box>
              )}
              {!isBuilding && !logsLoading && logs === null && (
                <Typography color="text.secondary">Контейнер не найден на сервере</Typography>
              )}
              {follow && !isBuilding && <Typography variant="caption" sx={{ opacity: .5, display: 'block', mt: 1 }}>— режим слежения, новые строки появятся автоматически —</Typography>}
            </Box>
          </Box>
          <CardContent>
            <Typography variant="caption" color="text.secondary">Логи получаются через `docker logs {record.name}`. В реальном бэкенде — стриминг по WebSocket.</Typography>
          </CardContent>
        </Card>
      )}

      {tab === 2 && (
        <Stack spacing={2}>
          <Card><CardContent>
            <Typography variant="h6" sx={{ fontWeight: 650, mb: 2 }}>Параметры запуска</Typography>
            <Box className={styles.kv}>
              <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Образ</span><span className={styles.kv__rowValue}>{spec?.image || record.image}</span></Box>
              <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Имя контейнера</span><span className={styles.kv__rowValue}>{spec?.name || record.app_name}</span></Box>
              <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Политика рестарта</span><span className={styles.kv__rowValue}>unless-stopped</span></Box>
              <Box className={styles.kv__row}><span className={styles.kv__rowLabel}>Обновлено</span><span className={styles.kv__rowValue}>{new Date(record.updated_at).toLocaleString()}</span></Box>
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
                {spec && Object.keys(spec.env).length > 0 ? (
                  Object.entries(spec.env).map(([k, v]) => (
                    <Box key={k} className={styles.kv__row}>
                      <span className={styles.kv__rowLabel}>{k}</span>
                      <span className={styles.kv__rowValue}>{v}</span>
                    </Box>
                  ))
                ) : (
                  <Typography variant="body2" color="text.secondary">Нет env</Typography>
                )}
              </Box>
            )}
            <Box className={styles.dockerPreview} sx={{ mt: 2 }}>{dockerCmd}</Box>
            <Button size="small" startIcon={<Copy size={14} />} sx={{ mt: 1 }} onClick={copyDocker} disabled={!dockerCmd}>Копировать docker run</Button>
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
            {volumes.map((volume) => `-v ${volume.name}:${volume.mountPath}`).join(' ') || '# нет томов'}
          </Box>
        </CardContent></Card>
      )}
    </Box>
  )
}