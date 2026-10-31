import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  useAxonSocket,
  WS_CLOSE_UNAUTHORIZED,
  WS_KEY_PROTOCOL_PREFIX,
  WS_SUBPROTOCOL,
} from './useAxonSocket'

class FakeSocket {
  static instances: FakeSocket[] = []
  url: string
  protocols: string[]
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  onclose: ((ev: { code: number }) => void) | null = null
  closed = false

  constructor(url: string, protocols: string[]) {
    this.url = url
    this.protocols = protocols
    FakeSocket.instances.push(this)
  }

  close() {
    this.closed = true
  }
}

const handlers = () => ({ onEvent: vi.fn(), onConnectionChange: vi.fn(), onUnauthorized: vi.fn() })

describe('useAxonSocket', () => {
  beforeEach(() => {
    FakeSocket.instances = []
    vi.stubGlobal('WebSocket', FakeSocket)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('sends the key in the subprotocol and never in the URL', () => {
    renderHook(() => useAxonSocket('axon_k_secret', handlers()))

    const ws = FakeSocket.instances[0]
    expect(ws.protocols).toEqual([WS_SUBPROTOCOL, WS_KEY_PROTOCOL_PREFIX + 'axon_k_secret'])
    expect(ws.url).toMatch(/\/ws$/)
    expect(ws.url).not.toContain('axon_k_secret')
    expect(ws.url).not.toContain('key=')
  })

  it('opens no socket without a key', () => {
    const h = handlers()
    renderHook(() => useAxonSocket(null, h))
    expect(FakeSocket.instances).toHaveLength(0)
    expect(h.onConnectionChange).toHaveBeenCalledWith(false)
  })

  it('forwards parsed events and ignores malformed ones', () => {
    const h = handlers()
    renderHook(() => useAxonSocket('k', h))
    const ws = FakeSocket.instances[0]

    ws.onmessage?.({ data: JSON.stringify({ type: 'connected', data: { version: '2.0.0' } }) })
    ws.onmessage?.({ data: 'not json' })

    expect(h.onEvent).toHaveBeenCalledTimes(1)
    expect(h.onEvent).toHaveBeenCalledWith({ type: 'connected', data: { version: '2.0.0' } })
  })

  it('asks for a new key and stops retrying when the server closes with 4401', () => {
    vi.useFakeTimers()
    const h = handlers()
    renderHook(() => useAxonSocket('wrong', h))

    FakeSocket.instances[0].onclose?.({ code: WS_CLOSE_UNAUTHORIZED })
    vi.advanceTimersByTime(60_000)

    expect(h.onUnauthorized).toHaveBeenCalledTimes(1)
    expect(FakeSocket.instances).toHaveLength(1)
  })

  it('reconnects after any other close', () => {
    vi.useFakeTimers()
    const h = handlers()
    renderHook(() => useAxonSocket('k', h))

    FakeSocket.instances[0].onclose?.({ code: 1006 })
    vi.advanceTimersByTime(1000)

    expect(h.onUnauthorized).not.toHaveBeenCalled()
    expect(FakeSocket.instances).toHaveLength(2)
  })
})
