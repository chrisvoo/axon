import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
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

  describe('scroll to top button', () => {
    const scrollTo = vi.fn()
    beforeEach(() => {
      scrollTo.mockClear()
      Element.prototype.scrollTo = scrollTo as unknown as typeof Element.prototype.scrollTo
    })

    const scrollBy = (el: HTMLElement, top: number) => {
      el.scrollTop = top
      fireEvent.scroll(el)
    }

    it('is hidden until the list is scrolled down', () => {
      render(<ActivityLog items={[makeRow()]} apiKey="k" onClear={vi.fn()} />)
      const btn = screen.getByRole('button', { name: /scroll to top/i, hidden: true })
      expect(btn).toHaveClass('opacity-0')
      scrollBy(screen.getByTestId('activity-scroll'), 500)
      expect(btn).toHaveClass('opacity-100')
      scrollBy(screen.getByTestId('activity-scroll'), 10)
      expect(btn).toHaveClass('opacity-0')
    })

    it('scrolls smoothly to the top when clicked', async () => {
      render(<ActivityLog items={[makeRow()]} apiKey="k" onClear={vi.fn()} />)
      scrollBy(screen.getByTestId('activity-scroll'), 500)
      await userEvent.click(screen.getByRole('button', { name: /scroll to top/i }))
      expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })
    })
  })
})
