import { useState, useCallback, useRef } from 'react'
import { checkRepository, fetchCheck } from '../../../api/containers'
import type { CheckStatus, DetectedPort } from '../../../api/containers'
import { ApiError } from '../../../api/client'
import { waitForCheck, CANCELLED_MESSAGE } from '../../../utils/check'
import { nextFreeHostPort, parsePort, type PortRow } from '../../../utils/ports'
import type { PortInfo } from '../../../hooks/usePorts'

export interface UseAutoFillOptions {
  appName: string
  repoUrl: string
  branch: string
  effectiveNodeId: string
  portsRange: { min: number; max: number }
  busyPorts: PortInfo[]
  ports: PortRow[]
  envRows: { key: string; value: string }[]
  appliedKeys: string[]
  setBranch: React.Dispatch<React.SetStateAction<string>>
  setEnvRows: React.Dispatch<React.SetStateAction<{ key: string; value: string }[]>>
  setAppliedKeys: React.Dispatch<React.SetStateAction<string[]>>
  setPorts: React.Dispatch<React.SetStateAction<PortRow[]>>
  setBuildMode: React.Dispatch<React.SetStateAction<'dockerfile' | 'compose'>>
  setSelectedServices: React.Dispatch<React.SetStateAction<string[]>>
  setCheck: React.Dispatch<React.SetStateAction<CheckStatus | null>>
  setFormError: React.Dispatch<React.SetStateAction<string>>
}

export interface UseAutoFillReturn {
  autoFillCheck: CheckStatus | null
  showAutoFillDialog: boolean
  autoFillBranch: string
  setAutoFillBranch: React.Dispatch<React.SetStateAction<string>>
  autoFillEnvKeys: string[]
  setAutoFillEnvKeys: React.Dispatch<React.SetStateAction<string[]>>
  autoFillPortIndices: number[]
  setAutoFillPortIndices: React.Dispatch<React.SetStateAction<number[]>>
  pendingBuildMode: 'dockerfile' | 'compose'
  setPendingBuildMode: React.Dispatch<React.SetStateAction<'dockerfile' | 'compose'>>
  pendingServices: string[]
  setPendingServices: React.Dispatch<React.SetStateAction<string[]>>
  checking: boolean
  elapsed: number
  handleAutoFill: () => Promise<void>
  handleAutoFillApply: () => void
  handleAutoFillCancel: () => void
}

/**
 * Hook for handling auto-fill logic
 */
export function useAutoFill(options: UseAutoFillOptions): UseAutoFillReturn {
  const {
    appName, repoUrl, branch, effectiveNodeId, portsRange, busyPorts, ports, envRows,
    appliedKeys, setBranch, setEnvRows, setAppliedKeys, setPorts,
    setBuildMode, setSelectedServices, setCheck, setFormError,
  } = options

  const [autoFillCheck, setAutoFillCheck] = useState<CheckStatus | null>(null)
  const [showAutoFillDialog, setShowAutoFillDialog] = useState(false)
  const [autoFillBranch, setAutoFillBranch] = useState('')
  const [autoFillEnvKeys, setAutoFillEnvKeys] = useState<string[]>([])
  const [autoFillPortIndices, setAutoFillPortIndices] = useState<number[]>([])
  const [pendingBuildMode, setPendingBuildMode] = useState<'dockerfile' | 'compose'>('dockerfile')
  const [pendingServices, setPendingServices] = useState<string[]>([])
  const [checking, setChecking] = useState(false)
  const [elapsed, setElapsed] = useState(0)
  const cancelRef = useRef(false)

  const handleAutoFill = useCallback(async () => {
    const repoUrlFilled = repoUrl.trim()
    if (!repoUrlFilled) {
      setFormError('Введите ссылку на репозиторий')
      return
    }
    if (!effectiveNodeId) {
      setFormError('Выберите узел для развёртывания')
      return
    }

    setFormError('')
    setChecking(true)
    setElapsed(0)
    cancelRef.current = false

    const timer = setInterval(() => setElapsed((seconds) => seconds + 1), 1000)

    try {
      const checkRes = await checkRepository({ app_name: appName.trim() || 'temp', repo_url: repoUrlFilled, node_id: effectiveNodeId, branch: branch.trim() || 'main' })
      const finishedCheck = await waitForCheck(checkRes.data.id, () => cancelRef.current, fetchCheck)
      setAutoFillCheck(finishedCheck)
      setCheck(finishedCheck)

      // Initialize auto-fill selections
      if (finishedCheck.branches && finishedCheck.branches.length > 0) {
        setAutoFillBranch(finishedCheck.branches[0])
      }
      if (finishedCheck.detected_env_vars) {
        setAutoFillEnvKeys(Object.keys(finishedCheck.detected_env_vars))
      }
      if (finishedCheck.detected_ports) {
        setAutoFillPortIndices(finishedCheck.detected_ports.map((_, i) => i))
      }
      // Initialize build mode selection
      if (finishedCheck.compose_file) {
        setPendingBuildMode('compose')
        setPendingServices(finishedCheck.services || [])
      } else {
        setPendingBuildMode('dockerfile')
        setPendingServices([])
      }
      setShowAutoFillDialog(true)
    } catch (checkError) {
      const isCancelled = checkError instanceof Error && checkError.message === CANCELLED_MESSAGE
      if (isCancelled) {
        return
      }
      if (checkError instanceof ApiError && checkError.status === 404) {
        setFormError('Проверка потеряна — сервер перезапускался? Запустите снова')
        return
      }
      if (checkError instanceof Error) {
        setFormError(checkError.message)
      } else {
        setFormError('Автозаполнение не удалось')
      }
    } finally {
      clearInterval(timer)
      setChecking(false)
    }
  }, [appName, repoUrl, branch, effectiveNodeId, setCheck, setFormError])

  const handleAutoFillApply = useCallback(() => {
    if (!autoFillCheck) return

    // Apply branch
    if (autoFillBranch) {
      setBranch(autoFillBranch)
    }

    // Apply env vars
    if (autoFillCheck.detected_env_vars && autoFillEnvKeys.length > 0) {
      const keptRows = envRows.filter((row) => {
        const key = row.key.trim()
        return key !== '' && !appliedKeys.includes(key)
      })
      const keptKeys = new Set(keptRows.map((row) => row.key.trim()))
      const addedRows = autoFillEnvKeys
        .filter((key) => !keptKeys.has(key) && autoFillCheck.detected_env_vars![key])
        .map((key) => ({ key, value: autoFillCheck.detected_env_vars![key] }))

      setAppliedKeys([...appliedKeys, ...autoFillEnvKeys])
      setEnvRows([...keptRows, ...addedRows])
    }

    // Apply ports: filter by selected build mode
    if (autoFillCheck.detected_ports && autoFillPortIndices.length > 0) {
      const selectedPorts = autoFillPortIndices
        .map((index) => autoFillCheck.detected_ports![index])
        .filter((port): port is DetectedPort => Boolean(port))
        .filter((port) => {
          if (pendingBuildMode === 'dockerfile') {
            return port.source === 'dockerfile'
          }
          if (pendingBuildMode === 'compose') {
            return port.source === 'compose'
          }
          return true
        })

      const usedHosts = new Set(
        busyPorts.filter((busy) => busy.nodeId === effectiveNodeId).map((busy) => busy.port),
      )
      const keptRows = ports.filter((row) => {
        const host = parsePort(row.host)
        const container = parsePort(row.container)
        if (host === null || container === null) {
          return false
        }
        usedHosts.add(host)
        return true
      })
      const usedContainers = new Set(
        keptRows.map((row) => parsePort(row.container)).filter((port): port is number => port !== null),
      )
      const addedRows: PortRow[] = []
      for (const detected of selectedPorts) {
        if (usedContainers.has(detected.container_port)) {
          continue
        }
        const hostPort = nextFreeHostPort(portsRange.min, portsRange.max, usedHosts)
        if (hostPort <= 0) {
          break
        }
        usedHosts.add(hostPort)
        usedContainers.add(detected.container_port)
        addedRows.push({ host: String(hostPort), container: String(detected.container_port) })
      }
      const nextPorts = [...keptRows, ...addedRows]
      setPorts(nextPorts.length > 0 ? nextPorts : [{ host: '', container: '80' }])
    }

    // Apply build mode and selected services
    if (autoFillCheck.compose_file) {
      setBuildMode(pendingBuildMode)
      setSelectedServices(pendingServices)
    } else {
      setBuildMode('dockerfile')
      setSelectedServices([])
    }

    setShowAutoFillDialog(false)
    setAutoFillCheck(null)
    setAutoFillBranch('')
    setAutoFillEnvKeys([])
    setAutoFillPortIndices([])
    setFormError('')
    // Toast will be handled by parent
  }, [autoFillCheck, autoFillBranch, autoFillEnvKeys, autoFillPortIndices, pendingBuildMode, pendingServices,
      envRows, appliedKeys, ports, busyPorts, effectiveNodeId, portsRange,
      setBranch, setEnvRows, setAppliedKeys, setPorts, setBuildMode, setSelectedServices, setFormError])

  const handleAutoFillCancel = useCallback(() => {
    setShowAutoFillDialog(false)
    setAutoFillCheck(null)
    setAutoFillBranch('')
    setAutoFillEnvKeys([])
    setAutoFillPortIndices([])
  }, [])

  return {
    autoFillCheck,
    showAutoFillDialog,
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
    checking,
    elapsed,
    handleAutoFill,
    handleAutoFillApply,
    handleAutoFillCancel,
  }
}