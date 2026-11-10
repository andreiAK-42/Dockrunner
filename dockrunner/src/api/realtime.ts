import { apiBase, getToken } from './client'

export type RealtimeTopic = 'containers' | 'checks' | 'deployment_requests'

export interface RealtimeEvent<T = unknown> {
  type: string
  topic: string
  data: T
}

type EventHandler = (event: RealtimeEvent) => void
type StatusHandler = (connected: boolean) => void

const RECONNECT_BASE_MS = 1000
const RECONNECT_MAX_MS = 30000

function wsBase(): string {
  return apiBase().replace(/^http/, 'ws')
}

/**
 * One WebSocket per browser session. Topics are multiplexed on it; the server
 * additionally filters events by owner and role, so subscribing to everything
 * from the dashboard is safe.
 */
class RealtimeClient {
  private socket: WebSocket | null = null
  private handlers = new Map<string, Set<EventHandler>>()
  private statusHandlers = new Set<StatusHandler>()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private attempts = 0
  private stopped = false
  private connected = false

  constructor() {
    if (typeof window !== 'undefined') {
      window.addEventListener('dockrunner:auth-cleared', () => this.stop())
      window.addEventListener('dockrunner:auth-updated', () => {
        if (!this.connected) this.ensureConnected()
      })
    }
  }

  isConnected() {
    return this.connected
  }

  subscribe(topic: RealtimeTopic, handler: EventHandler): () => void {
    let topicHandlers = this.handlers.get(topic)
    if (!topicHandlers) {
      topicHandlers = new Set()
      this.handlers.set(topic, topicHandlers)
    }
    topicHandlers.add(handler)

    this.stopped = false
    this.ensureConnected()
    this.send({ action: 'subscribe', topics: [topic] })

    return () => {
      topicHandlers?.delete(handler)
      if (topicHandlers && topicHandlers.size === 0) {
        this.handlers.delete(topic)
        this.send({ action: 'unsubscribe', topics: [topic] })
      }
    }
  }

  subscribeStatus(handler: StatusHandler): () => void {
    this.statusHandlers.add(handler)
    handler(this.connected)

    if (this.handlers.size > 0) {
      this.stopped = false
      this.ensureConnected()
    }

    return () => {
      this.statusHandlers.delete(handler)
    }
  }

  disconnect() {
    this.stop()
  }

  /** Re-open the socket after login (stop() is called on logout). */
  connect() {
    this.stopped = false
    this.ensureConnected()
  }

  private stop() {
    this.stopped = true
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    const socket = this.socket
    this.socket = null
    if (socket) {
      socket.onopen = null
      socket.onmessage = null
      socket.onclose = null
      socket.onerror = null
      try {
        socket.close()
      } catch {
        // уже закрыт
      }
    }
    this.setConnected(false)
  }

  private ensureConnected() {
    if (this.stopped) return
    if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
      return
    }

    const token = getToken()
    if (!token) return

    let socket: WebSocket
    try {
      socket = new WebSocket(`${wsBase()}/api/v1/ws?token=${encodeURIComponent(token)}`)
    } catch {
      this.scheduleReconnect()
      return
    }
    this.socket = socket

    socket.onopen = () => {
      if (this.socket !== socket) return
      this.attempts = 0
      this.setConnected(true)
      for (const topic of this.handlers.keys()) {
        this.send({ action: 'subscribe', topics: [topic] })
      }
    }

    socket.onmessage = (message) => {
      if (this.socket !== socket) return
      let event: RealtimeEvent
      try {
        event = JSON.parse(String(message.data)) as RealtimeEvent
      } catch {
        return
      }
      if (!event || !event.topic) return

      const handlers = this.handlers.get(event.topic)
      if (!handlers) return
      for (const handler of handlers) {
        try {
          handler(event)
        } catch {
          // один обработчик не должен ломать остальные
        }
      }
    }

    socket.onclose = () => {
      if (this.socket !== socket) return
      this.socket = null
      this.setConnected(false)
      this.scheduleReconnect()
    }

    socket.onerror = () => {
      try {
        socket.close()
      } catch {
        // уже закрыт
      }
    }
  }

  private scheduleReconnect() {
    if (this.stopped || this.reconnectTimer !== null) return
    const delay = Math.min(RECONNECT_MAX_MS, RECONNECT_BASE_MS * 2 ** this.attempts)
    this.attempts += 1
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.ensureConnected()
    }, delay)
  }

  private send(payload: unknown) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) return
    try {
      this.socket.send(JSON.stringify(payload))
    } catch {
      // соединение закрывается — переподключимся
    }
  }

  private setConnected(next: boolean) {
    if (this.connected === next) return
    this.connected = next
    for (const handler of this.statusHandlers) {
      try {
        handler(next)
      } catch {
        // status listener не должен ломать клиент
      }
    }
  }
}

export const realtime = new RealtimeClient()
