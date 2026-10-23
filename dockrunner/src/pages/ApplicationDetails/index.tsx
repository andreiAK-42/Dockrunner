import { Box, Button, Card, CardContent, Chip, Stack, Typography, Divider, Tab, Tabs, FormControl, InputLabel, Select, MenuItem, Alert, Tooltip } from '@mui/material'
import { ArrowLeft, Play, Square, RefreshCw, Terminal, HardDrive as HD, Copy, Download, Box as BoxIcon, Server, Activity, Trash2, ExternalLink } from 'lucide-react'
import { useParams, useNavigate } from 'react-router-dom'
import { useState, useEffect, useRef } from 'react'
import { getAppById } from '../../config/applications'
import { copyText } from '../../utils/clipboard'
import { refreshContainerByApp, removeContainer, startContainer, stopContainer, fetchMine, fetchBuildLog, fetchContainerLogs, fetchContainerSpec } from '../../api/containers'
import type { MineContainer, ContainerSpec } from '../../api/containers'
import { ApiError, apiBase, getToken } from '../../api/client'
import { timeAgo } from '../../utils/time'
import { useEnvConfigs } from '../../hooks/useEnvConfigs'
import { toDockerEnvArgs } from '../../types/envConfig'
import PortQuota from '../../components/PortQuota'
import styles from './ApplicationDetails.module.scss'

import { statusLabel } from '../../utils/status'

const POLL_INTERVAL_MS = 2000
const DELETE_TIMEOUT_MS = 120000
const OPERATION_TIMEOUT_MS = 90000
const REFRESH_TIMEOUT_MS = 600000
const BUILD_LOG_POLL_MS = 2000
const MESSAGE_HIDE_MS = 8000
const CONFIRM_RESET_MS = 5000

function specEnvArgs(spec: ContainerSpec | null) {
  if (!spec) {
    return ''
  }

  const args = Object.entries(spec.env).map(([envKey, envValue]) => `--env ${envKey}="${envValue}"`)
  return args.join(' ')
}

function formatVolumeMount(volume: { name: string; host_path: string; container_path: string }) {
  const source = volume.name || volume.host_path
  return `-v ${source}:${volume.container_path}`
}

function buildDockerRun(spec: ContainerSpec | null, envArgs: string) {
  if (!spec) {
    return '—'
  }

  const portArgs = spec.ports.map((port) => `-p ${port.host}:${port.container}`).join(' ')
  const volumeArgs = spec.volumes.map(formatVolumeMount).join(' ')
  const fullCommand = `docker run -d --name ${spec.name} ${portArgs} ${volumeArgs} ${envArgs} ${spec.image}`

  return fullCommand.replace(/\s+/g, ' ').trim()
}

function getErrorText(error: unknown, fallback: string): string {
  if (error instanceof Error) {
    return error.message
  }
  return fallback
}

export default function ApplicationDetails() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [tab, setTab] = useState(0)
  const app = getAppById(id || '')
  const { configs } = useEnvConfigs()
  const [envConfigId, setEnvConfigId] = useState('')
  const selectedEnv = configs.find((config) => config.id === envConfigId)
  const [updating, setUpdating] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [acting, setActing] = useState<'' | 'start' | 'stop' | 'restart'>('')
  const [updateMsg, setUpdateMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const [record, setRecord] = useState<MineContainer | null>(null)
  const [buildLog, setBuildLog] = useState<string | null>(null)
  const [buildLogError, setBuildLogError] = useState('')
  const buildLogBoxRef = useRef<HTMLDivElement | null>(null)
  const [logs, setLogs] = useState<string[] | null>(null)
  const [logsLoadedFor, setLogsLoadedFor] = useState('')
  const [spec, setSpec] = useState<ContainerSpec | null>(null)
  const cancelWaitRef = useRef(false)

  const busy = updating || deleting || acting !== ''

  let envArgs = specEnvArgs(spec)
  if (selectedEnv) {
    envArgs = toDockerEnvArgs(selectedEnv)
  }

  useEffect(() => {
    cancelWaitRef.current = false
    return () => {
      cancelWaitRef.current = true
    }
  }, [])

  useEffect(() => {
    fetchMine()
      .then((res) => {
        const foundRecord = res.data.find((container) => container.app_name === id) || null
        setRecord(foundRecord)
      })
      .catch(() => setRecord(null))
  }, [id])

  // Poll for record updates while building or during operations
  useEffect(() => {
    if (!record || (record.status !== 'building' && !busy)) {
      return
    }
    let alive = true
    const pollRecord = async () => {
      try {
        const res = await fetchMine()
        const found = res.data.find((c) => c.id === record.id) || null
        if (alive && found) {
          setRecord(found)
        }
      } catch {
        // ignore
      }
    }
    const timer = setInterval(pollRecord, POLL_INTERVAL_MS)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [record?.id, record?.status, busy])

  useEffect(() => {
    if (!record) {
      return
    }
    const recordId = record.id
    fetchContainerSpec(recordId)
      .then((res) => setSpec(res.data))
      .catch(() => setSpec(null))
  }, [record])

  let wantLogs = ''
  if (tab === 1 && record) {
    wantLogs = record.id
  }

  const isStaleLogs = logsLoadedFor !== wantLogs
  const logsLoading = wantLogs !== '' && (logs === null || isStaleLogs)

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
      if (!alive) {
        return
      }
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
          if (!alive) {
            return
          }
          setBuildLog(res.data)
          setBuildLogError('')
        })
        .catch((loadError) => {
          const isMissing = alive && loadError instanceof ApiError && loadError.status === 404
          if (isMissing) {
            setBuildLogError(loadError.message)
          }
        })
    }

    const startPollingFallback = () => {
      if (pollTimer !== null) {
        return
      }
      loadBuildLogOnce()
      pollTimer = setInterval(loadBuildLogOnce, BUILD_LOG_POLL_MS)
    }

    const openStream = () => {
      try {
        const httpBase = apiBase()
        const wsBase = httpBase.replace(/^http/, 'ws')
        const token = encodeURIComponent(getToken() || '')
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

  useEffect(() => {
    if (!wantLogs) {
      return
    }

    const loadingFor = wantLogs
    let alive = true
    fetchContainerLogs(loadingFor)
      .then((res) => {
        if (!alive) {
          return
        }
        setLogs(res.data.split('\n'))
        setLogsLoadedFor(loadingFor)
      })
      .catch(() => {
        if (!alive) {
          return
        }
        setLogs([])
        setLogsLoadedFor(loadingFor)
      })

    return () => {
      alive = false
    }
  }, [wantLogs])

  const handleCopyLogs = () => {
    const allLogs = (logs || []).join('\n')
    copyText(allLogs)
  }

  const waitForRecord = async (waitedId: string, done: (found: MineContainer | null) => boolean, timeoutMs: number, timeoutMsg: string): Promise<MineContainer | null> => {
    const started = Date.now()

    for (;;) {
      if (cancelWaitRef.current) {
        return null
      }

      await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS))

      if (cancelWaitRef.current) {
        return null
      }

      try {
        const res = await fetchMine()
        const found = res.data.find((container) => container.id === waitedId) || null
        setRecord(found)
        if (done(found)) {
          return found
        }
      } catch {
        // опрос продолжается — сервер мог перезапускаться
      }

      const timedOut = Date.now() - started > timeoutMs
      if (timedOut) {
        throw new Error(timeoutMsg)
      }
    }
  }

  const showMessage = (ok: boolean, text: string) => {
    setUpdateMsg({ ok, text })
    setTimeout(() => setUpdateMsg(null), MESSAGE_HIDE_MS)
  }

  const handleDelete = async () => {
    if (!record || busy) {
      return
    }

    if (!confirmDelete) {
      setConfirmDelete(true)
      setTimeout(() => setConfirmDelete(false), CONFIRM_RESET_MS)
      return
    }

    setConfirmDelete(false)
    cancelWaitRef.current = false
    setDeleting(true)
    setUpdateMsg(null)

    const deletedId = record.id

    try {
      await removeContainer(deletedId)
      await waitForRecord(deletedId, (found) => found === null, DELETE_TIMEOUT_MS, 'Удаление затянулось — проверьте список позже')
      navigate('/applications')
    } catch (deleteError) {
      setDeleting(false)
      const isConflict = deleteError instanceof ApiError && deleteError.status === 409
      if (isConflict) {
        showMessage(false, deleteError.message)
      } else {
        showMessage(false, getErrorText(deleteError, 'Не удалось удалить'))
      }
    }
  }

  const handleRefresh = async () => {
    if (!app || !record || updating) {
      return
    }

    cancelWaitRef.current = false
    setUpdating(true)
    setUpdateMsg(null)

    const refreshedId = record.id

    try {
      await refreshContainerByApp(app.name)
      const refreshed = await waitForRecord(
        refreshedId,
        (found) => found !== null && found.status !== 'building',
        REFRESH_TIMEOUT_MS,
        'Превышено время ожидания обновления',
      )

      if (!refreshed) {
        showMessage(false, 'Запись приложения пропала')
      } else if (refreshed.status === 'error') {
        showMessage(false, 'Обновление не удалось — проверьте статус и логи')
      } else {
        showMessage(true, 'Приложение обновлено и запущено')
      }
    } catch (refreshError) {
      const isConflict = refreshError instanceof ApiError && refreshError.status === 409
      if (isConflict) {
        showMessage(false, refreshError.message)
      } else {
        showMessage(false, getErrorText(refreshError, 'Не удалось запустить обновление'))
      }
    } finally {
      setUpdating(false)
      setTimeout(() => setUpdateMsg(null), MESSAGE_HIDE_MS)
    }
  }

  const handleOp = async (action: 'start' | 'stop' | 'restart') => {
    if (!record || busy) {
      return
    }

    cancelWaitRef.current = false
    setActing(action)
    setUpdateMsg(null)

    const operatedId = record.id

    let wantStatus = 'running'
    if (action === 'stop') {
      wantStatus = 'stopped'
    }

    let okText = 'Приложение перезапущено'
    if (action === 'stop') {
      okText = 'Приложение остановлено'
    } else if (action === 'start') {
      okText = 'Приложение запущено'
    }

    try {
      if (action === 'stop') {
        await stopContainer(operatedId)
      } else {
        await startContainer(operatedId)
      }

      const finished = await waitForRecord(
        operatedId,
        (found) => found !== null && found.status === wantStatus,
        OPERATION_TIMEOUT_MS,
        'Операция затянулась — проверьте статус позже',
      )

      if (finished && finished.status === 'error') {
        showMessage(false, 'Операция не удалась — проверьте статус и логи')
      } else {
        showMessage(true, okText)
      }
    } catch (opError) {
      const isKnownStatus = opError instanceof ApiError && (opError.status === 409 || opError.status === 404)
      if (isKnownStatus) {
        showMessage(false, opError.message)
      } else {
        showMessage(false, getErrorText(opError, 'Не удалось выполнить операцию'))
      }
    } finally {
      setActing('')
    }
  }

  return (
    <div className={styles.page}>
      <Button startIcon={<ArrowLeft size={16} />} onClick={() => navigate('/applications')} className={styles.backButton}>Назад к приложениям</Button>

      <Box className={styles.header}>
        <Box>
          <Typography variant="h4" className={styles.header__title}>{app.name} <Chip size="small" color={record?.status === 'running' ? 'success' : record?.status === 'error' ? 'error' : 'default'} label={record ? statusLabel(record.status) : app.status} /></Typography>
          <Stack direction="row" spacing={1} alignItems="center" className={styles.header__meta}><Typography variant="body2" color="text.secondary">{spec?.image || record?.image || '—'} • {record?.node_id || '—'}</Typography></Stack>
        </Box>
        <Stack direction="row" spacing={1} className={styles.header__actions}><Button variant="outlined" startIcon={<RefreshCw size={16} />} disabled={!record || busy} onClick={() => handleOp('restart')}>{acting === 'restart' ? 'Перезапуск...' : 'Перезапустить'}</Button><Button variant="outlined" startIcon={<Download size={16} />} disabled={!record || busy} onClick={handleRefresh}>{updating ? 'Обновление...' : 'Обновить'}</Button><Button variant="contained" color="success" startIcon={<Play size={16} />} sx={{ color: '#fff' }} disabled={!record || busy} onClick={() => handleOp('start')}>{acting === 'start' ? 'Запуск...' : 'Старт'}</Button><Button variant="contained" color="error" startIcon={<Square size={16} />} sx={{ color: '#fff' }} disabled={!record || busy} onClick={() => handleOp('stop')}>{acting === 'stop' ? 'Остановка...' : 'Стоп'}</Button><Button variant="outlined" color="error" startIcon={<Trash2 size={16} />} disabled={!record || busy} onClick={handleDelete}>{deleting ? 'Удаление...' : confirmDelete ? 'Точно удалить?' : 'Удалить'}</Button></Stack>
      </Box>

      {updateMsg && <Alert severity={updateMsg.ok ? 'success' : 'error'}>{updateMsg.text}</Alert>}

      <Card><Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ px: 2 }}><Tab label="Обзор" /><Tab label="Логи" /><Tab label="Настройки" /></Tabs><Divider /></Card>

      {tab === 0 && (
        <Stack spacing={3}>
          {isBuilding && (
            <Card><CardContent>
              <Typography variant="h6" sx={{ fontWeight: 650, mb: 2, display: 'flex', alignItems: 'center', gap: 1 }}><Terminal size={18} /> Сборка образа — живой лог</Typography>
              <Box ref={buildLogBoxRef} sx={{ bgcolor: '#0b1220', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, p: 2, borderRadius: 2, maxHeight: 380, overflowY: 'auto', whiteSpace: 'pre-wrap' }}>
                {buildLog === null ? (buildLogError || 'Подключение к логу сборки...') : buildLog === '' ? 'Пока пусто — первые шаги появятся через пару секунд...' : buildLog}
              </Box>
            </CardContent></Card>
          )}
          <Box className={styles.grid3}>
            <Card><CardContent><Stack direction="row" gap={2} alignItems="center"><Box className={`${styles.statIcon} ${styles['statIcon--primary']}`} sx={{ alignSelf: 'center' }}><BoxIcon size={18} /></Box><Box sx={{ minWidth: 0, flex: 1 }}><Typography variant="body2" color="text.secondary">Образ</Typography><Tooltip title={spec?.image || record?.image || ''}><Typography variant="body1" noWrap sx={{ fontWeight: 600 }}>{spec?.image || record?.image || '—'}</Typography></Tooltip></Box></Stack></CardContent></Card>
            <Card><CardContent><Stack direction="row" gap={2} alignItems="center"><Box className={`${styles.statIcon} ${styles['statIcon--success']}`} sx={{ alignSelf: 'center' }}><Server size={18} /></Box><Box><Typography variant="body2" color="text.secondary">Узел</Typography><Typography variant="body1" sx={{ fontWeight: 600 }}>{record?.node_id || '—'}</Typography></Box></Stack></CardContent></Card>
            <Card><CardContent><Stack direction="row" gap={2} alignItems="center"><Box className={`${styles.statIcon} ${styles['statIcon--info']}`} sx={{ alignSelf: 'center' }}><Activity size={18} /></Box><Box><Typography variant="body2" color="text.secondary">Статус</Typography><Typography variant="body1" sx={{ fontWeight: 600 }}>{record ? statusLabel(record.status) : '—'}{record ? ` · ${timeAgo(record.updated_at)}` : ''}</Typography></Box></Stack></CardContent></Card>
          </Box>

          <Card><CardContent>
            <Typography variant="h6" sx={{ fontWeight: 650, mb: 2 }}>Порты</Typography>
            {!spec || spec.ports.length === 0
              ? <Typography color="text.secondary">Нет проброшенных портов</Typography>
              : <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                  {spec.ports.map((p) => (
                    <Box key={`${p.host}-${p.container}`} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Chip size="small" label={`${p.host} → ${p.container}`} color="primary" variant="outlined" />
                      {p.host === spec.ports[0].host && (
                        <a href={`http://${record?.node_id}:${p.host}`} target="_blank" rel="noopener noreferrer">
                          <Button size="small" startIcon={<ExternalLink size={14} />} variant="outlined">Открыть</Button>
                        </a>
                      )}
                    </Box>
                  ))}
                </Stack>}
          </CardContent></Card>

          <Card><CardContent>
            <Typography variant="h6" sx={{ fontWeight: 650, mb: 2 }}>Тома</Typography>
            {!spec || spec.volumes.length === 0
              ? <Typography color="text.secondary">Нет томов у этого приложения</Typography>
              : <Stack spacing={1}>{spec.volumes.map((v, i) => <Box key={i} sx={{ display: 'flex', justifyContent: 'space-between', py: 0.7, borderBottom: '1px solid', borderColor: 'divider' }}><Typography variant="body2" sx={{ fontWeight: 500 }}>{v.name || v.host_path}</Typography><Typography variant="caption">{v.container_path}</Typography></Box>)}</Stack>}
          </CardContent></Card>

          <PortQuota compact />
        </Stack>
      )}
      {tab === 1 && (
        <Card>
          <CardContent>
            <Typography variant="h6" sx={{ fontWeight: 650, mb: 2, display: 'flex', alignItems: 'center', gap: 1 }}><Terminal size={18} /> Логи приложения</Typography>
            {logsLoading && <Typography color="text.secondary">Загрузка логов...</Typography>}
            {!logsLoading && logs !== null && (
              <Box sx={{ bgcolor: '#0b1220', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, p: 2, borderRadius: 2, maxHeight: 380, overflowY: 'auto', whiteSpace: 'pre-wrap' }}>
                {logs.length === 0 && '— нет логов —'}
                {logs.map((line, i) => <div key={i}>{line}</div>)}
              </Box>
            )}
            {!logsLoading && logs === null && (
              <Typography color="text.secondary">Контейнер не найден на сервере</Typography>
            )}
            <Button size="small" sx={{ mt: 1 }} startIcon={<Copy size={14} />} onClick={handleCopyLogs}>Копировать логи</Button>
          </CardContent>
        </Card>
      )}
      {tab === 2 && (
        <Stack spacing={2}>
          <Card><CardContent>
            <Typography variant="h6" sx={{ fontWeight: 650, mb: 1, display: 'flex', alignItems: 'center', gap: 1 }}><HD size={18} /> Параметры запуска</Typography>
            <Stack spacing={1.5}>
              <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', alignItems: 'center' }}>
                <Typography variant="body2" sx={{ fontWeight: 600 }}>{spec?.name || app.name}</Typography>
                <Chip label={spec?.image || record?.image || '—'} size="small" />
                <Chip label={spec?.status || record?.status || '—'} size="small" variant="outlined" />
              </Box>
              <Box sx={{ p: 1.5, borderRadius: 2, bgcolor: '#0f172a', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, whiteSpace: 'pre-wrap' }}>
                {buildDockerRun(spec, envArgs)}
              </Box>
              <Divider />
              <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>Env-конфигурация</Typography>
              <FormControl fullWidth size="small">
                <InputLabel>Выбрать сохранённый env-конфиг</InputLabel>
                <Select value={envConfigId} label="Выбрать сохранённый env-конфиг" onChange={e => setEnvConfigId(e.target.value)}>
                  <MenuItem value=""><em>— Переменные контейнера —</em></MenuItem>
                  {configs.map(c => <MenuItem key={c.id} value={c.id}>{c.name} ({c.vars.length} перем.)</MenuItem>)}
                </Select>
              </FormControl>
              {selectedEnv ? (
                <Box sx={{ p: 1.5, borderRadius: 2, bgcolor: '#0f172a', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, whiteSpace: 'pre-wrap' }}>
                  {toDockerEnvArgs(selectedEnv)}
                </Box>
              ) : (
                <Box>
                  {!spec || Object.keys(spec.env).length === 0 ? <Typography variant="body2" color="text.secondary">Нет env</Typography> :
                    Object.entries(spec.env).map(([k, v]) => <Box key={k} sx={{ display: 'flex', justifyContent: 'space-between', py: 0.7, borderBottom: '1px solid', borderColor: 'divider' }}><Typography variant="body2" sx={{ fontWeight: 500 }}>{k}</Typography><Typography variant="caption">{v}</Typography></Box>)}
                </Box>
              )}
              <PortQuota compact />
            </Stack>
          </CardContent></Card>
        </Stack>
      )}
    </div>
  )
}
