type Disk = {
  mountpoint: string
  total_gb: string
  used_pct: number
}

type Props = {
  stats: {
    hostname?: string
    platform?: string
    platform_ver?: string
    os?: string
    cpu_cores?: number
    mem_total_gb?: string
    mem_used_pct?: number
    disk?: Disk[]
  } | null
}

function Bar({ label, pct }: { label: string; pct: number }) {
  const clamped = Math.min(100, Math.max(0, pct))
  return (
    <div className="flex flex-col gap-1">
      <div className="flex justify-between text-xs text-zinc-400">
        <span>{label}</span>
        <span>{clamped.toFixed(0)}%</span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-zinc-800">
        <div className="h-full rounded-full bg-emerald-500/80" style={{ width: `${clamped}%` }} />
      </div>
    </div>
  )
}

export function SystemStatus({ stats }: Props) {
  if (!stats) {
    return (
      <div className="rounded-xl border border-zinc-800 bg-zinc-900/40 p-4">
        <p className="text-sm text-zinc-500">Collecting system metrics…</p>
      </div>
    )
  }

  const disk = stats.disk?.[0]

  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/40 p-4">
      <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-zinc-500">System</h3>
      <div className="flex flex-col gap-4">
        {typeof stats.mem_used_pct === 'number' ? <Bar label="Memory" pct={stats.mem_used_pct} /> : null}
        {disk ? <Bar label={`Disk ${disk.mountpoint}`} pct={disk.used_pct} /> : null}
        <dl className="space-y-1 text-xs text-zinc-400">
          <div className="flex justify-between gap-2">
            <dt>Host</dt>
            <dd className="font-mono text-zinc-200">{stats.hostname ?? '—'}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt>OS</dt>
            <dd className="text-right font-mono text-zinc-200">
              {stats.os} {stats.platform} {stats.platform_ver}
            </dd>
          </div>
          {typeof stats.cpu_cores === 'number' ? (
            <div className="flex justify-between gap-2">
              <dt>CPU cores</dt>
              <dd className="font-mono text-zinc-200">{stats.cpu_cores}</dd>
            </div>
          ) : null}
          {stats.mem_total_gb ? (
            <div className="flex justify-between gap-2">
              <dt>RAM total</dt>
              <dd className="font-mono text-zinc-200">{stats.mem_total_gb} GB</dd>
            </div>
          ) : null}
        </dl>
      </div>
    </div>
  )
}
