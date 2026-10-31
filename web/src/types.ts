export type AxonEventType =
  | 'connected'
  | 'tool_called'
  | 'tool_result'
  | 'input_required'
  | 'system_stats'
  | 'privileged_commands'

export type AxonEvent = {
  type: AxonEventType
  data?: Record<string, unknown>
}

export type ActivityStatus = 'pending' | 'waiting_input' | 'done' | 'error'

export type ActivityRow = {
  callId: string
  tool: string
  detail: string
  remoteIp?: string
  status: ActivityStatus
  durationMs?: number
  outputPreview?: string
  shellStatus?: string
  exitCode?: number
  processId?: string
  hint?: string
  lastOutput?: string
  isError?: boolean
  createdAt: number
}

// The dashboard and the agent use the same API key. The dashboard keeps it for the tab only.
export const API_KEY_STORAGE = 'axon_api_key'
