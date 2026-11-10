import { useState, useCallback, useMemo } from 'react'
import type { PortInfo } from '../../../hooks/usePorts'
import {
  randomFreePorts,
  parsePort,
  nextFreeHostPort,
  type PortRow,
  type MappedPort,
  DEFAULT_PORTS_MIN,
  DEFAULT_PORTS_MAX,
  SUGGEST_PORTS_COUNT,
} from '../../../utils/ports'

export interface UsePortManagerOptions {
  portsRange: { min: number; max: number }
  busyPorts: PortInfo[]
  effectiveNodeId: string
  freeCount: number
}

export interface UsePortManagerReturn {
  ports: PortRow[]
  setPorts: React.Dispatch<React.SetStateAction<PortRow[]>>
  suggestions: number[]
  regenerateSuggestions: () => void
  updatePort: (index: number, patch: Partial<PortRow>) => void
  removePort: (index: number) => void
  buildMappedPorts: () => MappedPort[]
}

/**
 * Hook for managing port form state and suggestions
 */
export function usePortManager(options: UsePortManagerOptions): UsePortManagerReturn {
  const { portsRange, busyPorts, effectiveNodeId, freeCount } = options

  const [ports, setPorts] = useState<PortRow[]>([{ host: '', container: '80' }])
  const [suggestionSeed, setSuggestionSeed] = useState(0)

  // Generate free port suggestions
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

  const regenerateSuggestions = useCallback(() => {
    setSuggestionSeed((seed) => seed + 1)
  }, [])

  const updatePort = useCallback((index: number, patch: Partial<PortRow>) => {
    setPorts((prev) => prev.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }, [])

  const removePort = useCallback((index: number) => {
    setPorts((prev) => prev.filter((_, i) => i !== index))
  }, [])

  const buildMappedPortsResult = useCallback((): MappedPort[] => {
    const mappedPorts: MappedPort[] = []

    for (const row of ports) {
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

      const busyPort = busyPorts.find((busy) => busy.nodeId === effectiveNodeId && busy.port === host)
      if (busyPort) {
        throw new Error(`Порт ${host} уже занят приложением «${busyPort.appName}» на выбранном узле`)
      }

      mappedPorts.push({ host_port: host, container_port: container })
    }

    return mappedPorts
  }, [ports, portsRange, busyPorts, effectiveNodeId])

  return {
    ports,
    setPorts,
    suggestions,
    regenerateSuggestions,
    updatePort,
    removePort,
    buildMappedPorts: buildMappedPortsResult,
  }
}