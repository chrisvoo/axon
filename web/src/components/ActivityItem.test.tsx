import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { ActivityRow } from '../types'
import { ActivityItem } from './ActivityItem'

vi.mock('./InputPrompt', () => ({
  InputPrompt: ({ processId }: { processId: string }) => (
    <div data-testid="input-prompt">{processId}</div>
  ),
}))

const BASE_ROW: ActivityRow = {
  callId: 'call-1',
  tool: 'shell',
  detail: 'echo hello',
  status: 'done',
  createdAt: new Date('2026-01-01T12:00:00Z').getTime(),
}

describe('ActivityItem', () => {
  it('renders the tool name', () => {
    render(<ActivityItem row={BASE_ROW} apiKey="k" />)
    expect(screen.getByText('shell')).toBeInTheDocument()
  })

  it('renders the detail string', () => {
    render(<ActivityItem row={BASE_ROW} apiKey="k" />)
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
          apiKey="k"
        />,
      )
      expect(screen.getByText(label)).toBeInTheDocument()
    },
  )

  it('renders duration in ms when provided', () => {
    render(<ActivityItem row={{ ...BASE_ROW, durationMs: 123 }} apiKey="k" />)
    expect(screen.getByText('123ms')).toBeInTheDocument()
  })

  it('does not render a duration row when durationMs is absent', () => {
    render(<ActivityItem row={BASE_ROW} apiKey="k" />)
    expect(screen.queryByText(/ms$/)).not.toBeInTheDocument()
  })

  it('renders the output preview', () => {
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: 'hello output' }} apiKey="k" />)
    expect(screen.getByText('hello output')).toBeInTheDocument()
  })

  it('shows "Show more" when the preview exceeds 10 lines', () => {
    const longOutput = Array.from({ length: 12 }, (_, i) => `line ${i + 1}`).join('\n')
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: longOutput }} apiKey="k" />)
    expect(screen.getByRole('button', { name: /show more/i })).toBeInTheDocument()
  })

  it('does not show "Show more" when the preview is ≤10 lines', () => {
    const shortOutput = Array.from({ length: 8 }, (_, i) => `line ${i + 1}`).join('\n')
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: shortOutput }} apiKey="k" />)
    expect(screen.queryByRole('button', { name: /show more/i })).not.toBeInTheDocument()
  })

  it('expands the full preview after clicking "Show more"', async () => {
    const lines = Array.from({ length: 12 }, (_, i) => `line ${i + 1}`)
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: lines.join('\n') }} apiKey="k" />)
    await userEvent.click(screen.getByRole('button', { name: /show more/i }))
    expect(screen.queryByRole('button', { name: /show more/i })).not.toBeInTheDocument()
    expect(screen.getByText(/line 12/)).toBeInTheDocument()
  })

  it('collapses the body when the header button is clicked', async () => {
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: 'visible output' }} apiKey="k" />)
    expect(screen.getByText('visible output')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /shell/i }))
    expect(screen.queryByText('visible output')).not.toBeInTheDocument()
  })

  it('re-expands the body after a second header click', async () => {
    render(<ActivityItem row={{ ...BASE_ROW, outputPreview: 'visible output' }} apiKey="k" />)
    const toggle = screen.getByRole('button', { name: /shell/i })
    await userEvent.click(toggle)
    await userEvent.click(toggle)
    expect(screen.getByText('visible output')).toBeInTheDocument()
  })

  it('renders shell status and exit code when present', () => {
    render(<ActivityItem row={{ ...BASE_ROW, shellStatus: 'done', exitCode: 0 }} apiKey="k" />)
    expect(screen.getByText(/shell status/i)).toBeInTheDocument()
    expect(screen.getByText(/exit/i)).toBeInTheDocument()
  })

  it('shows InputPrompt when status is waiting_input with a processId', () => {
    render(
      <ActivityItem row={{ ...BASE_ROW, status: 'waiting_input', processId: 'proc-abc' }} apiKey="k" />,
    )
    expect(screen.getByTestId('input-prompt')).toBeInTheDocument()
  })

  it('does not show InputPrompt for non-waiting statuses', () => {
    render(<ActivityItem row={{ ...BASE_ROW, status: 'done' }} apiKey="k" />)
    expect(screen.queryByTestId('input-prompt')).not.toBeInTheDocument()
  })
})
