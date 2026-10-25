import { describe, expect, it } from 'vitest'
import { reduceActivity } from './App'
import type { ActivityRow, AxonEvent } from './types'

function run(events: AxonEvent[]): Map<string, ActivityRow> {
  return events.reduce(reduceActivity, new Map<string, ActivityRow>())
}

const called: AxonEvent = { type: 'tool_called', data: { call_id: 'c1', tool: 'shell', detail: 'passwd' } }

describe('reduceActivity', () => {
  it('keeps a command that waits for input in the waiting state', () => {
    // Same order as the server: input_required first, then tool_result.
    const rows = run([
      called,
      { type: 'input_required', data: { call_id: 'c1', process_id: 'p1', hint: 'h' } },
      { type: 'tool_result', data: { call_id: 'c1', tool: 'shell', ok: true, shell_status: 'input_required', process_id: 'p1' } },
    ])
    const row = rows.get('c1')
    expect(row?.status).toBe('waiting_input')
    expect(row?.processId).toBe('p1')
  })

  it('marks a finished command as done', () => {
    const rows = run([
      called,
      { type: 'tool_result', data: { call_id: 'c1', tool: 'shell', ok: true, shell_status: 'completed', exit_code: 0 } },
    ])
    expect(rows.get('c1')?.status).toBe('done')
  })

  it('marks a failed call as error', () => {
    const rows = run([
      called,
      { type: 'tool_result', data: { call_id: 'c1', tool: 'shell', ok: false, is_error: true } },
    ])
    expect(rows.get('c1')?.status).toBe('error')
  })
})
