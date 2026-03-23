type Props = {
  connected: boolean
  version?: string
  requests: number
  errors: number
  since: number | null
}

export function SessionStats({ connected, version, requests, errors, since }: Props) {
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/40 p-4">
      <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-zinc-500">Session</h3>
      <dl className="space-y-2 text-xs text-zinc-400">
        <div className="flex items-center justify-between gap-2">
          <dt>Dashboard</dt>
          <dd className="flex items-center gap-2">
            <span
              className={`inline-block h-2 w-2 rounded-full ${connected ? 'bg-emerald-400' : 'bg-red-500'}`}
              aria-hidden
            />
            <span className="font-mono text-zinc-200">{connected ? 'Connected' : 'Reconnecting…'}</span>
          </dd>
        </div>
        {version ? (
          <div className="flex justify-between gap-2">
            <dt>Agent</dt>
            <dd className="font-mono text-zinc-200">v{version}</dd>
          </div>
        ) : null}
        <div className="flex justify-between gap-2">
          <dt>Tool calls</dt>
          <dd className="font-mono text-zinc-200">{requests}</dd>
        </div>
        <div className="flex justify-between gap-2">
          <dt>Errors</dt>
          <dd className="font-mono text-zinc-200">{errors}</dd>
        </div>
        {since ? (
          <div className="flex justify-between gap-2">
            <dt>Since</dt>
            <dd className="font-mono text-zinc-200">{new Date(since).toLocaleTimeString()}</dd>
          </div>
        ) : null}
      </dl>
    </div>
  )
}
