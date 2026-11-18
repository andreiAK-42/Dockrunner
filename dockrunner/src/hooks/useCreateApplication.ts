import { useState, useEffect, useRef, useMemo } from 'react'
import type { Dispatch, SetStateAction } from 'react'
import { useAgents } from './useAgents'
import { usePorts } from './usePorts'
import { useEnvConfigs } from './useEnvConfigs'
import { checkRepository, fetchCheck } from '../api/containers'
import { fetchQuotas } from '../api/quotas'
import { fetchPricing } from '../api/admin'
import { createDeploymentRequest } from '../api/deploymentRequests'
import type { WorkerNode } from '../types/workerNode'
import type { CheckStatus, DetectedPort } from '../api/containers'
import type { PortInfo } from './usePorts'
import type { PricingRate } from '../api/admin'
import { ApiError } from '../api/client'
import {
  randomFreePorts,
  parsePort,
  nextFreeHostPort,
  buildMappedPorts,
  type PortRow,
  type MappedPort,
  DEFAULT_PORTS_MIN,
  DEFAULT_PORTS_MAX,
  SUGGEST_PORTS_COUNT,
} from '../utils/ports'
import {
  waitForCheck,
  CANCELLED_MESSAGE,
} from '../utils/check'
import {
  findLeastLoadedNode,
} from '../utils/nodes'
import {
  buildMappedVolumes,
  buildEnvMap,
  type VolumeRow,
  type EnvRow,
  type MappedVolume,
} from '../utils/docker'
import { useFormArray } from './useFormArray'

export interface UseCreateApplicationReturn {
  // State
  appName: string
  setAppName: (v: string) => void
  repoUrl: string
  setRepoUrl: (v: string) => void
  branch: string
  setBranch: (v: string) => void
  deploymentMode: 'automatic' | 'manual'
  setDeploymentMode: (v: 'automatic' | 'manual') => void
  selectedNodeId: string
  setSelectedNodeId: (v: string) => void

  // Form arrays
  ports: ReturnType<typeof useFormArray<PortRow>>
  volumes: ReturnType<typeof useFormArray<VolumeRow>>
  envRows: ReturnType<typeof useFormArray<EnvRow>>

  // Env config
  envConfigId: string
  setEnvConfigId: (v: string) => void
  appliedKeys: string[]
  setAppliedKeys: (v: string[]) => void
  configs: { id: string; name: string; vars: { key: string; value: string }[] }[]
  fillFromConfig: (id: string) => void

  // Check state
  check: CheckStatus | null
  setCheck: (v: CheckStatus | null) => void
  autoFillCheck: CheckStatus | null
  setAutoFillCheck: (v: CheckStatus | null) => void
  showAutoFillDialog: boolean
  setShowAutoFillDialog: (v: boolean) => void
  autoFillBranch: string
  setAutoFillBranch: (v: string) => void
  autoFillEnvKeys: string[]
  setAutoFillEnvKeys: Dispatch<SetStateAction<string[]>>
  autoFillPortIndices: number[]
  setAutoFillPortIndices: Dispatch<SetStateAction<number[]>>

  // Build mode
  buildMode: 'dockerfile' | 'compose'
  setBuildMode: (v: 'dockerfile' | 'compose') => void
  selectedServices: string[]
  setSelectedServices: (v: string[]) => void
  showBuildModeDialog: boolean
  setShowBuildModeDialog: (v: boolean) => void
  pendingBuildMode: 'dockerfile' | 'compose'
  setPendingBuildMode: (v: 'dockerfile' | 'compose') => void
  pendingServices: string[]
  setPendingServices: Dispatch<SetStateAction<string[]>>

  // Loading states
  checking: boolean
  elapsed: number
  deploying: boolean
  formError: string
  setFormError: (v: string) => void
  toast: { severity: 'success' | 'error'; message: string } | null
  setToast: (v: { severity: 'success' | 'error'; message: string } | null) => void
  requestSubmitted: boolean
  requestId: string | null

  // Resource requests
  cpuCores: number
  setCpuCores: (v: number) => void
  memoryGB: number
  setMemoryGB: (v: number) => void
  diskGB: number
  setDiskGB: (v: number) => void
  durationHours: number
  setDurationHours: (v: number) => void
  rates: PricingRate[]
  priceEstimate: number | null

  // Nodes and ports
  nodes: WorkerNode[]
  onlineNodes: WorkerNode[]
  recommendedNode: WorkerNode | undefined
  busyPorts: PortInfo[]
  freeCount: number
  portsRange: { min: number; max: number }
  setPortsRange: (v: { min: number; max: number }) => void
  suggestionSeed: number
  setSuggestionSeed: (v: number) => void
  regenerateSuggestions: () => void
  suggestions: number[]

  // Actions
  handleCheck: () => Promise<void>
  handleCancelCheck: () => void
  handleAutoFill: () => Promise<void>
  handleAutoFillApply: () => void
  handleAutoFillCancel: () => void
  handleBuildModeConfirm: () => void
  handleBuildModeCancel: () => void
  handleDeploy: () => Promise<void>
}

export function useCreateApplication(): UseCreateApplicationReturn {
  const [appName, setAppName] = useState('')
  const [repoUrl, setRepoUrl] = useState('')
  const [branch, setBranch] = useState('')
  const [deploymentMode, setDeploymentMode] = useState<'automatic' | 'manual'>('automatic')
  const [selectedNodeId, setSelectedNodeId] = useState<string>('')

  // Form arrays using the new hook
  const ports = useFormArray<PortRow>({ initialValue: [{ host: '', container: '80' }] })
  const volumes = useFormArray<VolumeRow>({ initialValue: [{ hostPath: '', containerPath: '' }] })
  const envRows = useFormArray<EnvRow>({ initialValue: [{ key: '', value: '' }] })

  const [appliedKeys, setAppliedKeys] = useState<string[]>([])
  const [envConfigId, setEnvConfigId] = useState<string>('')
  const { configs } = useEnvConfigs()

  const [check, setCheck] = useState<CheckStatus | null>(null)
  const [autoFillCheck, setAutoFillCheck] = useState<CheckStatus | null>(null)
  const [showAutoFillDialog, setShowAutoFillDialog] = useState(false)
  const [autoFillBranch, setAutoFillBranch] = useState('')
  const [autoFillEnvKeys, setAutoFillEnvKeys] = useState<string[]>([])
  const [autoFillPortIndices, setAutoFillPortIndices] = useState<number[]>([])

  const [buildMode, setBuildMode] = useState<'dockerfile' | 'compose'>('dockerfile')
  const [selectedServices, setSelectedServices] = useState<string[]>([])
  const [showBuildModeDialog, setShowBuildModeDialog] = useState(false)
  const [pendingBuildMode, setPendingBuildMode] = useState<'dockerfile' | 'compose'>('dockerfile')
  const [pendingServices, setPendingServices] = useState<string[]>([])

  const [checking, setChecking] = useState(false)
  const [elapsed, setElapsed] = useState(0)
  const [deploying, setDeploying] = useState(false)
  const [formError, setFormError] = useState('')
  const [toast, setToast] = useState<{ severity: 'success' | 'error'; message: string } | null>(null)
  const [requestSubmitted, setRequestSubmitted] = useState(false)
  const [requestId, setRequestId] = useState<string | null>(null)

  // Resource request state
  const [cpuCores, setCpuCores] = useState(1)
  const [memoryGB, setMemoryGB] = useState(1)
  const [diskGB, setDiskGB] = useState(5)
  const [durationHours, setDurationHours] = useState(10)
  const [rates, setRates] = useState<PricingRate[]>([])

  const { nodes } = useAgents()
  const { ports: busyPorts, freeCount } = usePorts()
  const [portsRange, setPortsRange] = useState({ min: DEFAULT_PORTS_MIN, max: DEFAULT_PORTS_MAX })
  const [suggestionSeed, setSuggestionSeed] = useState(0)

  const onlineNodes = useMemo(() => nodes.filter((node) => node.status === 'online'), [nodes])
  const recommendedNode = useMemo(() => findLeastLoadedNode(onlineNodes), [onlineNodes])

  let effectiveNodeId = recommendedNode?.id || ''
  if (deploymentMode === 'manual') {
    effectiveNodeId = selectedNodeId
  }
  const isReady = check?.status === 'ready'

  // Suggestions for port selection
  const suggestions = useMemo(() => {
    const busyOnNode = new Set(
      busyPorts.filter((busy) => busy.nodeId === effectiveNodeId).map((busy) => busy.port),
    )
    const selectedHosts = new Set(
      ports.items.map((row) => parsePort(row.host)).filter((port): port is number => port !== null),
    )
    const taken = new Set([...busyOnNode, ...selectedHosts])
    const count = Math.min(SUGGEST_PORTS_COUNT, Math.max(0, freeCount))
    return randomFreePorts(count, portsRange.min, portsRange.max, taken, suggestionSeed + portsRange.min * 31 + portsRange.max)
  }, [effectiveNodeId, busyPorts, ports.items, freeCount, portsRange, suggestionSeed])

  const regenerateSuggestions = () => {
    setSuggestionSeed((seed) => seed + 1)
  }

  const activeServices = buildMode === 'compose' ? selectedServices : []
  const serviceCount = buildMode === 'compose' ? activeServices.length : 1

  const priceEstimate = useMemo(() => {
    const rate = rates.find((r) => r.agent_id === effectiveNodeId)
    if (!rate) {
      return null
    }
    const hourly = cpuCores * rate.cpu_per_core_hour + memoryGB * rate.mem_per_gb_hour + diskGB * rate.disk_per_gb_hour
    return Math.round(hourly * durationHours * serviceCount * 100) / 100
  }, [rates, effectiveNodeId, cpuCores, memoryGB, diskGB, durationHours, serviceCount])

  // Effects for initial data
  useEffect(() => {
    fetchQuotas()
      .then((res) => {
        const rangeValid = res.data.ports_min > 0 && res.data.ports_max >= res.data.ports_min
        if (rangeValid) {
          setPortsRange({ min: res.data.ports_min, max: res.data.ports_max })
        }
      })
      .catch(() => { /* останутся дефолты */ })
    fetchPricing().then(setRates).catch(() => setRates([]))
  }, [])

  const cancelRef = useRef(false)

  const fillFromConfig = (id: string) => {
    setEnvConfigId(id)
    const selectedConfig = configs.find((config) => config.id === id)

    const keptRows = envRows.items.filter((row) => {
      const key = row.key.trim()
      const isOwnRow = key !== '' && !appliedKeys.includes(key)
      return isOwnRow
    })

    if (!selectedConfig) {
      setAppliedKeys([])
      if (keptRows.length > 0) {
        envRows.setItems(keptRows)
      } else {
        envRows.setItems([{ key: '', value: '' }])
      }
      return
    }

    const keptKeys = new Set(keptRows.map((row) => row.key.trim()))
    const addedVars = selectedConfig.vars.filter((configVar) => !keptKeys.has(configVar.key))
    const addedRows = addedVars.map((configVar) => ({ key: configVar.key, value: configVar.value }))
    const selectedKeys = selectedConfig.vars.map((configVar) => configVar.key)

    setAppliedKeys(selectedKeys)
    envRows.setItems([...keptRows, ...addedRows])
  }

  const handleCheck = async () => {
    const appNameFilled = appName.trim()
    const repoUrlFilled = repoUrl.trim()
    const branchFilled = branch.trim()

    if (!appNameFilled || !repoUrlFilled) {
      setFormError('Укажите название приложения и ссылку на репозиторий')
      return
    }
    if (!branchFilled) {
      setFormError('Укажите ветку Git для деплоя')
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
      const checkRes = await checkRepository({ app_name: appNameFilled, repo_url: repoUrlFilled, node_id: effectiveNodeId, branch: branchFilled })
      const finishedCheck = await waitForCheck(checkRes.data.id, () => cancelRef.current, fetchCheck)
      setCheck(finishedCheck)
      if (finishedCheck.status === 'ready') {
        const found = [
          finishedCheck.detected_dockerfile ? 'Dockerfile' : '',
          finishedCheck.compose_file || '',
        ].filter(Boolean).join(' и ')
        setToast({ severity: 'success', message: found ? `Репозиторий проверен — найден ${found}` : 'Репозиторий проверен' })

        // Auto-fill detected environment variables from .env/.env.example
        if (finishedCheck.detected_env_vars && Object.keys(finishedCheck.detected_env_vars).length > 0) {
          const detectedEntries = Object.entries(finishedCheck.detected_env_vars)
          const keptRows = envRows.items.filter((row) => {
            const key = row.key.trim()
            return key !== '' && !appliedKeys.includes(key)
          })
          const keptKeys = new Set(keptRows.map((row) => row.key.trim()))
          const addedRows = detectedEntries
            .filter(([key]) => !keptKeys.has(key))
            .map(([key, value]) => ({ key, value }))

          setAppliedKeys(detectedEntries.map(([key]) => key))
          envRows.setItems([...keptRows, ...addedRows])
        }

        // Show modal for build mode selection
        const defaultMode = finishedCheck.compose_file ? 'compose' : 'dockerfile'
        setPendingBuildMode(defaultMode)
        setPendingServices(finishedCheck.services || [])
        setShowBuildModeDialog(true)
      } else {
        setToast({ severity: 'error', message: finishedCheck.error || 'Проверка не пройдена' })
      }
    } catch (checkError) {
      const isCancelled = checkError instanceof Error && checkError.message === CANCELLED_MESSAGE
      if (isCancelled) {
        return
      }

      const isLostCheck = checkError instanceof ApiError && checkError.status === 404
      if (isLostCheck) {
        setToast({ severity: 'error', message: 'Проверка потеряна — сервер перезапускался? Запустите снова' })
        return
      }

      if (checkError instanceof Error) {
        setToast({ severity: 'error', message: checkError.message })
      } else {
        setToast({ severity: 'error', message: 'Проверка не удалась' })
      }
    } finally {
      clearInterval(timer)
      setChecking(false)
    }
  }

  const handleCancelCheck = () => {
    cancelRef.current = true
  }

  const handleAutoFill = async () => {
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
      setCheck(finishedCheck) // Also set main check so button becomes "Отправить на рассмотрение"

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
        setToast({ severity: 'error', message: 'Проверка потеряна — сервер перезапускался? Запустите снова' })
        return
      }
      if (checkError instanceof Error) {
        setToast({ severity: 'error', message: checkError.message })
      } else {
        setToast({ severity: 'error', message: 'Автозаполнение не удалось' })
      }
    } finally {
      clearInterval(timer)
      setChecking(false)
    }
  }

  const handleAutoFillApply = () => {
    if (!autoFillCheck) return

    // Apply branch
    if (autoFillBranch) {
      setBranch(autoFillBranch)
    }

    // Apply env vars
    if (autoFillCheck.detected_env_vars && autoFillEnvKeys.length > 0) {
      const keptRows = envRows.items.filter((row) => {
        const key = row.key.trim()
        return key !== '' && !appliedKeys.includes(key)
      })
      const keptKeys = new Set(keptRows.map((row) => row.key.trim()))
      const addedRows = autoFillEnvKeys
        .filter((key) => !keptKeys.has(key) && autoFillCheck.detected_env_vars![key])
        .map((key) => ({ key, value: autoFillCheck.detected_env_vars![key] }))

      setAppliedKeys([...appliedKeys, ...autoFillEnvKeys])
      envRows.setItems([...keptRows, ...addedRows])
    }

    // Apply ports: replace empty/default rows, keep already filled mappings
    if (autoFillCheck.detected_ports && autoFillPortIndices.length > 0) {
      // Filter selected ports by the chosen build mode
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
      const keptRows = ports.items.filter((row) => {
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
      ports.setItems(nextPorts.length > 0 ? nextPorts : [{ host: '', container: '80' }])
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
    setToast({ severity: 'success', message: 'Данные автозаполнения применены' })
  }

  const handleAutoFillCancel = () => {
    setShowAutoFillDialog(false)
    setAutoFillCheck(null)
    setAutoFillBranch('')
    setAutoFillEnvKeys([])
    setAutoFillPortIndices([])
  }

  const handleBuildModeConfirm = () => {
    setBuildMode(pendingBuildMode)
    setSelectedServices(pendingServices)
    setShowBuildModeDialog(false)
  }

  const handleBuildModeCancel = () => {
    setShowBuildModeDialog(false)
    // Reset check so user can re-check if needed
    setCheck(null)
  }

  const handleDeploy = async () => {
    if (!check || !isReady) {
      return
    }
    if (buildMode === 'compose' && selectedServices.length === 0) {
      setFormError('Выберите хотя бы один сервис Docker Compose')
      return
    }

    let mappedPorts: MappedPort[]
    let mappedVolumes: MappedVolume[]
    try {
      mappedPorts = buildMappedPorts(ports.items, portsRange, busyPorts, effectiveNodeId)
      mappedVolumes = buildMappedVolumes(volumes.items)
    } catch (validationError) {
      if (validationError instanceof Error) {
        setFormError(validationError.message)
      } else {
        setFormError('Проверьте порты и тома')
      }
      return
    }
    const env = buildEnvMap(envRows.items)

    setFormError('')
    setDeploying(true)

    try {
      const quotas = await fetchQuotas().catch(() => null)
      if (quotas) {
        const freePorts = quotas.data.ports_limit - quotas.data.ports_used
        if (mappedPorts.length > freePorts) {
          throw new Error(`Превышен лимит портов: свободно ${freePorts}`)
        }
      }

      const res = await createDeploymentRequest({
        app_name: appName.trim(),
        repo_url: repoUrl.trim(),
        container_name: appName.trim(),
        node_id: effectiveNodeId,
        ports: mappedPorts,
        volumes: mappedVolumes,
        env,
        cpu_cores: cpuCores,
        memory_gb: memoryGB,
        disk_gb: diskGB,
        duration_hours: durationHours,
        build_mode: buildMode,
        services: activeServices,
        branch: branch.trim(),
      })

      setRequestSubmitted(true)
      setRequestId(res.data.id)
      setFormError('Заявка отправлена на одобрение администратору. Ожидайте решения.')
    } catch (deployError) {
      if (deployError instanceof Error) {
        setFormError(deployError.message)
      } else {
        setFormError('Отправка заявки не удалась')
      }
    } finally {
      setDeploying(false)
    }
  }

  return {
    // State
    appName,
    setAppName,
    repoUrl,
    setRepoUrl,
    branch,
    setBranch,
    deploymentMode,
    setDeploymentMode,
    selectedNodeId,
    setSelectedNodeId,

    // Form arrays
    ports,
    volumes,
    envRows,

    // Env config
    envConfigId,
    setEnvConfigId,
    appliedKeys,
    setAppliedKeys,
    configs,
    fillFromConfig,

    // Check state
    check,
    setCheck,
    autoFillCheck,
    setAutoFillCheck,
    showAutoFillDialog,
    setShowAutoFillDialog,
    autoFillBranch,
    setAutoFillBranch,
    autoFillEnvKeys,
    setAutoFillEnvKeys,
    autoFillPortIndices,
    setAutoFillPortIndices,

    // Build mode
    buildMode,
    setBuildMode,
    selectedServices,
    setSelectedServices,
    showBuildModeDialog,
    setShowBuildModeDialog,
    pendingBuildMode,
    setPendingBuildMode,
    pendingServices,
    setPendingServices,

    // Loading states
    checking,
    elapsed,
    deploying,
    formError,
    setFormError,
    toast,
    setToast,
    requestSubmitted,
    requestId,

    // Resource requests
    cpuCores,
    setCpuCores,
    memoryGB,
    setMemoryGB,
    diskGB,
    setDiskGB,
    durationHours,
    setDurationHours,
    rates,
    priceEstimate,

    // Nodes and ports
    nodes,
    onlineNodes,
    recommendedNode,
    busyPorts,
    freeCount,
    portsRange,
    setPortsRange,
    suggestionSeed,
    setSuggestionSeed,
    regenerateSuggestions,
    suggestions,

    // Actions
    handleCheck,
    handleCancelCheck,
    handleAutoFill,
    handleAutoFillApply,
    handleAutoFillCancel,
    handleBuildModeConfirm,
    handleBuildModeCancel,
    handleDeploy,
  }
}