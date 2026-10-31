import { useEffect, useRef } from 'react'
import type { AxonEvent } from '../types'

// Keep these in step with internal/hub/hub.go.
export const WS_SUBPROTOCOL = 'axon.v1'
export const WS_KEY_PROTOCOL_PREFIX = 'axon.key.'
export const WS_CLOSE_UNAUTHORIZED = 4401

type Options = {
  onEvent: (event: AxonEvent) => void
  onConnectionChange: (connected: boolean) => void
  /** The server closed the connection because the key is missing or wrong. */
  onUnauthorized?: () => void
}

export function useAxonSocket(apiKey: string | null, { onEvent, onConnectionChange, onUnauthorized }: Options) {
  const onEventRef = useRef(onEvent)
  const onChangeRef = useRef(onConnectionChange)
  const onUnauthorizedRef = useRef(onUnauthorized)

  onEventRef.current = onEvent
  onChangeRef.current = onConnectionChange
  onUnauthorizedRef.current = onUnauthorized

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

      // The key goes in the Sec-WebSocket-Protocol header, not in the URL: a URL
      // shows in proxy logs and browser history.
      ws = new WebSocket(url.toString(), [WS_SUBPROTOCOL, WS_KEY_PROTOCOL_PREFIX + apiKey])

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

      ws.onclose = (ev) => {
        onChangeRef.current(false)
        if (cancelled) {
          return
        }
        if (ev.code === WS_CLOSE_UNAUTHORIZED) {
          // A wrong key stays wrong: ask for a new one instead of retrying.
          cancelled = true
          onUnauthorizedRef.current?.()
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
