import { useEffect, useRef, useState } from 'react'
import { ApiError, apiGetJson } from '../api'

/**
 * Loads the agent's API key for the setup snippet. Only the operator key can read it.
 * A 401 means the stored operator key is wrong, so the caller can ask for it again.
 */
export function useAgentKey(operatorKey: string | null, onUnauthorized: () => void): string | null {
  const [agentKey, setAgentKey] = useState<string | null>(null)
  const onUnauthorizedRef = useRef(onUnauthorized)
  onUnauthorizedRef.current = onUnauthorized

  useEffect(() => {
    if (!operatorKey) {
      setAgentKey(null)
      return
    }
    let cancelled = false
    apiGetJson<{ api_key: string }>('/api/agent-config', operatorKey)
      .then((cfg) => {
        if (!cancelled) setAgentKey(cfg.api_key)
      })
      .catch((err) => {
        if (!cancelled && err instanceof ApiError && err.status === 401) onUnauthorizedRef.current()
      })
    return () => {
      cancelled = true
    }
  }, [operatorKey])

  return agentKey
}
