import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { ActivityRow } from '../types'
import { ActivityItem } from './ActivityItem'

const BASE_ROW: ActivityRow = {
  callId: 'call-1',
  tool: 'shell',
  detail: 'echo hello',
  status: 'done',
  createdAt: new Date('2026-01-01T12:00:00Z').getTime(),
}

describe('ActivityItem', () => {
  it('renders the tool name', () => {
    render(<ActivityItem row={BASE_ROW} />)
    expect(screen.getByText('shell')).toBeInTheDocument()
  })

  it('renders the detail string', () => {
    render(<ActivityItem row={BASE_ROW} />)
    expect(screen.getByText('echo hello')).toBeInTheDocument()
  })

  it.each([
    ['done', 'done'],
    ['pending', 'running'],
    ['error', 'error'],
    ['waiting_input', 'waiting'],
  ] as [ActivityRow['status'], string][])(
    'shows "%s" badge label for status "%s"',
    (status, label) => {
      render(
        <ActivityItem
          row={{ ...BASE_ROW, status, processId: status === 'waiting_input' ? 'p1' : undefined }}
         
        />,
      )
      expect(screen.getByText(label)).toBeInTheDocument()
    },
  )

  it('renders duration in ms when provided', () => {
    render(<ActivityItem row={{ ...BASE_ROW, durationMs: 123 }} />)
    expect(screen.getByText('123ms')).toBeInTheDocument()
  })

  it('does not render a duration row when durationMs is absent', () => {
    render(<ActivityItem row={BASE_ROW} />)
    expect(screen.queryByText(/ms$/)).not.toBeInTheDocument()
  })

  it('renders the output preview', () => {
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: 'hello output' }} />)
    expect(screen.getByText('hello output')).toBeInTheDocument()
  })

  it('scrolls a long output instead of truncating it', () => {
    const longOutput = Array.from({ length: 50 }, (_, i) => `line ${i + 1}`).join('\n')
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: longOutput }} />)
    expect(screen.queryByRole('button', { name: /show more/i })).not.toBeInTheDocument()
    expect(screen.getByTestId('output')).toHaveClass('max-h-80', 'overflow-auto')
    expect(screen.getByTestId('output').textContent).toContain('line 50')
  })

  it('shows only the stdout of a shell envelope', () => {
    const envelope = JSON.stringify({ status: 'completed', stdout: 'a\n  b', exit_code: 0, command: 'ls' }, null, 2)
    render(<ActivityItem row={{ ...BASE_ROW, shellStatus: 'completed', outputPreview: envelope }} />)
    expect(screen.getByTestId('output').textContent).toBe('a\n  b')
  })

  it('keeps a long command on one line with a horizontal scrollbar', () => {
    const cmd = 'echo ' + 'x'.repeat(500)
    render(<ActivityItem row={{ ...BASE_ROW, detail: cmd }} />)
    const el = screen.getByTestId('command')
    expect(el).toHaveClass('overflow-x-auto', 'whitespace-nowrap')
    expect(el).not.toHaveClass('truncate')
    expect(el.textContent).toBe(cmd)
  })

  it('collapses the body when the header button is clicked', async () => {
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: 'visible output' }} />)
    expect(screen.getByText('visible output')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /shell/i }))
    expect(screen.queryByText('visible output')).not.toBeInTheDocument()
  })

  it('re-expands the body after a second header click', async () => {
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: 'visible output' }} />)
    const toggle = screen.getByRole('button', { name: /shell/i })
    await userEvent.click(toggle)
    await userEvent.click(toggle)
    expect(screen.getByText('visible output')).toBeInTheDocument()
  })

  it('renders shell status and exit code when present', () => {
    render(<ActivityItem row={{ ...BASE_ROW, shellStatus: 'done', exitCode: 0 }} />)
    expect(screen.getByText(/shell status/i)).toBeInTheDocument()
    expect(screen.getByText(/exit/i)).toBeInTheDocument()
  })

  it('shows a read-only notice when the command waits for input', () => {
    render(<ActivityItem row={{ ...BASE_ROW, status: 'waiting_input', processId: 'proc-abc', hint: 'Use send_input.' }} />)
    expect(screen.getByTestId('waiting-notice')).toHaveTextContent('Use send_input.')
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/stdin/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^(send|cancel)$/i })).not.toBeInTheDocument()
  })

  it('shows a default notice when the hint is empty', () => {
    render(<ActivityItem row={{ ...BASE_ROW, status: 'waiting_input', hint: '' }} />)
    expect(screen.getByTestId('waiting-notice')).toHaveTextContent(/send_input/)
  })

  it('does not show the notice for non-waiting statuses', () => {
    render(<ActivityItem row={{ ...BASE_ROW, status: 'done' }} />)
    expect(screen.queryByTestId('waiting-notice')).not.toBeInTheDocument()
  })
})
