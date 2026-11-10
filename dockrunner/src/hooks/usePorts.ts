import { useEffect } from 'react'
import type { MineContainer } from '../api/containers'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchMineContainers } from '../store/slices/containersSlice'
import { fetchQuotasThunk } from '../store/slices/quotasSlice'

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
  const dispatch = useAppDispatch()
  const containers = useAppSelector((state) => state.containers.mine)
  const containersStatus = useAppSelector((state) => state.containers.mineStatus)
  const quotas = useAppSelector((state) => state.quotas)

  useEffect(() => {
    if (containersStatus === 'idle') {
      void dispatch(fetchMineContainers())
    }
    if (quotas.status === 'idle') {
      void dispatch(fetchQuotasThunk())
    }
  }, [containersStatus, quotas.status, dispatch])

  const used = collectUsedPorts(containers)
  const limit = quotas.data?.ports_limit ?? 0

  const usedCount = used.length
  const freeCount = Math.max(0, limit - usedCount)
  const isFull = limit > 0 && usedCount >= limit

  return {
    ports: used,
    usedCount,
    freeCount,
    limit,
    loading:
      containersStatus === 'idle' ||
      containersStatus === 'loading' ||
      quotas.status === 'idle' ||
      quotas.status === 'loading',
    isFull,
  }
}
