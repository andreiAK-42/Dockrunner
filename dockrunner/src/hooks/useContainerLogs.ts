import { useEffect, useState } from 'react'
import { fetchContainerLogs } from '../api/containers'
import { openAuthedWS } from '../api/ws'

const FALLBACK_POLL_MS = 5000

function splitLines(text: string): string[] {
  return text.split('\n').filter((line) => line !== '')
}

// The agent sends plain text for history and JSON {stream, line, time} for
// live lines, so duplicates are easy to avoid: initial tail comes over HTTP.
function liveLine(raw: string): string | null {
  if (!raw.startsWith('{')) return null
  try {
    const parsed = JSON.parse(raw) as { line?: string }
    return typeof parsed.line === 'string' ? parsed.line : null
  } catch {
    return null
  }
}

interface ContainerLogsState {
  lines: string[] | null
  error: string
  live: boolean
}

/**
 * Container logs: HTTP tail for history, then a WebSocket for live lines.
 * If the socket is unavailable the hook falls back to periodic tail polling.
 */
export function useContainerLogs(containerId: string, enabled: boolean, maxLines = 1000): ContainerLogsState {
  const [lines, setLines] = useState<string[] | null>(null)
  const [error, setError] = useState('')
  const [live, setLive] = useState(false)

  useEffect(() => {
    if (!enabled || !containerId) {
      // Сброс логов при отключении — синхронизация с внешним потоком.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setLines(null)
      setError('')
      setLive(false)
      return
    }

    let alive = true
    let socket: WebSocket | null = null
    let pollTimer: ReturnType<typeof setInterval> | null = null
    let historyFromHttp = false
    let gotLive = false

    // Новый контейнер — начинаем с чистого листа.
    setLines(null)
    setError('')
    setLive(false)

    const appendLines = (next: string[]) => {
      if (!alive || next.length === 0) return
      setLines((prev) => {
        const merged = prev ? [...prev, ...next] : next
        return merged.length > maxLines ? merged.slice(-maxLines) : merged
      })
    }

    const replaceWithTail = () => {
      fetchContainerLogs(containerId, maxLines)
        .then((res) => {
          if (!alive) return
          const next = splitLines(res.data)
          setLines(next.length > maxLines ? next.slice(-maxLines) : next)
          historyFromHttp = next.length > 0
          setError('')
        })
        .catch(() => {
          if (alive) setError('Не удалось загрузить логи')
        })
    }

    const startPollingFallback = () => {
      if (pollTimer !== null || !alive) return
      setLive(false)
      pollTimer = setInterval(replaceWithTail, FALLBACK_POLL_MS)
    }

    replaceWithTail()

    const disposeWS = openAuthedWS(`/api/v1/containers/${containerId}/logs/stream`, {
      onOpen: (ws) => {
        socket = ws
        if (alive) setLive(true)

        ws.onmessage = (message) => {
          if (!alive) return
          const raw = String(message.data)

          const parsed = liveLine(raw)
          if (parsed !== null) {
            gotLive = true
            appendLines(splitLines(parsed))
            return
          }

          // Plain text = history. Use it only when the HTTP tail was empty,
          // otherwise the same lines would be shown twice.
          if (!historyFromHttp) {
            appendLines(splitLines(raw))
          }
        }

        ws.onerror = () => {
          if (alive && !gotLive) {
            startPollingFallback()
          }
        }

        ws.onclose = () => {
          if (alive) {
            setLive(false)
            startPollingFallback()
          }
        }
      },
      onClose: () => {
        if (alive) {
          setLive(false)
          startPollingFallback()
        }
      },
    })

    return () => {
      alive = false
      disposeWS()
      if (pollTimer !== null) clearInterval(pollTimer)
      try {
        socket?.close()
      } catch {
        // уже закрыт
      }
    }
  }, [containerId, enabled, maxLines])

  return { lines, error, live }
}
