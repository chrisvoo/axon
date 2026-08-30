import { useState } from 'react'
import type { AxonEvent } from '../types'
import { SCENARIOS } from './fixtures'

type Props = {
  onInject: (event: AxonEvent) => void
}

const STEP_DELAY_MS = 350

export function DemoPanel({ onInject }: Props) {
  const [selectedIdx, setSelectedIdx] = useState(0)
  const [busy, setBusy] = useState(false)

  const fire = async () => {
    setBusy(true)
    const events = SCENARIOS[selectedIdx].build()
    for (const ev of events) {
      onInject(ev)
      await new Promise<void>((r) => setTimeout(r, STEP_DELAY_MS))
    }
    setBusy(false)
  }

  const scenario = SCENARIOS[selectedIdx]

  return (
    <div className="rounded-lg border border-violet-500/30 bg-violet-500/10 p-3">
      <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-violet-300">
        Demo mode
      </p>
      <select
        value={selectedIdx}
        onChange={(e) => setSelectedIdx(Number(e.target.value))}
        className="mb-2 w-full rounded-md border border-zinc-700 bg-zinc-900 px-2 py-1.5 text-xs text-zinc-200 focus:outline-none focus:border-violet-500"
      >
        {SCENARIOS.map((s, i) => (
          <option key={i} value={i}>
            {s.label}
          </option>
        ))}
      </select>
      {scenario.kind === 'input_required' && (
        <p className="mb-2 text-xs text-amber-300">
          ⚠ This scenario shows the input dialog — submit/cancel are mocked.
        </p>
      )}
      <button
        type="button"
        disabled={busy}
        onClick={() => void fire()}
        className="w-full rounded-md bg-violet-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-violet-500 disabled:opacity-50"
      >
        {busy ? 'Injecting…' : 'Inject event'}
      </button>
    </div>
  )
}
