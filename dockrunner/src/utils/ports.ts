// Port utilities for Docker port management

export const MAX_PORT_NUMBER = 65535
export const DEFAULT_PORTS_MIN = 3000
export const DEFAULT_PORTS_MAX = 4000
export const SUGGEST_PORTS_COUNT = 10

/**
 * Mulberry32 PRNG for deterministic random port selection
 */
function mulberry32(seed: number) {
  let state = seed >>> 0
  return () => {
    state |= 0
    state = (state + 0x6d2b79f5) | 0
    let t = Math.imul(state ^ (state >>> 15), 1 | state)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (state >>> 14)) >>> 0) / 4294967296
  }
}

/**
 * Generate random free ports within range, avoiding taken ports
 */
export function randomFreePorts(
  count: number,
  min: number,
  max: number,
  taken: Set<number>,
  seed: number
): number[] {
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

/**
 * Parse string to valid port number
 */
export function parsePort(value: string): number | null {
  const parsed = Number(value)
  const isValidPort = Number.isInteger(parsed) && parsed > 0 && parsed <= MAX_PORT_NUMBER
  if (!isValidPort) {
    return null
  }
  return parsed
}

/**
 * Find next free host port in range
 */
export function nextFreeHostPort(min: number, max: number, used: Set<number>): number {
  for (let port = min; port <= max; port++) {
    if (!used.has(port)) {
      return port
    }
  }
  return 0
}

export interface PortRow {
  host: string
  container: string
}

export interface MappedPort {
  host_port: number
  container_port: number
}

export interface PortInfo {
  nodeId: string
  port: number
  appName: string
}

/**
 * Build mapped ports from form rows, validating against busy ports and range
 */
export function buildMappedPorts(
  portRows: PortRow[],
  portsRange: { min: number; max: number },
  busyPorts: PortInfo[],
  nodeId: string
): MappedPort[] {
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