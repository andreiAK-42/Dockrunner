import { useEffect, useState } from 'react'
import { fetchMine } from '../api/containers'
import type { MineContainer } from '../api/containers'
import { fetchQuotas } from '../api/quotas'

export interface PortInfo {
  port: number
  used: boolean
  containerId: string
  containerName: string
  appName: string
  nodeId: string
}

function collectUsedPorts(containers: MineContainer[]): PortInfo[] {
  const usedPorts: PortInfo[] = []

  for (const container of containers) {
    const ports = container.ports || []
    for (const portMapping of ports) {
      usedPorts.push({
        port: portMapping.host_port,
        used: true,
        containerId: container.id,
        containerName: container.app_name,
        appName: container.app_name,
        nodeId: container.node_id,
      })
    }
  }

  usedPorts.sort((first, second) => first.port - second.port)
  return usedPorts
}

export function usePorts() {
  const [used, setUsed] = useState<PortInfo[]>([])
  const [limit, setLimit] = useState(0)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.allSettled([fetchMine(), fetchQuotas()]).then(([mineResult, quotasResult]) => {
      if (mineResult.status === 'fulfilled') {
        const usedPorts = collectUsedPorts(mineResult.value.data)
        setUsed(usedPorts)
      }

      if (quotasResult.status === 'fulfilled') {
        setLimit(quotasResult.value.data.ports_limit)
      }

      setLoading(false)
    })
  }, [])

  const usedCount = used.length
  const freeCount = Math.max(0, limit - usedCount)
  const isFull = limit > 0 && usedCount >= limit

  return {
    ports: used,
    usedCount,
    freeCount,
    limit,
    loading,
    isFull,
  }
}
