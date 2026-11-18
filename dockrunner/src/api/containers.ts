import { apiRequest } from './client'

export interface PortSpec {
  host_port: number
  container_port: number
}

export interface VolumeSpec {
  name?: string
  host_path?: string
  container_path: string
}

export interface DetectedPort {
  container_port: number
  protocol: string
  source: string
  service?: string
}

export interface CheckStatus {
  id: string
  app_name: string
  repo_url: string
  node_id: string
  status: string
  detected_dockerfile: boolean
  compose_file: string
  services: string[]
  branches?: string[]
  detected_env_vars?: Record<string, string>
  detected_ports?: DetectedPort[]
  error: string
}

export interface PortMapping {
  host_port: number
  container_port: number
}

export interface MineContainer {
  id: string
  app_id: string
  app_name: string
  owner_login: string
  image: string
  status: string
  node_id: string
  node_ip?: string
  ports: PortMapping[]
  cpu_cores?: number
  memory_gb?: number
  disk_gb?: number
  expires_at?: string | null
  updated_at: string
}

export function fetchMine() {
  return apiRequest<{ data: MineContainer[] }>('/api/v1/containers/mine')
}

export function fetchContainerLogs(id: string, tail = 10000) {
  return apiRequest<{ data: string }>(`/api/v1/containers/${id}/logs?tail=${tail}`)
}

export interface ContainerStats {
  cpu_percent: number
  mem_mb: number
}

export function fetchContainerStats(id: string) {
  return apiRequest<{ data: ContainerStats }>(`/api/v1/containers/${id}/stats`)
}

export interface ContainerSpecPort {
  host: number
  container: number
}

export interface ContainerSpecVolume {
  name?: string
  host_path?: string
  container_path: string
  read_only?: boolean
}

export interface ContainerSpec {
  id: string
  name: string
  image: string
  status: string
  ports: ContainerSpecPort[]
  volumes: ContainerSpecVolume[]
  env: Record<string, string> | null
  entrypoint?: string[]
  cmd?: string[]
  restart_policy?: string
  network?: string
  working_dir?: string
  user?: string
}

// Разные источники spec отдают порты/тома по-разному: живой агент —
// {host, container}, а фолбэк бэкенда из БД — {host_port, container_port}.
// Приводим всё к единому виду, чтобы UI не показывал "undefined → undefined".
interface RawSpecPort {
  host?: number
  container?: number
  host_port?: number
  container_port?: number
}

interface RawSpecVolume {
  name?: string | null
  host_path?: string | null
  container_path?: string | null
  read_only?: boolean
}

interface RawContainerSpec extends Omit<ContainerSpec, 'ports' | 'volumes'> {
  ports?: RawSpecPort[] | null
  volumes?: RawSpecVolume[] | null
}

function normalizeSpecPorts(raw: RawSpecPort[] | null | undefined): ContainerSpecPort[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((port) => ({
      host: port.host ?? port.host_port ?? 0,
      container: port.container ?? port.container_port ?? 0,
    }))
    .filter((port) => port.host > 0 && port.container > 0)
}

function normalizeSpecVolumes(raw: RawSpecVolume[] | null | undefined): ContainerSpecVolume[] {
  if (!Array.isArray(raw)) return []
  return raw
    .filter((volume) => typeof volume.container_path === 'string' && volume.container_path !== '')
    .map((volume) => ({
      name: volume.name ?? undefined,
      host_path: volume.host_path ?? undefined,
      container_path: volume.container_path as string,
      read_only: volume.read_only,
    }))
}

function normalizeContainerSpec(raw: RawContainerSpec): ContainerSpec {
  return {
    ...raw,
    ports: normalizeSpecPorts(raw.ports),
    volumes: normalizeSpecVolumes(raw.volumes),
  }
}

export async function fetchContainerSpec(id: string) {
  const res = await apiRequest<{ data: RawContainerSpec }>(`/api/v1/containers/${id}/spec`)
  return { ...res, data: normalizeContainerSpec(res.data) }
}

export function checkRepository(payload: { app_name: string; repo_url: string; node_id: string; branch: string }) {
  return apiRequest<{ data: CheckStatus }>('/api/v1/containers/check', {
    method: 'POST',
    body: payload,
  })
}

export function fetchCheck(id: string) {
  return apiRequest<{ data: CheckStatus }>(`/api/v1/containers/check/${id}`)
}

export function runContainer(payload: {
  app_name: string
  repo_url?: string
  image?: string
  container_name?: string
  node_id: string
  ports: PortSpec[]
  volumes: VolumeSpec[]
  env: Record<string, string>
  entrypoint?: string[]
  cmd?: string[]
  restart_policy?: string
  network?: string
  working_dir?: string
  user?: string
  build_mode?: 'dockerfile' | 'compose'
  branch?: string
  services?: string[]
}) {
  return apiRequest<{ data: { deployment_id: string } }>('/api/v1/containers/run', {
    method: 'POST',
    body: payload,
  })
}

export function refreshContainerByApp(app_name: string) {
  return apiRequest<{ data: { deployment_id: string } }>('/api/v1/containers/refresh', {
    method: 'POST',
    body: { app_name },
  })
}

export function removeContainer(id: string) {
  return apiRequest<{ data: { message: string } }>(`/api/v1/containers/${id}`, {
    method: 'DELETE',
  })
}

export function startContainer(id: string) {
  return apiRequest<{ data: { message: string } }>(`/api/v1/containers/${id}/start`, {
    method: 'POST',
  })
}

export function stopContainer(id: string) {
  return apiRequest<{ data: { message: string } }>(`/api/v1/containers/${id}/stop`, {
    method: 'POST',
  })
}

export function fetchBuildLog(id: string) {
  return apiRequest<{ data: string }>(`/api/v1/containers/${id}/build-log`)
}

function tokenizeDockerCommand(command: string): string[] {
  const tokens: string[] = []
  let token = '', quote = '', escaped = false, active = false
  for (const ch of command.trim()) {
    if (escaped) { token += ch; escaped = false; active = true; continue }
    if (ch === '\\' && quote !== "'") { escaped = true; active = true; continue }
    if (quote) { if (ch === quote) quote = ''; else token += ch; active = true; continue }
    if (ch === '"' || ch === "'") { quote = ch; active = true; continue }
    if (/\s/.test(ch)) { if (active) { tokens.push(token); token = ''; active = false }; continue }
    token += ch; active = true
  }
  if (escaped || quote) throw new Error('Незакрытая кавычка или escape в команде')
  if (active) tokens.push(token)
  return tokens
}

export interface ParsedDockerRunCommand {
  name: string
  image: string
  env: Record<string, string>
  ports: { host: number; container: number }[] | null
  volumes: { name: string; host_path: string; container_path: string }[] | null
  restart_policy: string
  entrypoint: string[]
  cmd: string[]
  working_dir: string
  user: string
  network?: string
}

export function parseDockerRunCommand(command: string): ParsedDockerRunCommand {
  const args = tokenizeDockerCommand(command)
  if (args[0] !== 'docker' || args[1] !== 'run') throw new Error('Ожидается команда docker run')
  const result: ParsedDockerRunCommand = { name: '', image: '', env: {}, ports: [], volumes: [], restart_policy: 'no', entrypoint: [], cmd: [], working_dir: '', user: '' }
  let i = 2
  while (i < args.length) {
    const arg = args[i]
    if (arg === '-d' || arg === '--detach') { i++; continue }
    const takesValue = ['--name','--env','-e','--publish','-p','--volume','-v','--restart','--entrypoint','--network','-w','--workdir','-u','--user']
    if (takesValue.includes(arg)) {
      const value = args[i+1]; if (value === undefined) throw new Error(`Для ${arg} не задано значение`); i += 2
      if (arg === '--name') result.name = value
      else if (arg === '--env' || arg === '-e') { const at = value.indexOf('='); if (at <= 0) throw new Error(`Некорректная переменная окружения: ${value}`); result.env[value.slice(0,at)] = value.slice(at+1) }
      else if (arg === '-p' || arg === '--publish') { const bits = value.split(':'); const host = Number(bits[bits.length-2]), container = Number(bits[bits.length-1]); if (!host || !container) throw new Error(`Некорректный порт: ${value}`); (result.ports ??= []).push({host,container}) }
      else if (arg === '-v' || arg === '--volume') { const bits = value.split(':'); if (bits.length < 2) throw new Error(`Некорректный том: ${value}`); (result.volumes ??= []).push({name: bits.slice(0,-1).join(':'),host_path: bits.slice(0,-1).join(':'),container_path: bits[bits.length-1]}) }
      else if (arg === '--restart') result.restart_policy = value
      else if (arg === '--entrypoint') result.entrypoint = [value]
      else if (arg === '--network') result.network = value
      else if (arg === '-w' || arg === '--workdir') result.working_dir = value
      else if (arg === '-u' || arg === '--user') result.user = value
      continue
    }
    if (arg.startsWith('-')) throw new Error(`Параметр ${arg} пока нельзя применить через редактор. Поддерживаются --name, -e, -p, -v, --restart, --entrypoint, -w и -u.`)
    result.image = arg
    result.cmd = args.slice(i+1)
    break
  }
  if (!result.image) throw new Error('В команде не указан образ контейнера')
  if (!result.name) throw new Error('В команде не указано имя контейнера (--name)')
  return result
}

export function applyDockerCommand(id: string, command: string) {
  return apiRequest<{ ok: boolean; container_id: string; status: string }>(`/api/v1/containers/${id}/recreate`, {
    method: 'POST', body: parseDockerRunCommand(command),
  })
}

export function requestContainerUpdate(id: string, command: string) {
  return apiRequest<{ message: string; request_id: string }>(`/api/v1/containers/${id}/request-update`, {
    method: 'POST', body: parseDockerRunCommand(command),
  })
}
