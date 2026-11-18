import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchContainerSpec, fetchBuildLog } from '../api/containers'
import { fetchMineContainers, containerStatusPatched } from '../store/slices/containersSlice'
import { refreshContainerByApp, removeContainer, startContainer, stopContainer, applyDockerCommand } from '../api/containers'
import { useRealtimeStatus } from './useRealtime'
import { useContainerLogs } from './useContainerLogs'
import { useEnvConfigs } from './useEnvConfigs'
import type { MineContainer, ContainerSpec } from '../api/containers'
import { ApiError } from '../api/client'
import { openAuthedWS } from '../api/ws'
import { IN_PROGRESS_STATUSES } from '../utils/status'
import { buildDockerRun, specEnvArgs, shellArg, getErrorText } from '../utils/dockerCommand'

const POLL_INTERVAL_MS = 5000
const DELETE_TIMEOUT_MS = 120000
const OPERATION_TIMEOUT_MS = 90000
const REFRESH_TIMEOUT_MS = 600000
const BUILD_LOG_POLL_MS = 5000
const MESSAGE_HIDE_MS = 8000
const CONFIRM_RESET_MS = 5000
const MAX_BUILD_LOG_LINES = 2000
const MAX_LOG_LINES = 1000

export interface UseContainerDetailOptions {
  id: string
  findContainer: (containers: MineContainer[]) => MineContainer | null
  onDeleted?: () => void
}

export interface UseContainerDetailReturn {
  // State
  record: MineContainer | null
  spec: ContainerSpec | null
  selectedEnvConfig: { id: string; name: string; vars: { key: string; value: string }[] } | undefined
  envConfigs: { id: string; name: string; vars: { key: string; value: string }[] }[]
  setEnvConfigId: (id: string) => void
  envConfigId: string
  editedDockerCmd: string | null
  setEditedDockerCmd: (cmd: string | null) => void
  applyingCommand: boolean
  updating: boolean
  deleting: boolean
  confirmDelete: boolean
  setConfirmDelete: (v: boolean) => void
  acting: '' | 'start' | 'stop' | 'restart'
  updateMsg: { ok: boolean; text: string } | null
  syncing: boolean
  syncError: string | null
  syncSuccess: string | null
  connected: boolean
  busy: boolean
  followBuildLog: boolean
  setFollowBuildLog: (v: boolean) => void
  buildLog: string | null
  buildLogError: string
  buildLogBoxRef: React.RefObject<HTMLDivElement | null>
  logsEnabled: boolean
  logs: string[] | null
  logsError: string
  logsLive: boolean
  logsLoading: boolean
  isBuilding: boolean
  generatedDockerCmd: string
  displayedDockerCmd: string
  recordId: string | undefined

  // Actions
  verifyContainerStatus: () => Promise<void>
  handleDelete: () => Promise<void>
  handleRefresh: () => Promise<void>
  applyEditedCommand: () => Promise<void>
  handleOp: (action: 'start' | 'stop' | 'restart') => Promise<void>
  handleCopyLogs: () => void
  showMessage: (ok: boolean, text: string) => void
}

export function useContainerDetail(options: UseContainerDetailOptions): UseContainerDetailReturn {
  const { id, findContainer, onDeleted } = options
  const dispatch = useAppDispatch()

  const { configs } = useEnvConfigs()
  const [envConfigId, setEnvConfigId] = useState('')
  const [editedDockerCmd, setEditedDockerCmd] = useState<string | null>(null)
  const [applyingCommand, setApplyingCommand] = useState(false)
  const [updating, setUpdating] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [acting, setActing] = useState<'' | 'start' | 'stop' | 'restart'>('')
  const [updateMsg, setUpdateMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const [syncing, setSyncing] = useState(false)
  const [syncError, setSyncError] = useState<string | null>(null)
  const [syncSuccess, setSyncSuccess] = useState<string | null>(null)
  const [followBuildLog, setFollowBuildLog] = useState(true)
  const [buildLog, setBuildLog] = useState<string | null>(null)
  const [buildLogError, setBuildLogError] = useState('')
  const buildLogBoxRef = useRef<HTMLDivElement | null>(null)

  const record = useAppSelector((state) => findContainer(state.containers.mine) || null)
  const [spec, setSpec] = useState<ContainerSpec | null>(null)
  const cancelWaitRef = useRef(false)
  const connected = useRealtimeStatus()

  const selectedEnv = configs.find((config) => config.id === envConfigId)

  // Verify actual container status by fetching spec
  const verifyContainerStatus = useCallback(async () => {
    setSyncing(true)
    setSyncError(null)
    setSyncSuccess(null)

    try {
      // Fetch fresh record from backend to avoid stale closure
      const containers = await dispatch(fetchMineContainers()).unwrap()
      const currentRecord = findContainer(containers)

      if (!currentRecord) {
        setSyncError('Контейнер не найден')
        setSyncing(false)
        return
      }

      const specRes = await fetchContainerSpec(currentRecord.id)
      const actualStatus = specRes.data.status

      if (actualStatus !== currentRecord.status) {
        // Бэкенд уже обновил статус в БД, но Redux мог остаться со старым
        // (например, после сообщения «контейнер не найден» в логах).
        dispatch(containerStatusPatched({ id: currentRecord.id, status: actualStatus }))
        setSyncSuccess(`Статус исправлен: было "${currentRecord.status}", стало "${actualStatus}"`)
      } else {
        setSyncSuccess('Статус актуален, изменений нет')
      }
      // Also update spec
      setSpec(specRes.data)
    } catch (err: unknown) {
      // Handle 404 - backend DB out of sync with Docker on agent
      // Try to trigger a refresh from the agent
      if (err instanceof ApiError && err.status === 404) {
        try {
          await refreshContainerByApp(id || '')
          setSyncSuccess('Запрошено обновление с агента, статус обновится через несколько секунд')
        } catch {
          setSyncError('Не удалось запросить обновление контейнера')
        }
      } else {
        setSyncError(err instanceof ApiError && err.message ? err.message : 'Не удалось проверить статус контейнера')
      }
    } finally {
      setSyncing(false)
    }
  }, [id, findContainer, dispatch])

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

  // Auto-verify status on initial load and when switching to another container
  const verifiedRef = useRef<string | null>(null)
  useEffect(() => {
    const rid = record?.id
    if (!rid || verifiedRef.current === rid) {
      return
    }
    verifiedRef.current = rid
    void verifyContainerStatus()
  }, [record?.id, verifyContainerStatus])

  // Fallback status sync: only while the WebSocket is down
  useEffect(() => {
    if (!record || connected) return

    const interval = setInterval(() => {
      verifyContainerStatus()
    }, 30000)

    return () => clearInterval(interval)
  }, [record, connected, verifyContainerStatus])

  // Fallback record polling while building
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
  }, [record, record?.id, record?.status, busy, connected, dispatch])

  useEffect(() => {
    const recordId = record?.id
    // Новый контейнер — не показываем spec/лог сборки предыдущего.
    // Сброс намеренный: состояние привязано к id контейнера.
    /* eslint-disable react-hooks/set-state-in-effect */
    setSpec(null)
    setBuildLog(null)
    setBuildLogError('')
    setEditedDockerCmd(null)
    /* eslint-enable react-hooks/set-state-in-effect */
    if (!recordId) {
      return
    }
    let alive = true
    fetchContainerSpec(recordId)
      .then((res) => {
        if (alive) setSpec(res.data)
      })
      .catch(() => {
        if (alive) setSpec(null)
      })
    return () => {
      alive = false
    }
  }, [record?.id])

  const logsEnabled = !!record // We'll let the component decide which tab
  const { lines: logs, error: logsError, live: logsLive } = useContainerLogs(record?.id || '', logsEnabled, MAX_LOG_LINES)
  const logsLoading = logsEnabled && logs === null && !logsError

  const isBuilding = record ? IN_PROGRESS_STATUSES.has(record.status) : false
  const recordId = record?.id

  // Build log WebSocket handling
  useEffect(() => {
    if (!recordId) {
      return
    }

    if (!isBuilding) {
      // Сборка завершилась (успешно или с ошибкой) — живой поток больше
      // недоступен/не нужен, подгружаем сохранённый лог по HTTP, чтобы
      // причину падения было видно и после перехода статуса в error.
      let alive = true
      fetchBuildLog(recordId)
        .then((res) => {
          if (alive) {
            setBuildLog(res.data ? res.data.replace(/\x1b\[[0-9;]*m/g, '') : '') // eslint-disable-line no-control-regex
            setBuildLogError('')
          }
        })
        .catch(() => {
          if (alive) {
            setBuildLog(null)
            setBuildLogError('')
          }
        })
      return () => {
        alive = false
      }
    }

    let alive = true
    let socket: WebSocket | null = null
    let gotData = false
    let pollTimer: ReturnType<typeof setInterval> | null = null

    // Фолбэк, если живой WS-поток недоступен (например, у агента не задан
    // AGENT_PUBLIC_URL): агент всё равно пишет лог сборки в своё хранилище,
    // забираем его периодически по HTTP.
    const startHttpFallback = () => {
      if (pollTimer !== null) return
      const load = () => {
        fetchBuildLog(recordId)
          .then((res) => {
            if (alive && res.data) {
              gotData = true
              setBuildLogError('')
              setBuildLog(stripAnsi(res.data))
            }
          })
          .catch(() => {})
      }
      load()
      pollTimer = setInterval(load, BUILD_LOG_POLL_MS)
    }

    // Docker/buildkit красит вывод ANSI-кодами (например, \x1b[91m для ошибок) —
    // в окне лога они мусором, убираем.
    const stripAnsi = (text: string) => text.replace(/\x1b\[[0-9;]*m/g, '') // eslint-disable-line no-control-regex

    const appendLine = (line: string) => {
      if (!alive) {
        return
      }
      gotData = true
      setBuildLogError('')
      setBuildLog((prev) => {
        const clean = stripAnsi(line)
        if (prev === null || prev === '') {
          return clean
        }
        const lines = prev.split('\n')
        if (lines.length >= MAX_BUILD_LOG_LINES) {
          return lines.slice(-(MAX_BUILD_LOG_LINES - 1)).join('\n') + '\n' + clean
        }
        return `${prev}\n${clean}`
      })
    }

    const disposeWS = openAuthedWS(`/api/v1/containers/${recordId}/build-log/stream`, {
      onOpen: (ws) => {
        socket = ws
        ws.onmessage = (event) => appendLine(String(event.data))
        ws.onerror = () => {
          if (alive && !gotData) {
            setBuildLogError('Живой поток недоступен, обновляю лог по HTTP…')
            startHttpFallback()
          }
        }
      },
      onClose: () => {
        if (alive && !gotData) {
          setBuildLogError('Живой поток недоступен, обновляю лог по HTTP…')
          startHttpFallback()
        }
      },
    })

    return () => {
      alive = false
      if (pollTimer !== null) {
        clearInterval(pollTimer)
      }
      disposeWS()
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
    navigator.clipboard.writeText(allLogs)
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
      onDeleted?.()
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
      await refreshContainerByApp(record.app_name || '')
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

  return {
    // State
    record,
    spec,
    selectedEnvConfig: selectedEnv,
    envConfigs: configs,
    setEnvConfigId,
    envConfigId,
    editedDockerCmd,
    setEditedDockerCmd,
    applyingCommand,
    updating,
    deleting,
    confirmDelete,
    setConfirmDelete,
    acting,
    updateMsg,
    syncing,
    syncError,
    syncSuccess,
    connected,
    busy,
    followBuildLog,
    setFollowBuildLog,
    buildLog,
    buildLogError,
    buildLogBoxRef,
    logsEnabled,
    logs,
    logsError,
    logsLive,
    logsLoading,
    isBuilding,
    generatedDockerCmd,
    displayedDockerCmd,
    recordId,

    // Actions
    verifyContainerStatus,
    handleDelete,
    handleRefresh,
    applyEditedCommand,
    handleOp,
    handleCopyLogs,
    showMessage,
  }
}