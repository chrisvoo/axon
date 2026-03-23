import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiPostJson } from './api'

describe('apiPostJson', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends a POST with the correct method, headers, and body', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      json: async () => ({ result: 'ok' }),
    } as Response)

    await apiPostJson('/api/test', 'axon_k_test', { foo: 'bar' })

    expect(fetch).toHaveBeenCalledWith('/api/test', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: 'Bearer axon_k_test',
      },
      body: JSON.stringify({ foo: 'bar' }),
    })
  })

  it('returns the parsed JSON body on success', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      json: async () => ({ id: 42 }),
    } as Response)

    const result = await apiPostJson('/api/test', 'k', {})
    expect(result).toEqual({ id: 42 })
  })

  it('throws with the response text when the request fails', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: false,
      text: async () => 'unauthorized',
      statusText: 'Unauthorized',
    } as Response)

    await expect(apiPostJson('/api/test', 'bad-key', {})).rejects.toThrow('unauthorized')
  })

  it('falls back to statusText when the response body is empty', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: false,
      text: async () => '',
      statusText: 'Forbidden',
    } as Response)

    await expect(apiPostJson('/api/test', 'k', {})).rejects.toThrow('Forbidden')
  })
})
