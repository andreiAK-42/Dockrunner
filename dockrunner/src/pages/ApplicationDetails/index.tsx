import { Box, Button, Card, CardContent, Chip, Stack, Typography, Divider, Tab, Tabs, FormControl, InputLabel, Select, MenuItem, Alert, Tooltip, TextField, Snackbar } from '@mui/material'
import { ArrowLeft, Play, Square, RefreshCw, Terminal, HardDrive as HD, Copy, Download, Box as BoxIcon, Server, Activity, Trash2, ExternalLink, AlertCircle } from 'lucide-react'
import { useParams, useNavigate } from 'react-router-dom'
import { useState, useEffect, useRef, useCallback } from 'react'
import { copyText } from '../../utils/clipboard'
import { refreshContainerByApp, removeContainer, startContainer, stopContainer, fetchMine, fetchBuildLog, fetchContainerSpec, applyDockerCommand } from '../../api/containers'
import type { MineContainer, ContainerSpec } from '../../api/containers'
import { ApiError, apiBase, getToken } from '../../api/client'
import { timeAgo } from '../../utils/time'
import { useEnvConfigs } from '../../hooks/useEnvConfigs'
import { useRealtimeStatus } from '../../hooks/useRealtime'
import { useContainerLogs } from '../../hooks/useContainerLogs'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { containerStatusPatched, fetchMineContainers } from '../../store/slices/containersSlice'
import { toDockerEnvArgs } from '../../types/envConfig'
import PortQuota from '../../components/PortQuota'
import styles from './ApplicationDetails.module.scss'

import { statusLabel, IN_PROGRESS_STATUSES } from '../../utils/status'

const POLL_INTERVAL_MS = 5000
const DELETE_TIMEOUT_MS = 120000
const OPERATION_TIMEOUT_MS = 90000
const REFRESH_TIMEOUT_MS = 600000
const BUILD_LOG_POLL_MS = 5000
const MESSAGE_HIDE_MS = 8000
const CONFIRM_RESET_MS = 5000
const MAX_BUILD_LOG_LINES = 2000
const MAX_LOG_LINES = 1000

function shellArg(value: string): string {
  if (/^[A-Za-z0-9_./:@%+=,-]+$/.test(value)) return value
  return `'${value.replace(/'/g, `"'"'`)}'`
}

function specEnvArgs(spec: ContainerSpec | null) {
  if (!spec) return ''
  return Object.entries(spec.env).map(([key, value]) => `--env ${shellArg(`${key}=${value}`)}`).join(' ')
}

function formatVolumeMount(volume: { name: string; host_path: string; container_path: string }) {
  const source = volume.name || volume.host_path
  return `-v ${shellArg(`${source}:${volume.container_path}`)}`
}

function buildDockerRun(spec: ContainerSpec | null, envArgs: string) {
  if (!spec) return ''
  const args = ['docker run -d', `--name ${shellArg(spec.name)}`]
  for (const port of spec.ports) args.push(`-p ${port.host}:${port.container}`)
  args.push(...spec.volumes.map(formatVolumeMount), envArgs)
  if (spec.restart_policy) args.push(`--restart ${shellArg(spec.restart_policy)}`)
  if (spec.network) args.push(`--network ${shellArg(spec.network)}`)
  if (spec.entrypoint?.length) args.push(`--entrypoint ${shellArg(spec.entrypoint[0])}`)
  if (spec.working_dir) args.push(`-w ${shellArg(spec.working_dir)}`)
  if (spec.user) args.push(`-u ${shellArg(spec.user)}`)
  args.push(shellArg(spec.image))
  if (spec.cmd?.length || (spec.entrypoint && spec.entrypoint.length > 1)) args.push([...(spec.entrypoint?.slice(1) || []), ...(spec.cmd || [])].map(shellArg).join(' '))
  return args.filter(Boolean).join(' ').trim()
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
  const dispatch = useAppDispatch()
  const [tab, setTab] = useState(0)
  const { configs } = useEnvConfigs()
  const [envConfigId, setEnvConfigId] = useState('')
  const [editedDockerCmd, setEditedDockerCmd] = useState<string | null>(null)
  const [applyingCommand, setApplyingCommand] = useState(false)
  const selectedEnv = configs.find((config) => config.id === envConfigId)
  const [updating, setUpdating] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [acting, setActing] = useState<'' | 'start' | 'stop' | 'restart'>('')
  const [updateMsg, setUpdateMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const record = useAppSelector(
    (state) => state.containers.mine.find((container) => container.app_name === id) || null,
  )
  const [buildLog, setBuildLog] = useState<string | null>(null)
  const [buildLogError, setBuildLogError] = useState('')
  const buildLogBoxRef = useRef<HTMLDivElement | null>(null)
  const [spec, setSpec] = useState<ContainerSpec | null>(null)
  const [followBuildLog, setFollowBuildLog] = useState(true)
  const cancelWaitRef = useRef(false)
  const [syncing, setSyncing] = useState(false)
  const [syncError, setSyncError] = useState<string | null>(null)
  const [syncSuccess, setSyncSuccess] = useState<string | null>(null)
  const connected = useRealtimeStatus()

  // Verify actual container status by fetching spec
  const verifyContainerStatus = useCallback(async () => {
    setSyncing(true)
    setSyncError(null)
    setSyncSuccess(null)

    try {
      // Fetch fresh record from backend to avoid stale closure
      const mineRes = await fetchMine()
      const currentRecord = mineRes.data.find((c) => c.app_name === id) || null

      if (!currentRecord) {
        setSyncError('Контейнер не найден')
        setSyncing(false)
        return
      }

      const specRes = await fetchContainerSpec(currentRecord.id)
      const actualStatus = specRes.data.status

      if (actualStatus !== currentRecord.status) {
        dispatch(containerStatusPatched({ id: currentRecord.id, status: actualStatus }))
        setSyncSuccess(`Статус исправлен: было "${currentRecord.status}", стало "${actualStatus}"`)
      } else {
        setSyncSuccess('Статус актуален, изменений нет')
      }
      // Also update spec
      setSpec(specRes.data)
    } catch (err: any) {
      // Handle 404 - backend DB out of sync with Docker on agent
      // Try to trigger a refresh from the agent
      if (err?.status === 404) {
        try {
          await refreshContainerByApp(id || '')
          setSyncSuccess('Запрошено обновление с агента, статус обновится через несколько секунд')
        } catch {
          setSyncError('Не удалось запросить обновление контейнера')
        }
      } else {
        setSyncError('Не удалось проверить статус контейнера')
      }
    } finally {
      setSyncing(false)
    }
  }, [id, dispatch])

  const busy = updating || deleting || applyingCommand || acting !== ''

  let envArgs = specEnvArgs(spec)
  if (selectedEnv) {
    envArgs = selectedEnv.vars.map((item) => `--env ${shellArg(`${item.key}=${item.value}`)}`).join(' ')
  }

  const generatedDockerCmd = buildDockerRun(spec, envArgs)
  const displayedDockerCmd = editedDockerCmd ?? generatedDockerCmd

  useEffect(() => {
    cancelWaitRef.current = false
    return () => {
      cancelWaitRef.current = true
    }
  }, [])

  useEffect(() => {
    void dispatch(fetchMineContainers())
  }, [id, dispatch])

  // Auto-verify status on initial load only (not on every record change to avoid loops)
  const verifiedRef = useRef(false)
  useEffect(() => {
    if (!verifiedRef.current && record) {
      verifiedRef.current = true
      void verifyContainerStatus()
    }
  }, [record, verifyContainerStatus])

  // Fallback status sync: only while the WebSocket is down.
  useEffect(() => {
    if (!record || connected) return

    const interval = setInterval(() => {
      verifyContainerStatus()
    }, 30000)

    return () => clearInterval(interval)
  }, [record, connected, verifyContainerStatus])

  // Fallback record polling while building: with WS events this is rarely used.
  useEffect(() => {
    if (connected || !record || (!IN_PROGRESS_STATUSES.has(record.status) && !busy)) {
      return
    }
    let alive = true
    const pollRecord = async () => {
      if (!alive) return
      try {
        await dispatch(fetchMineContainers()).unwrap()
      } catch {
        // ignore
      }
    }
    const timer = setInterval(pollRecord, POLL_INTERVAL_MS)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [record?.id, record?.status, busy, connected, dispatch])

  useEffect(() => {
    if (!record) {
      return
    }
    const recordId = record.id
    fetchContainerSpec(recordId)
      .then((res) => setSpec(res.data))
      .catch(() => setSpec(null))
  }, [record])

  const logsEnabled = tab === 1 && !!record
  const { lines: logs, error: logsError, live: logsLive } = useContainerLogs(record?.id || '', logsEnabled, MAX_LOG_LINES)
  const logsLoading = logsEnabled && logs === null && !logsError

  const isBuilding = record ? IN_PROGRESS_STATUSES.has(record.status) : false
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
        const lines = prev.split('\n')
        if (lines.length >= MAX_BUILD_LOG_LINES) {
          // Keep only the last MAX_BUILD_LOG_LINES - 1 lines plus the new one
          return lines.slice(-(MAX_BUILD_LOG_LINES - 1)).join('\n') + '\n' + line
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
    if (!followBuildLog) return
    const logBox = buildLogBoxRef.current
    if (logBox) {
      logBox.scrollTop = logBox.scrollHeight
    }
  }, [buildLog, followBuildLog])

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
        const containers = await dispatch(fetchMineContainers()).unwrap()
        const found = containers.find((container) => container.id === waitedId) || null
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
    if (!id || !record || updating) {
      return
    }

    cancelWaitRef.current = false
    setUpdating(true)
    setUpdateMsg(null)

    const refreshedId = record.id

    try {
      await refreshContainerByApp(record?.app_name || '')
      const refreshed = await waitForRecord(
        refreshedId,
        (found) => found !== null && !IN_PROGRESS_STATUSES.has(found.status),
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

  const applyEditedCommand = async () => {
    if (!record?.id || !displayedDockerCmd.trim()) return
    const confirmed = window.confirm('Контейнер будет остановлен и пересоздан с новыми параметрами. Данные в подключённых томах сохранятся, но изменения внутри файловой системы контейнера могут быть потеряны. Продолжить?')
    if (!confirmed) return
    setApplyingCommand(true)
    showMessage(false, '')
    try {
      await applyDockerCommand(record.id, displayedDockerCmd)
      setEditedDockerCmd(null)
      const refreshed = await fetchContainerSpec(record.id)
      setSpec(refreshed.data)
      showMessage(true, 'Параметры применены: контейнер пересоздан.')
      await dispatch(fetchMineContainers()).unwrap()
    } catch (e) {
      showMessage(false, e instanceof Error ? e.message : 'Не удалось применить параметры')
    } finally {
      setApplyingCommand(false)
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
          <Typography variant="h4" className={styles.header__title}>{record?.app_name || id} <Chip size="small" color={record?.status === 'running' ? 'success' : record?.status === 'error' ? 'error' : 'default'} label={record ? statusLabel(record.status) : '—'} /></Typography>
          <Stack direction="row" spacing={1} alignItems="center" className={styles.header__meta}><Typography variant="body2" color="text.secondary">{spec?.image || record?.image || '—'} • {record?.node_id || '—'}</Typography></Stack>
        </Box>
        <Stack direction="row" spacing={1} className={styles.header__actions}>
          <Button
            variant="outlined"
            startIcon={<RefreshCw size={16} />}
            onClick={verifyContainerStatus}
            disabled={!record || busy || syncing}
          >
            {syncing ? 'Проверка...' : 'Проверить статус'}
          </Button>
          <Button variant="outlined" startIcon={<RefreshCw size={16} />} disabled={!record || busy} onClick={() => handleOp('restart')}>{acting === 'restart' ? 'Перезапуск...' : 'Перезапустить'}</Button>
          <Button variant="outlined" startIcon={<Download size={16} />} disabled={!record || busy} onClick={handleRefresh}>{updating ? 'Обновление...' : 'Обновить'}</Button>
          <Button variant="contained" color="success" startIcon={<Play size={16} />} sx={{ color: '#fff' }} disabled={!record || busy} onClick={() => handleOp('start')}>{acting === 'start' ? 'Запуск...' : 'Старт'}</Button>
          <Button variant="contained" color="error" startIcon={<Square size={16} />} sx={{ color: '#fff' }} disabled={!record || busy} onClick={() => handleOp('stop')}>{acting === 'stop' ? 'Остановка...' : 'Стоп'}</Button>
          <Button variant="outlined" color="error" startIcon={<Trash2 size={16} />} disabled={!record || busy} onClick={handleDelete}>{deleting ? 'Удаление...' : confirmDelete ? 'Точно удалить?' : 'Удалить'}</Button>
        </Stack>
      </Box>

      {updateMsg && <Alert severity={updateMsg.ok ? 'success' : 'error'}>{updateMsg.text}</Alert>}
      <Snackbar open={!!syncError} autoHideDuration={6000} onClose={() => setSyncError(null)}>
        <Alert severity="error" onClose={() => setSyncError(null)} sx={{ flexGrow: 1 }}>
          <AlertCircle size={18} sx={{ mr: 1 }} />
          {syncError}
        </Alert>
      </Snackbar>
      <Snackbar open={!!syncSuccess} autoHideDuration={6000} onClose={() => setSyncSuccess(null)}>
        <Alert severity="success" onClose={() => setSyncSuccess(null)} sx={{ flexGrow: 1 }}>
          {syncSuccess}
        </Alert>
      </Snackbar>

      <Card><Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ px: 2 }}><Tab label="Обзор" /><Tab label="Логи" /><Tab label="Настройки" /></Tabs><Divider /></Card>

      {tab === 0 && (
        <Stack spacing={3}>
          {isBuilding && (
            <Card><CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6" sx={{ fontWeight: 650, display: 'flex', alignItems: 'center', gap: 1 }}><Terminal size={18} /> Сборка образа — живой лог</Typography>
                <label sx={{ display: 'flex', alignItems: 'center', gap: 0.5, cursor: 'pointer' }}>
                  <input
                    type="checkbox"
                    checked={followBuildLog}
                    onChange={(e) => setFollowBuildLog(e.target.checked)}
                  />
                  <Typography variant="caption">Автоскролл</Typography>
                </label>
              </Box>
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
                        <a href={`http://${record?.node_ip || record?.node_id}:${p.host}`} target="_blank" rel="noopener noreferrer">
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
            <Typography variant="h6" sx={{ fontWeight: 650, mb: 2, display: 'flex', alignItems: 'center', gap: 1 }}>
              <Terminal size={18} /> Логи приложения
              {logsLive && <Chip label="live" size="small" color="success" variant="outlined" />}
            </Typography>
            {logsLoading && <Typography color="text.secondary">Загрузка логов...</Typography>}
            {logsError && !logsLoading && <Alert severity="warning" sx={{ mb: 1 }}>{logsError}</Alert>}
            {!logsLoading && logs !== null && (
              <Box sx={{ bgcolor: '#0b1220', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, p: 2, borderRadius: 2, maxHeight: 380, overflowY: 'auto', whiteSpace: 'pre-wrap' }}>
                {logs.length === 0 && '— нет логов —'}
                {logs.map((line, i) => <div key={i}>{line}</div>)}
              </Box>
            )}
            {!logsLoading && logs === null && !logsError && (
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
                <Typography variant="body2" sx={{ fontWeight: 600 }}>{spec?.name || record?.app_name || id}</Typography>
                <Chip label={spec?.image || record?.image || '—'} size="small" />
                <Chip label={spec?.status || record?.status || '—'} size="small" variant="outlined" />
              </Box>
              <TextField fullWidth multiline minRows={3} maxRows={10} label="Команда запуска (можно редактировать)" value={displayedDockerCmd} onChange={(e) => setEditedDockerCmd(e.target.value)} placeholder={spec ? 'Команда не сформирована: проверьте образ и имя контейнера' : 'Не удалось загрузить спецификацию контейнера'} sx={{ '& textarea': { fontFamily: 'monospace', fontSize: 12 } }} />
              {!displayedDockerCmd && <Alert severity="warning">Команда не сформирована. Проверьте, что API агента возвращает образ, имя и параметры контейнера.</Alert>}
              <Alert severity="info">Можно изменить строку и скопировать её. Это не изменяет уже созданный контейнер: чтобы применить параметры, его необходимо пересоздать.</Alert>
              <Stack direction="row" spacing={1}>
                <Button size="small" startIcon={<Copy size={14} />} onClick={() => copyText(displayedDockerCmd)} disabled={!displayedDockerCmd.trim()}>Копировать команду</Button>
                <Button size="small" onClick={() => setEditedDockerCmd(null)} disabled={editedDockerCmd === null}>Сбросить изменения</Button>
                  <Button size="small" variant="contained" onClick={applyEditedCommand} disabled={applyingCommand || !displayedDockerCmd.trim()}>{applyingCommand ? 'Пересоздание…' : 'Применить и пересоздать'}</Button>
              </Stack>
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
