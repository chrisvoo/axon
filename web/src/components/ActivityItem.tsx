import { useState } from 'react'
import type { ActivityRow } from '../types'
import { InputPrompt } from './InputPrompt'

const MAX_PREVIEW = 10

function countLines(s: string): number {
  if (!s) {
    return 0
  }
  return s.split('\n').length
}

function truncateLines(s: string, max: number): { text: string; truncated: boolean } {
  const lines = s.split('\n')
  if (lines.length <= max) {
    return { text: s, truncated: false }
  }
  return { text: lines.slice(0, max).join('\n'), truncated: true }
}

type Props = {
  row: ActivityRow
  apiKey: string
}

export function ActivityItem({ row, apiKey }: Props) {
  const [open, setOpen] = useState(true)
  const [expanded, setExpanded] = useState(false)

  const preview = row.outputPreview ?? ''
  const { text: shown, truncated } = expanded
    ? { text: preview, truncated: false }
    : truncateLines(preview, MAX_PREVIEW)
  const lines = countLines(preview)

  const statusLabel =
    row.status === 'waiting_input'
      ? 'waiting'
      : row.status === 'pending'
        ? 'running'
        : row.status === 'error'
          ? 'error'
          : 'done'

  const badgeClass =
    row.status === 'waiting_input'
      ? 'bg-amber-500/20 text-amber-200'
      : row.status === 'error'
        ? 'bg-red-500/20 text-red-200'
        : row.status === 'pending'
          ? 'bg-sky-500/20 text-sky-200'
          : 'bg-emerald-500/20 text-emerald-200'

  return (
    <article className="rounded-xl border border-zinc-800 bg-zinc-900/60">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-start gap-3 px-4 py-3 text-left"
      >
        <span className="mt-1 font-mono text-xs text-zinc-500">{new Date(row.createdAt).toLocaleTimeString()}</span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-sm font-semibold text-emerald-400">{row.tool}</span>
            <span className={`rounded px-2 py-0.5 text-xs font-medium ${badgeClass}`}>{statusLabel}</span>
            {row.durationMs !== undefined ? (
              <span className="text-xs text-zinc-500">{row.durationMs}ms</span>
            ) : null}
          </div>
          {row.detail ? (
            <p className="mt-1 truncate font-mono text-xs text-zinc-400" title={row.detail}>
              {row.detail}
            </p>
          ) : null}
        </div>
      </button>
      {open ? (
        <div className="space-y-3 border-t border-zinc-800 px-4 py-3">
          {row.shellStatus ? (
            <p className="text-xs text-zinc-400">
              shell status: <span className="font-mono text-zinc-200">{row.shellStatus}</span>
              {row.exitCode !== undefined ? (
                <span className="ml-2">
                  exit <span className="font-mono">{row.exitCode}</span>
                </span>
              ) : null}
            </p>
          ) : null}
          {preview ? (
            <div>
              <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-black/40 p-3 font-mono text-xs text-zinc-200">
                {shown}
              </pre>
              {truncated && !expanded ? (
                <button
                  type="button"
                  onClick={() => setExpanded(true)}
                  className="mt-2 text-xs font-medium text-emerald-400 hover:text-emerald-300"
                >
                  Show more ({lines} lines)
                </button>
              ) : null}
            </div>
          ) : null}
          {row.status === 'waiting_input' && row.processId ? (
            <InputPrompt processId={row.processId} apiKey={apiKey} hint={row.hint} />
          ) : null}
        </div>
      ) : null}
    </article>
  )
}
