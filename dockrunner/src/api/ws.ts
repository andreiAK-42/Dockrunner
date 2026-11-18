import { apiBase, getToken } from './client'

interface AuthedWSHandlers {
  /** Вызывается после успешной аутентификации (auth-сообщение отправлено). */
  onOpen: (socket: WebSocket) => void
  /** Вызывается при закрытии соединения без внутреннего переподключения. */
  onClose?: () => void
}

/**
 * Открывает WebSocket к нашему API с аутентификацией. Сначала пробуем
 * подключиться без токена в URL и послать {"action":"auth","token":...}
 * после connect — чтобы JWT не попадал в логи прокси и history.
 * Если сервер закрыл соединение до open (не умеет auth-сообщение),
 * прозрачно переподключаемся классически с ?token=.
 */
export function openAuthedWS(path: string, handlers: AuthedWSHandlers): () => void {
  const wsBase = apiBase().replace(/^http/, 'ws')
  const token = getToken()

  let socket: WebSocket | null = null
  let disposed = false
  let retriedWithQuery = false

  const connect = (withQueryToken: boolean) => {
    if (disposed) return
    const url = withQueryToken && token ? `${wsBase}${path}?token=${encodeURIComponent(token)}` : `${wsBase}${path}`
    try {
      socket = new WebSocket(url)
    } catch {
      return
    }

    let opened = false

    socket.onopen = () => {
      if (disposed || socket === null) return
      opened = true
      if (!withQueryToken && token) {
        socket.send(JSON.stringify({ action: 'auth', token }))
      }
      handlers.onOpen(socket)
    }

    socket.onerror = () => {
      if (disposed) return
      try {
        socket?.close()
      } catch {
        // уже закрыт
      }
    }

    socket.onclose = () => {
      if (disposed) return
      // Закрыто до open — сервер ждёт токен в query, повторяем с ?token=.
      if (!opened && !withQueryToken && !retriedWithQuery) {
        retriedWithQuery = true
        connect(true)
        return
      }
      handlers.onClose?.()
    }
  }

  connect(false)

  return () => {
    disposed = true
    try {
      socket?.close()
    } catch {
      // уже закрыт
    }
  }
}
