import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../api'
import { useAgentKey } from './useAgentKey'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return { ...actual, apiGetJson: vi.fn() }
})

describe('useAgentKey', () => {
  beforeEach(() => {
    vi.mocked(api.apiGetJson).mockReset()
  })

  it('loads the agent key with the operator key', async () => {
    vi.mocked(api.apiGetJson).mockResolvedValue({ api_key: 'axon_k_agent' })
    const { result } = renderHook(() => useAgentKey('axon_o_op', vi.fn()))
    await waitFor(() => expect(result.current).toBe('axon_k_agent'))
    expect(api.apiGetJson).toHaveBeenCalledWith('/api/agent-config', 'axon_o_op')
  })

  it('does not call the server without an operator key', () => {
    const { result } = renderHook(() => useAgentKey(null, vi.fn()))
    expect(result.current).toBeNull()
    expect(api.apiGetJson).not.toHaveBeenCalled()
  })

  it('asks for the operator key again on a 401', async () => {
    vi.mocked(api.apiGetJson).mockImplementation(() => Promise.reject(new api.ApiError('unauthorized', 401)))
    const onUnauthorized = vi.fn()
    renderHook(() => useAgentKey('axon_k_old_agent_key', onUnauthorized))
    await waitFor(() => expect(onUnauthorized).toHaveBeenCalledTimes(1))
  })

  it('keeps the key empty and stays connected on another error', async () => {
    vi.mocked(api.apiGetJson).mockImplementation(() => Promise.reject(new api.ApiError('boom', 500)))
    const onUnauthorized = vi.fn()
    const { result } = renderHook(() => useAgentKey('axon_o_op', onUnauthorized))
    await waitFor(() => expect(api.apiGetJson).toHaveBeenCalled())
    expect(result.current).toBeNull()
    expect(onUnauthorized).not.toHaveBeenCalled()
  })
})
