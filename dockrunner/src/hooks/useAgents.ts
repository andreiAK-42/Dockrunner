import { useEffect, useState } from 'react'
import { fetchAgents } from '../api/agents'
import type { ApiAgent } from '../api/agents'
import type { WorkerNode } from '../types/workerNode'

const ONLINE_THRESHOLD_MS = 90000

function round1(value: number) {
  return Math.round(value * 10) / 10
}

function buildCpuUsage(agent: ApiAgent) {
  const total = agent.cpu_total > 0 ? agent.cpu_total : 0

  if (total > 0) {
    const used = (agent.cpu_percent / 100) * total
    return { used: round1(used), total }
  }

  return { used: round1(agent.cpu_percent), total }
}

function toNode(agent: ApiAgent): WorkerNode {
  const lastSeenAgo = Date.now() - new Date(agent.last_seen).getTime()
  const online = lastSeenAgo < ONLINE_THRESHOLD_MS
  const cpu = buildCpuUsage(agent)

  return {
    id: agent.id,
    name: agent.name,
    city: agent.city || '',
    ip: agent.ip || '',
    coordinates: [agent.lng, agent.lat] as [number, number],
    status: online ? 'online' : 'offline',
    cpu: { ...cpu, mhz: agent.cpu_mhz },
    memory: { used: round1(agent.mem_used_gb), total: round1(agent.mem_total_gb), type: agent.mem_type, speedMhz: agent.mem_speed_mhz },
    disk: { used: round1(agent.disk_used_gb), total: round1(agent.disk_total_gb), type: agent.disk_type },
    containers: agent.containers,
    net: { down: agent.net_down_mbps || 0, up: agent.net_up_mbps || 0 },
    lastSeen: agent.last_seen,
    healthUrl: agent.api_url,
    description: agent.description,
  }
}

export function useAgents() {
  const [nodes, setNodes] = useState<WorkerNode[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetchAgents()
      .then((res) => {
        const workerNodes = res.data.map(toNode)
        setNodes(workerNodes)
        setError('')
      })
      .catch(() => setError('Серверы недоступны'))
      .finally(() => setLoading(false))
  }, [])

  return { nodes, loading, error }
}
