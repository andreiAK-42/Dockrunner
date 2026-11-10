import { realtime } from '../api/realtime'
import type { RealtimeEvent } from '../api/realtime'
import type { CheckStatus } from '../api/containers'
import type { DeploymentRequest } from '../api/deploymentRequests'
import { store } from './index'
import { authCleared, persistUser } from './slices/authSlice'
import { containerEventReceived, fetchAdminContainersThunk, fetchMineContainers, resetContainers, type ContainerEventPayload } from './slices/containersSlice'
import { fetchMineRequests, fetchPendingRequestsThunk, requestEventReceived, resetDeploymentRequests } from './slices/deploymentRequestsSlice'
import { checkReceived, resetChecks } from './slices/checksSlice'
import { resetEnvConfigs } from './slices/envConfigsSlice'
import { resetAgents } from './slices/agentsSlice'
import { resetQuotas } from './slices/quotasSlice'
import { setRealtimeConnected } from './slices/realtimeSlice'

let started = false
let mineRefreshTimer: ReturnType<typeof setTimeout> | null = null

function scheduleMineRefresh() {
  if (mineRefreshTimer !== null) return
  mineRefreshTimer = setTimeout(() => {
    mineRefreshTimer = null
    void store.dispatch(fetchMineContainers())
  }, 300)
}

function handleContainerEvent(event: RealtimeEvent) {
  const data = event.data as ContainerEventPayload
  if (!data?.id) return

  const state = store.getState()
  const user = state.auth.user
  const isAdmin = user?.role === 'admin'
  const isMine = !data.owner_login || data.owner_login === user?.login

  store.dispatch(containerEventReceived(data))

  if (data.removed) return

  if (isMine && !state.containers.mine.some((container) => container.id === data.id)) {
    scheduleMineRefresh()
  }
  if (isAdmin && !state.containers.admin.some((container) => container.id === data.id)) {
    void store.dispatch(fetchAdminContainersThunk())
  }
}

function handleRequestEvent(event: RealtimeEvent) {
  const request = event.data as DeploymentRequest
  if (!request?.id) return

  const state = store.getState()
  const user = state.auth.user
  const isAdmin = user?.role === 'admin'

  store.dispatch(
    requestEventReceived({
      eventType: event.type,
      request,
      currentLogin: user?.login,
      isAdmin,
    }),
  )

  if (event.type === 'deployment_request.created') {
    if (request.ownerLogin === user?.login) {
      void store.dispatch(fetchMineRequests())
    }
    if (isAdmin) {
      void store.dispatch(fetchPendingRequestsThunk())
    }
  }
}

function handleCheckEvent(event: RealtimeEvent) {
  const check = event.data as CheckStatus
  if (!check?.id) return
  store.dispatch(checkReceived(check))
}

function resetUserData() {
  store.dispatch(resetContainers())
  store.dispatch(resetDeploymentRequests())
  store.dispatch(resetEnvConfigs())
  store.dispatch(resetAgents())
  store.dispatch(resetQuotas())
  store.dispatch(resetChecks())
}

/**
 * Wires the singleton WebSocket to the Redux store once, so pages only read
 * state and never subscribe to events themselves.
 */
export function startRealtimeBridge() {
  if (started) return
  started = true

  realtime.subscribeStatus((connected) => {
    store.dispatch(setRealtimeConnected(connected))
  })

  realtime.subscribe('containers', handleContainerEvent)
  realtime.subscribe('deployment_requests', handleRequestEvent)
  realtime.subscribe('checks', handleCheckEvent)

  if (typeof window !== 'undefined') {
    window.addEventListener('dockrunner:auth-cleared', () => {
      store.dispatch(authCleared())
      persistUser(null)
      resetUserData()
    })

    window.addEventListener('dockrunner:auth-updated', () => {
      realtime.connect()
    })
  }
}
