import { useMemo, useRef, useState, useEffect } from 'react'
import { Box, Typography, Card, CardContent, Grid, TextField, MenuItem, Button, Stack, Divider, FormControl, InputLabel, Select, Radio, RadioGroup, FormControlLabel, Checkbox, Alert, IconButton, CircularProgress, Snackbar, Dialog, DialogTitle, DialogContent, DialogActions } from '@mui/material'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft, Save, GitBranch, CheckCircle2, Plus, Trash2, RefreshCw } from 'lucide-react'
import ServerMap from '../../components/ServerMap'
import { percent } from '../../types/workerNode'
import type { WorkerNode } from '../../types/workerNode'
import { useAgents } from '../../hooks/useAgents'
import { usePorts } from '../../hooks/usePorts'
import type { PortInfo } from '../../hooks/usePorts'
import { useEnvConfigs } from '../../hooks/useEnvConfigs'
import { checkRepository, fetchCheck } from '../../api/containers'
import type { CheckStatus } from '../../api/containers'
import { ApiError } from '../../api/client'
import { fetchQuotas } from '../../api/quotas'
import { createDeploymentRequest } from '../../api/deploymentRequests'
import { fetchPricing } from '../../api/admin'
import type { PricingRate } from '../../api/admin'
import styles from './CreateApplication.module.scss'

interface PortRow {
  host: string
  container: string
}

interface VolumeRow {
  hostPath: string
  containerPath: string
}

interface EnvRow {
  key: string
  value: string
}

interface MappedPort {
  host_port: number
  container_port: number
}

interface MappedVolume {
  name?: string
  host_path?: string
  container_path: string
}

const CHECK_POLL_INTERVAL_MS = 2000
const CHECK_TIMEOUT_MS = 90000
const DEFAULT_PORTS_MIN = 3000
const DEFAULT_PORTS_MAX = 4000
const MAX_PORT_NUMBER = 65535
const CANCELLED_MESSAGE = 'cancelled'
const SUGGEST_PORTS_COUNT = 10

function mulberry32(seed: number) {
  let state = seed >>> 0
  return () => {
    state |= 0
    state = (state + 0x6d2b79f5) | 0
    let t = Math.imul(state ^ (state >>> 15), 1 | state)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

function randomFreePorts(count: number, min: number, max: number, taken: Set<number>, seed: number): number[] {
  const picked: number[] = []
  const range = max - min + 1
  if (range <= 0 || count <= 0) {
    return picked
  }
  const rand = mulberry32(seed)
  let guard = 0
  while (picked.length < count && guard < count * 50 + 200) {
    guard += 1
    const port = min + Math.floor(rand() * range)
    if (taken.has(port) || picked.includes(port)) {
      continue
    }
    picked.push(port)
  }
  return picked.sort((a, b) => a - b)
}

function delay(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms))
}

function isCheckFinished(status: string) {
  return status === 'ready' || status === 'failed'
}

async function waitForCheck(id: string, isCancelled: () => boolean): Promise<CheckStatus> {
  const started = Date.now()

  for (;;) {
    if (isCancelled()) {
      throw new Error(CANCELLED_MESSAGE)
    }

    await delay(CHECK_POLL_INTERVAL_MS)

    if (isCancelled()) {
      throw new Error(CANCELLED_MESSAGE)
    }

    const res = await fetchCheck(id)
    if (isCheckFinished(res.data.status)) {
      return res.data
    }

    const waitedTooLong = Date.now() - started > CHECK_TIMEOUT_MS
    if (waitedTooLong) {
      throw new Error('Превышено время ожидания проверки')
    }
  }
}

function getWorkerName(nodes: { id: string; name: string }[], id: string) {
  const foundNode = nodes.find((node) => node.id === id)
  if (foundNode) {
    return foundNode.name
  }
  return id
}

function parsePort(value: string): number | null {
  const parsed = Number(value)
  const isValidPort = Number.isInteger(parsed) && parsed > 0 && parsed <= MAX_PORT_NUMBER
  if (!isValidPort) {
    return null
  }
  return parsed
}

function getNodeLoad(node: WorkerNode): number {
  const cpuLoad = percent(node.cpu.used, node.cpu.total)
  const memoryLoad = percent(node.memory.used, node.memory.total)
  return cpuLoad + memoryLoad
}

function findLeastLoadedNode(onlineNodes: WorkerNode[]): WorkerNode | undefined {
  const sortedByLoad = [...onlineNodes].sort((first, second) => getNodeLoad(first) - getNodeLoad(second))
  return sortedByLoad[0]
}

function buildMappedPorts(portRows: PortRow[], portsRange: { min: number; max: number }, busyPorts: PortInfo[], nodeId: string): MappedPort[] {
  const mappedPorts: MappedPort[] = []

  for (const row of portRows) {
    const bothEmpty = !row.host.trim() && !row.container.trim()
    if (bothEmpty) {
      continue
    }

    const host = parsePort(row.host)
    const container = parsePort(row.container)
    if (host === null || container === null) {
      throw new Error('Порты должны быть числами от 1 до 65535')
    }

    const outOfRange = host < portsRange.min || host > portsRange.max
    if (outOfRange) {
      throw new Error(`Хост-порт должен быть в диапазоне ${portsRange.min}–${portsRange.max}`)
    }

    const busyPort = busyPorts.find((busy) => busy.nodeId === nodeId && busy.port === host)
    if (busyPort) {
      throw new Error(`Порт ${host} уже занят приложением «${busyPort.appName}» на выбранном узле`)
    }

    mappedPorts.push({ host_port: host, container_port: container })
  }

  return mappedPorts
}

function buildMappedVolumes(volumeRows: VolumeRow[]): MappedVolume[] {
  const mappedVolumes: MappedVolume[] = []

  for (const row of volumeRows) {
    const containerPath = row.containerPath.trim()
    const hostPath = row.hostPath.trim()

    const bothEmpty = !containerPath && !hostPath
    if (bothEmpty) {
      continue
    }

    if (!containerPath) {
      throw new Error('У каждого тома должен быть путь в контейнере')
    }

    const volume: MappedVolume = { container_path: containerPath }

    if (hostPath) {
      const isPath = hostPath.includes('/') || hostPath.includes('\\') || hostPath.startsWith('.')
      if (isPath) {
        volume.host_path = hostPath
      } else {
        volume.name = hostPath
      }
    }

    mappedVolumes.push(volume)
  }

  return mappedVolumes
}

function buildEnvMap(envRows: EnvRow[]): Record<string, string> {
  const env: Record<string, string> = {}

  for (const row of envRows) {
    const key = row.key.trim()
    if (key) {
      env[key] = row.value
    }
  }

  return env
}

export default function CreateApplication() {
  const navigate = useNavigate()
  const [appName, setAppName] = useState('')
  const [repoUrl, setRepoUrl] = useState('')
  const [branch, setBranch] = useState('')
  const [deploymentMode, setDeploymentMode] = useState<'automatic' | 'manual'>('automatic')
  const [selectedNodeId, setSelectedNodeId] = useState<string>('')
  const [ports, setPorts] = useState<PortRow[]>([{ host: '', container: '80' }])
  const [volumes, setVolumes] = useState<VolumeRow[]>([{ hostPath: '', containerPath: '' }])
  const [envRows, setEnvRows] = useState<EnvRow[]>([{ key: '', value: '' }])
  const [appliedKeys, setAppliedKeys] = useState<string[]>([])
  const [envConfigId, setEnvConfigId] = useState<string>('')
  const { configs } = useEnvConfigs()
  const [check, setCheck] = useState<CheckStatus | null>(null)
  const [buildMode, setBuildMode] = useState<'dockerfile' | 'compose'>('dockerfile')
  const [selectedServices, setSelectedServices] = useState<string[]>([])
  const [checking, setChecking] = useState(false)
  const [elapsed, setElapsed] = useState(0)
  const [deploying, setDeploying] = useState(false)
  const [formError, setFormError] = useState('')
  const [toast, setToast] = useState<{ severity: 'success' | 'error', message: string } | null>(null)
  const [requestSubmitted, setRequestSubmitted] = useState(false)
  const [requestId, setRequestId] = useState<string | null>(null)
  const [showBuildModeDialog, setShowBuildModeDialog] = useState(false)
  const [pendingBuildMode, setPendingBuildMode] = useState<'dockerfile' | 'compose'>('dockerfile')
  const [pendingServices, setPendingServices] = useState<string[]>([])
  const cancelRef = useRef(false)

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

  const onlineNodes = useMemo(() => nodes.filter((node) => node.status === 'online'), [nodes])
  const recommendedNode = useMemo(() => findLeastLoadedNode(onlineNodes), [onlineNodes])

  let effectiveNodeId = recommendedNode?.id || ''
  if (deploymentMode === 'manual') {
    effectiveNodeId = selectedNodeId
  }
  const isReady = check?.status === 'ready'

  // Варианты хост-портов: случайные свободные на узле, не более лимита.
  // Пересчитываются при смене узла/лимита/выбора и по кнопке «Другие варианты».
  const suggestions = useMemo(() => {
    const busyOnNode = new Set(
      busyPorts.filter((busy) => busy.nodeId === effectiveNodeId).map((busy) => busy.port),
    )
    const selectedHosts = new Set(
      ports.map((row) => parsePort(row.host)).filter((port): port is number => port !== null),
    )
    const taken = new Set([...busyOnNode, ...selectedHosts])
    const count = Math.min(SUGGEST_PORTS_COUNT, Math.max(0, freeCount))
    return randomFreePorts(count, portsRange.min, portsRange.max, taken, suggestionSeed + portsRange.min * 31 + portsRange.max)
  }, [effectiveNodeId, busyPorts, ports, freeCount, portsRange, suggestionSeed])

  const regenerateSuggestions = () => {
    setSuggestionSeed((seed) => seed + 1)
  }
  const composeServices = useMemo(() => {
    if (!check || check.status !== 'ready' || !check.compose_file) {
      return []
    }
    return check.services || []
  }, [check])
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

  const updatePort = (index: number, patch: Partial<PortRow>) => {
    const nextPorts = ports.map((row, rowIndex) => {
      if (rowIndex === index) {
        return { ...row, ...patch }
      }
      return row
    })
    setPorts(nextPorts)
  }

  const removePort = (index: number) => {
    setPorts(ports.filter((_, rowIndex) => rowIndex !== index))
  }

  const updateVolume = (index: number, patch: Partial<VolumeRow>) => {
    const nextVolumes = volumes.map((row, rowIndex) => {
      if (rowIndex === index) {
        return { ...row, ...patch }
      }
      return row
    })
    setVolumes(nextVolumes)
  }

  const removeVolume = (index: number) => {
    setVolumes(volumes.filter((_, rowIndex) => rowIndex !== index))
  }

  const updateEnv = (index: number, patch: Partial<EnvRow>) => {
    const nextRows = envRows.map((row, rowIndex) => {
      if (rowIndex === index) {
        return { ...row, ...patch }
      }
      return row
    })
    setEnvRows(nextRows)
  }

  const removeEnv = (index: number) => {
    setEnvRows(envRows.filter((_, rowIndex) => rowIndex !== index))
  }

  const fillFromConfig = (id: string) => {
    setEnvConfigId(id)
    const selectedConfig = configs.find((config) => config.id === id)

    // Убираем переменные прошлого конфига, свои строки не трогаем.
    const keptRows = envRows.filter((row) => {
      const key = row.key.trim()
      const isOwnRow = key !== '' && !appliedKeys.includes(key)
      return isOwnRow
    })

    if (!selectedConfig) {
      setAppliedKeys([])
      if (keptRows.length > 0) {
        setEnvRows(keptRows)
      } else {
        setEnvRows([{ key: '', value: '' }])
      }
      return
    }

    const keptKeys = new Set(keptRows.map((row) => row.key.trim()))
    const addedVars = selectedConfig.vars.filter((configVar) => !keptKeys.has(configVar.key))
    const addedRows = addedVars.map((configVar) => ({ key: configVar.key, value: configVar.value }))
    const selectedKeys = selectedConfig.vars.map((configVar) => configVar.key)

    setAppliedKeys(selectedKeys)
    setEnvRows([...keptRows, ...addedRows])
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
      const finishedCheck = await waitForCheck(checkRes.data.id, () => cancelRef.current)
      setCheck(finishedCheck)
      if (finishedCheck.status === 'ready') {
        const found = [
          finishedCheck.detected_dockerfile ? 'Dockerfile' : '',
          finishedCheck.compose_file || '',
        ].filter(Boolean).join(' и ')
        setToast({ severity: 'success', message: found ? `Репозиторий проверен — найден ${found}` : 'Репозиторий проверен' })
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
      mappedPorts = buildMappedPorts(ports, portsRange, busyPorts, effectiveNodeId)
      mappedVolumes = buildMappedVolumes(volumes)
    } catch (validationError) {
      if (validationError instanceof Error) {
        setFormError(validationError.message)
      } else {
        setFormError('Проверьте порты и тома')
      }
      return
    }
    const env = buildEnvMap(envRows)

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

  return (
    <Box className={styles.page}>
      <Box className={styles.header}><Button startIcon={<ArrowLeft size={16} />} onClick={() => navigate('/applications')} className={styles.header__back}>Назад</Button><Typography variant="h5" className={styles.header__title}>Создание приложения</Typography></Box>
      <Card><CardContent className={styles.cardInner}>
        <Grid container spacing={4}>
          <Grid size={{ xs: 12 }}><Typography variant="h6" className={styles.sectionTitle}>1. Основная информация</Typography>
            <Grid container spacing={3}>
              <Grid size={{ xs: 12 }}><TextField fullWidth label="Название приложения" placeholder="например, my-awesome-app" value={appName} onChange={(e) => setAppName(e.target.value)} helperText="Название станет именем контейнера" /></Grid>
              <Grid size={{ xs: 12 }}><TextField fullWidth label="Ссылка на Git-репозиторий" placeholder="https://github.com/org/repo" value={repoUrl} onChange={(e) => setRepoUrl(e.target.value)} helperText="Сервер склонирует проект в папку пользователя и проверит Dockerfile или docker-compose в корне" /></Grid>
              <Grid size={{ xs: 12 }}><TextField fullWidth label="Ветка Git" placeholder="main" value={branch} onChange={(e) => setBranch(e.target.value)} helperText="Обязательное поле — укажите ветку для деплоя (например, main, master, develop)" /></Grid>
            </Grid>
          </Grid>
          <Grid size={{ xs: 12 }}><Divider /></Grid>

          <Grid size={{ xs: 12 }}><Typography variant="h6" className={styles.sectionTitle}>2. Цель деплоя и проверка</Typography>
            <FormControl><RadioGroup row value={deploymentMode} onChange={(e) => setDeploymentMode(e.target.value as 'automatic' | 'manual')}><FormControlLabel value="automatic" control={<Radio />} label="Автоматически — планировщик выберет узел" /><FormControlLabel value="manual" control={<Radio />} label="Выбрать узел вручную" /></RadioGroup></FormControl>
            {onlineNodes.length === 0 && (
              <Alert severity="warning" sx={{ mb: 1.5 }}>Нет доступных серверов — проверьте соединение с backend и heartbeat воркеров.</Alert>
            )}
            {deploymentMode === 'automatic' && recommendedNode && (
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>Рекомендуемый узел: <b>{recommendedNode.name} ({recommendedNode.city})</b></Typography>
            )}
            {deploymentMode === 'automatic' && recommendedNode && (
              <Box className={styles.mapWrap}><ServerMap nodes={nodes} selectedNodeId={recommendedNode.id} /></Box>
            )}
            {deploymentMode === 'manual' && (
              <Stack spacing={1.5} sx={{ mb: 1.5 }}>
                {onlineNodes.map((node) => (
                  <Button
                    key={node.id}
                    variant={selectedNodeId === node.id ? 'contained' : 'outlined'}
                    onClick={() => setSelectedNodeId(node.id)}
                    sx={{ justifyContent: 'flex-start' }}
                  >
                    {node.name} ({node.city}) — свободно CPU {(node.cpu.total - node.cpu.used).toFixed(1)}, RAM {(node.memory.total - node.memory.used).toFixed(1)} ГБ
                  </Button>
                ))}
              </Stack>
            )}
            {deploymentMode === 'manual' && (
              <Box className={styles.mapWrap}><ServerMap nodes={nodes} selectedNodeId={selectedNodeId} onSelectNode={(node) => setSelectedNodeId(node.id)} /></Box>
            )}
            {deploymentMode === 'manual' && selectedNodeId && <Typography variant="body2" className={styles.alert} color="text.secondary">Выбран узел: <b>{getWorkerName(onlineNodes, selectedNodeId)}</b></Typography>}
            {formError && <Alert severity="error" sx={{ mt: 2 }}>{formError}</Alert>}
          </Grid>
          <Grid size={{ xs: 12 }}><Divider /></Grid>

          <Grid size={{ xs: 12 }}><Typography variant="h6" className={styles.sectionTitle}>3. Порты</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
              Хост-порт выбирается из свободных на узле (диапазон {portsRange.min}–{portsRange.max}). Свободно по лимиту: {freeCount}.
            </Typography>
            <Stack spacing={1.5}>
              {ports.map((row, i) => {
                const currentHost = parsePort(row.host)
                const options = currentHost !== null && !suggestions.includes(currentHost)
                  ? [currentHost, ...suggestions]
                  : suggestions
                return (
                  <Stack key={i} direction={{ xs: 'column', sm: 'row' }} spacing={1.5} alignItems={{ sm: 'center' }}>
                    <TextField size="small" label="Порт контейнера" placeholder="80" value={row.container} onChange={(e) => updatePort(i, { container: e.target.value })} sx={{ flex: 1 }} />
                    <FormControl size="small" sx={{ flex: 1 }}>
                      <InputLabel>Хост-порт</InputLabel>
                      <Select value={row.host} label="Хост-порт" onChange={(e) => updatePort(i, { host: e.target.value })}>
                        {row.host === '' && <MenuItem value=""><em>— Выберите —</em></MenuItem>}
                        {options.map((port) => (
                          <MenuItem key={port} value={String(port)}>{port}</MenuItem>
                        ))}
                      </Select>
                    </FormControl>
                    <IconButton size="small" color="error" onClick={() => removePort(i)} disabled={ports.length === 1}><Trash2 size={16} /></IconButton>
                  </Stack>
                )
              })}
              <Stack direction="row" spacing={1.5}>
                <Button variant="outlined" size="small" startIcon={<Plus size={14} />} onClick={() => {
                  const usedHosts = new Set([
                    ...busyPorts.filter((busy) => busy.nodeId === effectiveNodeId).map((busy) => busy.port),
                    ...ports.map((row) => parsePort(row.host)).filter((port): port is number => port !== null),
                    ...suggestions,
                  ])
                  const [next] = randomFreePorts(1, portsRange.min, portsRange.max, usedHosts, Date.now())
                  setPorts([...ports, { host: next !== undefined ? String(next) : '', container: '' }])
                }} sx={{ alignSelf: 'flex-start' }}>Добавить порт</Button>
                <Button variant="outlined" size="small" startIcon={<RefreshCw size={14} />} onClick={regenerateSuggestions} sx={{ alignSelf: 'flex-start' }}>Другие варианты</Button>
              </Stack>
            </Stack>
          </Grid>
          <Grid size={{ xs: 12 }}><Divider /></Grid>

          <Grid size={{ xs: 12 }}><Typography variant="h6" className={styles.sectionTitle}>4. Тома</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>Локальный путь — только внутри вашей папки на сервере, наружу выйти нельзя. Просто имя — именованный том.</Typography>
            <Stack spacing={1.5}>
              {volumes.map((row, i) => (
                <Stack key={i} direction={{ xs: 'column', sm: 'row' }} spacing={1.5} alignItems={{ sm: 'center' }}>
                  <TextField size="small" label="Хост-путь или имя" placeholder="data/db" value={row.hostPath} onChange={(e) => updateVolume(i, { hostPath: e.target.value })} sx={{ flex: 1 }} />
                  <TextField size="small" label="Путь в контейнере" placeholder="/data" value={row.containerPath} onChange={(e) => updateVolume(i, { containerPath: e.target.value })} sx={{ flex: 1 }} />
                  <IconButton size="small" color="error" onClick={() => removeVolume(i)} disabled={volumes.length === 1}><Trash2 size={16} /></IconButton>
                </Stack>
              ))}
              <Button variant="outlined" size="small" startIcon={<Plus size={14} />} onClick={() => setVolumes([...volumes, { hostPath: '', containerPath: '' }])} sx={{ alignSelf: 'flex-start' }}>Добавить том</Button>
            </Stack>
          </Grid>
          <Grid size={{ xs: 12 }}><Divider /></Grid>

          <Grid size={{ xs: 12 }}><Typography variant="h6" className={styles.sectionTitle}>6. Ресурсы и период</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Укажите необходимые ресурсы. Стоимость рассчитается автоматически по тарифам узла.</Typography>
            <Grid container spacing={3}>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField fullWidth type="number" label="CPU (ядер)" value={cpuCores} onChange={(e) => setCpuCores(Math.max(0.1, parseFloat(e.target.value) || 0.1))} inputProps={{ step: 0.1, min: 0.1 }} />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField fullWidth type="number" label="Оперативная память (ГБ)" value={memoryGB} onChange={(e) => setMemoryGB(Math.max(0.1, parseFloat(e.target.value) || 0.1))} inputProps={{ step: 0.1, min: 0.1 }} />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField fullWidth type="number" label="Диск (ГБ)" value={diskGB} onChange={(e) => setDiskGB(Math.max(0.1, parseFloat(e.target.value) || 0.1))} inputProps={{ step: 0.1, min: 0.1 }} />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField fullWidth type="number" label="Период запуска (часов)" value={durationHours} onChange={(e) => setDurationHours(Math.max(1, parseInt(e.target.value) || 1))} inputProps={{ step: 1, min: 1 }} />
              </Grid>
            </Grid>
            <Alert severity="info" sx={{ mt: 2 }}>
              После проверки репозитория вы отправите заявку администратору. После одобрения приложение запустится на {durationHours} ч, затем автоматически остановится. Стоимость спишется с баланса.
              {priceEstimate !== null && <> Ориентировочно: {priceEstimate.toFixed(2)} руб за {serviceCount} конт.</>}
            </Alert>
          </Grid>
          <Grid size={{ xs: 12 }}><Divider /></Grid>

          <Grid size={{ xs: 12 }}><Typography variant="h6" className={styles.sectionTitle}>5. Переменные окружения</Typography>
            <FormControl fullWidth size="small" sx={{ mb: 1.5 }}>
              <InputLabel>Заполнить из конфига</InputLabel>
              <Select value={envConfigId} label="Заполнить из конфига" onChange={(e) => fillFromConfig(e.target.value)}>
                <MenuItem value=""><em>— Не использовать —</em></MenuItem>
                {configs.map((c) => <MenuItem key={c.id} value={c.id}>{c.name} — {c.vars.length} перем.</MenuItem>)}
              </Select>
            </FormControl>
            <Stack spacing={1.5}>
              {envRows.map((row, i) => (
                <Stack key={i} direction={{ xs: 'column', sm: 'row' }} spacing={1.5} alignItems={{ sm: 'center' }}>
                  <TextField size="small" label="Ключ" placeholder="NODE_ENV" value={row.key} onChange={(e) => updateEnv(i, { key: e.target.value })} sx={{ flex: 1 }} />
                  <TextField size="small" label="Значение" placeholder="production" value={row.value} onChange={(e) => updateEnv(i, { value: e.target.value })} sx={{ flex: 1.6 }} />
                  <IconButton size="small" color="error" onClick={() => removeEnv(i)} disabled={envRows.length === 1}><Trash2 size={16} /></IconButton>
                </Stack>
              ))}
              <Button variant="outlined" size="small" startIcon={<Plus size={14} />} onClick={() => setEnvRows([...envRows, { key: '', value: '' }])} sx={{ alignSelf: 'flex-start' }}>Добавить переменную</Button>
            </Stack>
          </Grid>
        </Grid>
      </CardContent></Card>
      <Box className={styles.footerActions}>
        <Button variant="outlined" onClick={() => navigate('/applications')}>Отмена</Button>
        {requestSubmitted ? (
          <Alert severity="success" sx={{ flex: 1, textAlign: 'center' }}>
            Заявка #{requestId?.slice(0, 8)} отправлена на одобрение. Администратор рассмотрит её в ближайшее время.
          </Alert>
        ) : isReady ? (
          <Button variant="contained" color="primary" startIcon={<Save size={18} />} disabled={deploying} onClick={handleDeploy}>
            {deploying ? 'Отправка заявки…' : 'Отправить заявку на запуск'}
          </Button>
        ) : checking ? (
          <>
            <Typography variant="body2" color="text.secondary" sx={{ alignSelf: 'center' }}>
              Ожидание ответа от {getWorkerName(onlineNodes, effectiveNodeId)}… {elapsed}с
            </Typography>
            <Button variant="contained" color="primary" disabled startIcon={<CircularProgress size={18} color="inherit" />}>Проверка…</Button>
            <Button variant="outlined" onClick={handleCancelCheck}>Отмена</Button>
          </>
        ) : (
          <Button variant="contained" color="primary" startIcon={<GitBranch size={18} />} onClick={handleCheck}>Проверить репозиторий</Button>
        )}
      </Box>
      <Dialog open={showBuildModeDialog} onClose={handleBuildModeCancel} maxWidth="md" fullWidth>
        <DialogTitle>Выбор способа сборки и запуска</DialogTitle>
        <DialogContent>
          <Typography variant="body1" paragraph>
            Репозиторий проверен. Выберите способ сборки:
          </Typography>
          <RadioGroup value={pendingBuildMode} onChange={(e) => setPendingBuildMode(e.target.value as 'dockerfile' | 'compose')}>
            {check?.detected_dockerfile && <FormControlLabel value="dockerfile" control={<Radio />} label="Dockerfile — один контейнер" />}
            {!!check?.compose_file && <FormControlLabel value="compose" control={<Radio />} label={`Docker Compose — ${check.compose_file}`} />}
          </RadioGroup>
          {pendingBuildMode === 'compose' && pendingServices.length > 0 && (
            <Box sx={{ mt: 2 }}>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                Выберите сервисы, которые нужно запустить:
              </Typography>
              {pendingServices.map((service) => (
                <FormControlLabel
                  key={service}
                  sx={{ display: 'flex' }}
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
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleBuildModeCancel} variant="outlined">
            Отмена
          </Button>
          <Button onClick={handleBuildModeConfirm} variant="contained" color="primary" disabled={pendingBuildMode === 'compose' && pendingServices.length === 0}>
            Подтвердить
          </Button>
        </DialogActions>
      </Dialog>
      <Snackbar
        open={toast !== null}
        autoHideDuration={6000}
        onClose={() => setToast(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        <Alert severity={toast?.severity || 'info'} onClose={() => setToast(null)} sx={{ width: '100%' }}>
          {toast?.message}
        </Alert>
      </Snackbar>
    </Box>
  )
}
