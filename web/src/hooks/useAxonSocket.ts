import { useEffect, useRef } from 'react'
import type { AxonEvent } from '../types'

type Options = {
  onEvent: (event: AxonEvent) => void
  onConnectionChange: (connected: boolean) => void
}

export function useAxonSocket(apiKey: string | null, { onEvent, onConnectionChange }: Options) {
  const onEventRef = useRef(onEvent)
  const onChangeRef = useRef(onConnectionChange)

  onEventRef.current = onEvent
  onChangeRef.current = onConnectionChange

  useEffect(() => {
    if (!apiKey) {
      onChangeRef.current(false)
      return
    }

    let ws: WebSocket | null = null
    let cancelled = false
    let attempt = 0
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined

    const clearReconnect = () => {
      if (reconnectTimer !== undefined) {
        clearTimeout(reconnectTimer)
        reconnectTimer = undefined
      }
    }

    const connect = () => {
      if (cancelled) {
        return
      }

      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = new URL('/ws', `${proto}//${window.location.host}`)
      url.searchParams.set('key', apiKey)

      ws = new WebSocket(url.toString())

      ws.onopen = () => {
        attempt = 0
        onChangeRef.current(true)
      }

      ws.onmessage = (ev) => {
        try {
          const parsed = JSON.parse(ev.data as string) as AxonEvent
          onEventRef.current(parsed)
        } catch {
          /* ignore malformed */
        }
      }

      ws.onerror = () => {
        ws?.close()
      }

      ws.onclose = () => {
        onChangeRef.current(false)
        if (cancelled) {
          return
        }
        const delay = Math.min(30_000, 1000 * 2 ** attempt)
        attempt += 1
        reconnectTimer = window.setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      cancelled = true
      clearReconnect()
      ws?.close()
      onChangeRef.current(false)
    }
  }, [apiKey])
}
