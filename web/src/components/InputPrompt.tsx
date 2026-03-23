import { FormEvent, useState } from 'react'
import { apiPostJson } from '../api'

type Props = {
  processId: string
  apiKey: string
  hint?: string
}

export function InputPrompt({ processId, apiKey, hint }: Props) {
  const [value, setValue] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sent, setSent] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await apiPostJson('/api/input', apiKey, { process_id: processId, data: value })
      setValue('')
      setSent(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send input')
    } finally {
      setBusy(false)
    }
  }

  const cancel = async () => {
    setBusy(true)
    setError(null)
    try {
      await apiPostJson('/api/cancel', apiKey, { process_id: processId })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel')
    } finally {
      setBusy(false)
    }
  }

  if (sent) {
    return (
      <div className="mt-3 rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-200">
        Input sent. Waiting for the command to finish…
      </div>
    )
  }

  return (
    <div className="mt-3 rounded-lg border border-amber-500/40 bg-amber-500/10 p-4">
      <div className="mb-3 flex items-start gap-2 text-amber-200">
        <span className="mt-0.5 text-lg" aria-hidden>
          ⚠
        </span>
        <div>
          <p className="font-medium">Command needs interactive input</p>
          {hint ? <p className="mt-1 text-sm text-amber-100/80">{hint}</p> : null}
        </div>
      </div>
      <form onSubmit={submit} className="flex flex-col gap-3">
        <label className="text-xs font-medium uppercase tracking-wide text-zinc-400">stdin</label>
        <textarea
          value={value}
          onChange={(ev) => setValue(ev.target.value)}
          rows={3}
          className="w-full resize-y rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 font-mono text-sm text-zinc-100 outline-none focus:border-amber-500"
          placeholder="Password or response…"
        />
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <div className="flex flex-wrap gap-2">
          <button
            type="submit"
            disabled={busy}
            className="rounded-md bg-amber-600 px-4 py-2 text-sm font-semibold text-zinc-950 hover:bg-amber-500 disabled:opacity-50"
          >
            Send
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={() => void cancel()}
            className="rounded-md border border-zinc-600 px-4 py-2 text-sm text-zinc-200 hover:bg-zinc-800 disabled:opacity-50"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  )
}
