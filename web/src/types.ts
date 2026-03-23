export type AxonEventType =
  | 'connected'
  | 'tool_called'
  | 'tool_result'
  | 'input_required'
  | 'system_stats'
  | 'dashboard_input'
  | 'dashboard_cancel'

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

export const API_KEY_STORAGE = 'axon_api_key'
