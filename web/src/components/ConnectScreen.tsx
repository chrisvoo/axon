import { FormEvent, useState } from 'react'
import { API_KEY_STORAGE } from '../types'

type Props = {
  onConnect: (apiKey: string) => void
}

export function ConnectScreen({ onConnect }: Props) {
  const [key, setKey] = useState(() => sessionStorage.getItem(API_KEY_STORAGE) ?? '')
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    const trimmed = key.trim()
    if (!trimmed) {
      setError('Enter your API key')
      return
    }
    sessionStorage.setItem(API_KEY_STORAGE, trimmed)
    setError(null)
    onConnect(trimmed)
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-zinc-950 px-4">
      <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-900/80 p-8 shadow-xl backdrop-blur">
        <div className="mb-6 text-center">
          <div className="mb-2 text-2xl font-semibold tracking-tight text-white">Axon</div>
          <p className="text-sm text-zinc-400">Enter the API key shown when you run `axon serve`</p>
        </div>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <label className="flex flex-col gap-2 text-sm font-medium text-zinc-300">
            API key
            <input
              type="password"
              autoComplete="off"
              value={key}
              onChange={(ev) => setKey(ev.target.value)}
              className="rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 font-mono text-sm text-zinc-100 outline-none ring-emerald-500/30 focus:border-emerald-600 focus:ring"
              placeholder="axon_k_…"
            />
          </label>
          {error ? <p className="text-sm text-red-400">{error}</p> : null}
          <button
            type="submit"
            className="rounded-lg bg-emerald-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-emerald-500"
          >
            Connect
          </button>
        </form>
      </div>
    </div>
  )
}
