import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { ActivityRow } from '../types'
import { ActivityLog } from './ActivityLog'

vi.mock('./ActivityItem', () => ({
  ActivityItem: ({ row }: { row: ActivityRow }) => (
    <div data-testid="activity-item">{row.tool}</div>
  ),
}))

const makeRow = (overrides: Partial<ActivityRow> = {}): ActivityRow => ({
  callId: 'call-1',
  tool: 'shell',
  detail: 'ls',
  status: 'done',
  createdAt: Date.now(),
  ...overrides,
})

describe('ActivityLog', () => {
  it('renders the Activity heading', () => {
    render(<ActivityLog items={[]} apiKey="k" onClear={vi.fn()} />)
    expect(screen.getByRole('heading', { name: /activity/i })).toBeInTheDocument()
  })

  it('renders the Clear button', () => {
    render(<ActivityLog items={[]} apiKey="k" onClear={vi.fn()} />)
    expect(screen.getByRole('button', { name: /clear/i })).toBeInTheDocument()
  })

  it('shows the empty state message when there are no items', () => {
    render(<ActivityLog items={[]} apiKey="k" onClear={vi.fn()} />)
    expect(screen.getByText(/waiting for tool calls/i)).toBeInTheDocument()
  })

  it('renders one ActivityItem per row', () => {
    const rows = [makeRow({ callId: 'a' }), makeRow({ callId: 'b', tool: 'glob' })]
    render(<ActivityLog items={rows} apiKey="k" onClear={vi.fn()} />)
    expect(screen.getAllByTestId('activity-item')).toHaveLength(2)
    expect(screen.getByText('glob')).toBeInTheDocument()
  })

  it('does not show the empty state when items are present', () => {
    render(<ActivityLog items={[makeRow()]} apiKey="k" onClear={vi.fn()} />)
    expect(screen.queryByText(/waiting for tool calls/i)).not.toBeInTheDocument()
  })

  it('calls onClear when the Clear button is clicked', async () => {
    const onClear = vi.fn()
    render(<ActivityLog items={[makeRow()]} apiKey="k" onClear={onClear} />)
    await userEvent.click(screen.getByRole('button', { name: /clear/i }))
    expect(onClear).toHaveBeenCalledOnce()
  })
})
