import { useRef, useState } from 'react'
import type { ActivityRow } from '../types'
import { ActivityItem } from './ActivityItem'

type Props = {
  items: ActivityRow[]
  apiKey: string
  onClear: () => void
  mockApi?: boolean
}

// Scroll distance (px) after which the back-to-top button appears
const SHOW_TOP_AFTER = 200

export function ActivityLog({ items, apiKey, onClear, mockApi }: Props) {
  const scroller = useRef<HTMLDivElement>(null)
  const [showTop, setShowTop] = useState(false)

  return (
    <section className="relative flex min-h-0 flex-1 flex-col rounded-2xl border border-zinc-800 bg-zinc-900/40">
      <header className="flex shrink-0 items-center justify-between border-b border-zinc-800 px-4 py-3">
        <h2 className="text-sm font-semibold text-zinc-200">Activity</h2>
        <button
          type="button"
          onClick={onClear}
          className="rounded-md border border-zinc-700 px-3 py-1 text-xs font-medium text-zinc-300 hover:bg-zinc-800"
        >
          Clear
        </button>
      </header>
      <div
        ref={scroller}
        data-testid="activity-scroll"
        onScroll={(e) => setShowTop(e.currentTarget.scrollTop > SHOW_TOP_AFTER)}
        className="flex min-h-0 flex-1 flex-col gap-3 overflow-x-hidden overflow-y-auto p-4"
      >
        {items.length === 0 ? (
          <p className="text-sm text-zinc-500">Waiting for tool calls from the AI agent</p>
        ) : (
          items.map((row) => <ActivityItem key={row.callId} row={row} apiKey={apiKey} mockApi={mockApi} />)
        )}
      </div>
      <button
        type="button"
        aria-label="Scroll to top"
        tabIndex={showTop ? 0 : -1}
        onClick={() => scroller.current?.scrollTo({ top: 0, behavior: 'smooth' })}
        className={`absolute bottom-5 right-5 flex h-10 w-10 items-center justify-center rounded-full border border-zinc-700 bg-zinc-800 text-zinc-200 shadow-lg transition-all duration-300 hover:bg-zinc-700 ${
          showTop ? 'translate-y-0 opacity-100' : 'pointer-events-none translate-y-2 opacity-0'
        }`}
      >
        <svg viewBox="0 0 20 20" fill="currentColor" className="h-5 w-5" aria-hidden="true">
          <path fillRule="evenodd" d="M10 4a1 1 0 0 1 .7.3l5 5a1 1 0 1 1-1.4 1.4L11 7.4V15a1 1 0 1 1-2 0V7.4l-3.3 3.3a1 1 0 0 1-1.4-1.4l5-5A1 1 0 0 1 10 4Z" clipRule="evenodd" />
        </svg>
      </button>
    </section>
  )
}
