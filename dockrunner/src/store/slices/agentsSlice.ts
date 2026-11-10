import { createAsyncThunk, createSlice } from '@reduxjs/toolkit'
import { fetchAgents } from '../../api/agents'
import type { ApiAgent } from '../../api/agents'
import type { WorkerNode } from '../../types/workerNode'

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

interface AgentsState {
  nodes: WorkerNode[]
  status: 'idle' | 'loading' | 'succeeded' | 'failed'
  error: string
}

const initialState: AgentsState = {
  nodes: [],
  status: 'idle',
  error: '',
}

export const fetchAgentNodes = createAsyncThunk<WorkerNode[]>('agents/fetch', async () => {
  const res = await fetchAgents()
  return res.data.map(toNode)
})

const agentsSlice = createSlice({
  name: 'agents',
  initialState,
  reducers: {
    resetAgents() {
      return initialState
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchAgentNodes.pending, (state) => {
        state.status = 'loading'
        state.error = ''
      })
      .addCase(fetchAgentNodes.fulfilled, (state, action) => {
        state.status = 'succeeded'
        state.nodes = action.payload
      })
      .addCase(fetchAgentNodes.rejected, (state) => {
        state.status = 'failed'
        state.error = 'Серверы недоступны'
      })
  },
})

export const { resetAgents } = agentsSlice.actions

export default agentsSlice.reducer
