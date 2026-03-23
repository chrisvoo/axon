import type { ActivityRow } from '../types'
import { ActivityItem } from './ActivityItem'

type Props = {
  items: ActivityRow[]
  apiKey: string
  onClear: () => void
}

export function ActivityLog({ items, apiKey, onClear }: Props) {
  return (
    <section className="flex min-h-0 flex-1 flex-col rounded-2xl border border-zinc-800 bg-zinc-900/40">
      <header className="flex items-center justify-between border-b border-zinc-800 px-4 py-3">
        <h2 className="text-sm font-semibold text-zinc-200">Activity</h2>
        <button
          type="button"
          onClick={onClear}
          className="rounded-md border border-zinc-700 px-3 py-1 text-xs font-medium text-zinc-300 hover:bg-zinc-800"
        >
          Clear
        </button>
      </header>
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-4">
        {items.length === 0 ? (
          <p className="text-sm text-zinc-500">Waiting for tool calls from Cursor…</p>
        ) : (
          items.map((row) => <ActivityItem key={row.callId} row={row} apiKey={apiKey} />)
        )}
      </div>
    </section>
  )
}
