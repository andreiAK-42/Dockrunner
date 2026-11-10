import { useEffect, useRef } from 'react'
import { realtime } from '../api/realtime'
import type { RealtimeEvent, RealtimeTopic } from '../api/realtime'
import { useAppSelector } from '../store/hooks'

/**
 * Subscribes to one realtime topic for the lifetime of the component.
 * The handler always sees the latest props/state through a ref.
 */
export function useRealtime(topic: RealtimeTopic, handler: (event: RealtimeEvent) => void) {
  const handlerRef = useRef(handler)

  useEffect(() => {
    handlerRef.current = handler
  }, [handler])

  useEffect(() => {
    return realtime.subscribe(topic, (event) => handlerRef.current(event))
  }, [topic])
}

/** WebSocket connection state, kept in the Redux store. */
export function useRealtimeStatus(): boolean {
  return useAppSelector((state) => state.realtime.connected)
}
