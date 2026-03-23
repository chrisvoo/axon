import { useCallback, useMemo, useState } from 'react'
import { useAxonSocket } from './hooks/useAxonSocket'
import type { ActivityRow, ActivityStatus, AxonEvent } from './types'
import { API_KEY_STORAGE } from './types'
import { ActivityLog } from './components/ActivityLog'
import { ConnectScreen } from './components/ConnectScreen'
import { MCPConfigPanel } from './components/MCPConfigPanel'
import { SessionStats } from './components/SessionStats'
import { SystemStatus } from './components/SystemStatus'

function mergeRow(
  prev: ActivityRow | undefined,
  patch: Partial<ActivityRow> & { callId: string; tool?: string },
): ActivityRow {
  const base: ActivityRow =
    prev ??
    ({
      callId: patch.callId,
      tool: patch.tool ?? 'unknown',
      detail: '',
      status: 'pending',
      createdAt: Date.now(),
    } satisfies ActivityRow)
  return { ...base, ...patch, callId: patch.callId }
}

function reduceActivity(rows: Map<string, ActivityRow>, event: AxonEvent): Map<string, ActivityRow> {
  const next = new Map(rows)
  const d = event.data ?? {}

  switch (event.type) {
    case 'tool_called': {
      const callId = String(d.call_id ?? '')
      if (!callId) {
        return next
      }
      next.set(
        callId,
        mergeRow(undefined, {
          callId,
          tool: String(d.tool ?? ''),
          detail: String(d.detail ?? ''),
          remoteIp: String(d.remote_ip ?? ''),
          status: 'pending',
          createdAt: Date.now(),
        }),
      )
      return next
    }
    case 'tool_result': {
      const callId = String(d.call_id ?? '')
      if (!callId) {
        return next
      }
      const prev = next.get(callId)
      const ok = d.ok !== false && d.is_error !== true
      const status: ActivityStatus = ok ? 'done' : 'error'
      next.set(
        callId,
        mergeRow(prev, {
          callId,
          tool: String(d.tool ?? prev?.tool ?? ''),
          status,
          durationMs: typeof d.duration_ms === 'number' ? d.duration_ms : prev?.durationMs,
          outputPreview: typeof d.output_preview === 'string' ? d.output_preview : prev?.outputPreview,
          shellStatus: typeof d.shell_status === 'string' ? d.shell_status : prev?.shellStatus,
          exitCode: typeof d.exit_code === 'number' ? d.exit_code : prev?.exitCode,
          processId: typeof d.process_id === 'string' ? d.process_id : prev?.processId,
          isError: Boolean(d.is_error) || ok === false,
        }),
      )
      return next
    }
    case 'input_required': {
      const callId = String(d.call_id ?? '')
      if (!callId) {
        return next
      }
      const prev = next.get(callId)
      next.set(
        callId,
        mergeRow(prev, {
          callId,
          tool: String(d.tool ?? prev?.tool ?? 'shell'),
          status: 'waiting_input',
          processId: String(d.process_id ?? ''),
          lastOutput: String(d.last_output ?? ''),
          hint: String(d.hint ?? ''),
        }),
      )
      return next
    }
    default:
      return next
  }
}

export default function App() {
  const [apiKey, setApiKey] = useState<string | null>(() => sessionStorage.getItem(API_KEY_STORAGE))
  const [wsConnected, setWsConnected] = useState(false)
  const [agentVersion, setAgentVersion] = useState<string | undefined>()
  const [connectedAt, setConnectedAt] = useState<number | null>(null)
  const [systemStats, setSystemStats] = useState<Record<string, unknown> | null>(null)
  const [rows, setRows] = useState<Map<string, ActivityRow>>(() => new Map())
  const [requestCount, setRequestCount] = useState(0)
  const [errorCount, setErrorCount] = useState(0)

  const onEvent = useCallback((event: AxonEvent) => {
    if (event.type === 'connected') {
      const v = event.data?.version
      setAgentVersion(typeof v === 'string' ? v : undefined)
      setConnectedAt(Date.now())
      return
    }
    if (event.type === 'system_stats') {
      setSystemStats((event.data as Record<string, unknown>) ?? null)
      return
    }
    if (event.type === 'tool_called') {
      setRequestCount((c) => c + 1)
    }
    if (event.type === 'tool_result') {
      const d = event.data ?? {}
      if (d.ok === false || d.is_error === true) {
        setErrorCount((c) => c + 1)
      }
    }
    setRows((prev) => reduceActivity(prev, event))
  }, [])

  const onConnectionChange = useCallback((connected: boolean) => {
    setWsConnected(connected)
  }, [])

  useAxonSocket(apiKey, { onEvent, onConnectionChange: onConnectionChange })

  const sortedItems = useMemo(() => {
    return [...rows.values()].sort((a, b) => a.createdAt - b.createdAt)
  }, [rows])

  const handleConnect = (key: string) => {
    sessionStorage.setItem(API_KEY_STORAGE, key)
    setApiKey(key)
    setRows(new Map())
    setRequestCount(0)
    setErrorCount(0)
    setConnectedAt(null)
    setAgentVersion(undefined)
  }

  const handleDisconnect = () => {
    sessionStorage.removeItem(API_KEY_STORAGE)
    setApiKey(null)
  }

  if (!apiKey) {
    return <ConnectScreen onConnect={handleConnect} />
  }

  return (
    <div className="flex min-h-screen flex-col lg:flex-row">
      <aside className="flex w-full flex-col gap-4 border-b border-zinc-800 p-4 lg:w-80 lg:border-b-0 lg:border-r">
        <div>
          <div className="text-lg font-semibold text-white">Axon</div>
          <p className="text-xs text-zinc-500">Remote agent dashboard</p>
        </div>
        <SessionStats
          connected={wsConnected}
          version={agentVersion}
          requests={requestCount}
          errors={errorCount}
          since={connectedAt}
        />
        <SystemStatus stats={systemStats} />
        <MCPConfigPanel apiKey={apiKey} />
        <button
          type="button"
          onClick={handleDisconnect}
          className="rounded-md border border-zinc-700 px-3 py-2 text-xs font-medium text-zinc-300 hover:bg-zinc-800"
        >
          Change API key
        </button>
      </aside>
      <main className="flex min-h-0 flex-1 flex-col p-4">
        <ActivityLog
          items={sortedItems}
          apiKey={apiKey}
          onClear={() => {
            setRows(new Map())
            setRequestCount(0)
            setErrorCount(0)
          }}
        />
      </main>
    </div>
  )
}
