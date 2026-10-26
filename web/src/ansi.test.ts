import { describe, expect, it } from 'vitest'
import { extractShellOutput } from './ansi'

describe('extractShellOutput', () => {
  it('returns plain text unchanged', () => {
    expect(extractShellOutput('hello\nworld')).toBe('hello\nworld')
  })

  it('returns the stdout of a complete envelope with its formatting', () => {
    const env = JSON.stringify({ status: 'completed', stdout: '  PID CMD\n    1 init\n', command: 'ps' }, null, 2)
    expect(extractShellOutput(env)).toBe('  PID CMD\n    1 init\n')
  })

  it('appends stderr after stdout', () => {
    const env = JSON.stringify({ status: 'completed', stdout: 'out', stderr: 'warn' })
    expect(extractShellOutput(env)).toBe('out\nwarn')
  })

  it('reads a preview cut by the server in the middle of the stdout', () => {
    const cut = '{\n  "status": "completed",\n  "stdout": "line1\\nline2\\nli…'
    expect(extractShellOutput(cut)).toBe('line1\nline2\nli…')
  })

  it('drops a dangling escape at the cut', () => {
    expect(extractShellOutput('{"stdout": "abc\\')).toBe('abc')
    expect(extractShellOutput('{"stdout": "abc\\u00')).toBe('abc')
  })

  it('shows last_output for an input_required envelope', () => {
    const env = JSON.stringify({ status: 'input_required', last_output: 'Name: ', process_id: 'p' })
    expect(extractShellOutput(env)).toBe('Name: ')
  })

  it('falls back to the raw text when no output key exists', () => {
    expect(extractShellOutput('{"status":"ok"}')).toBe('{"status":"ok"}')
  })
})
