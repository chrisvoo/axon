import { useState } from 'react'
import type { ActivityRow } from '../types'
import { InputPrompt } from './InputPrompt'
import { parseAnsi } from '../ansi'

const MAX_LINES = 10

function countLines(s: string) { return s ? s.split('\n').length : 0 }

function truncateLines(s: string, max: number): { text: string; truncated: boolean } {
  const lines = s.split('\n')
  if (lines.length <= max) return { text: s, truncated: false }
  return { text: lines.slice(0, max).join('\n'), truncated: true }
}

function AnsiPre({ text }: { text: string }) {
  const spans = parseAnsi(text)
  return (
    <>
      {spans.map((span, i) => (
        <span
          key={i}
          style={{
            color: span.fg,
            fontWeight: span.bold ? 'bold' : undefined,
          }}
        >
          {span.text}
        </span>
      ))}
    </>
  )
}

type Props = { row: ActivityRow; apiKey: string; mockApi?: boolean }

export function ActivityItem({ row, apiKey, mockApi }: Props) {
  const [open, setOpen] = useState(true)
  const [expanded, setExpanded] = useState(false)

  const preview = row.outputPreview ?? ''
  const { text: shown, truncated } = expanded
    ? { text: preview, truncated: false }
    : truncateLines(preview, MAX_LINES)
  const lines = countLines(preview)

  const statusLabel =
    row.status === 'waiting_input' ? 'waiting'
    : row.status === 'pending' ? 'running'
    : row.status === 'error' ? 'error'
    : 'done'

  const badgeClass =
    row.status === 'waiting_input' ? 'bg-amber-500/20 text-amber-200'
    : row.status === 'error' ? 'bg-red-500/20 text-red-200'
    : row.status === 'pending' ? 'bg-sky-500/20 text-sky-200'
    : 'bg-emerald-500/20 text-emerald-200'

  return (
    <article className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900/60">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-start gap-3 px-4 py-3 text-left"
      >
        <span className="mt-1 shrink-0 font-mono text-xs text-zinc-500">
          {new Date(row.createdAt).toLocaleTimeString()}
        </span>
        <div className="min-w-0 flex-1 overflow-hidden">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-sm font-semibold text-emerald-400">{row.tool}</span>
            <span className={`rounded px-2 py-0.5 text-xs font-medium ${badgeClass}`}>{statusLabel}</span>
            {row.durationMs !== undefined && (
              <span className="text-xs text-zinc-500">{row.durationMs}ms</span>
            )}
          </div>
          {row.detail && (
            <p className="mt-1 truncate font-mono text-xs text-zinc-400" title={row.detail}>
              {row.detail}
            </p>
          )}
        </div>
      </button>

      {open && (
        <div className="space-y-3 border-t border-zinc-800 px-4 py-3">
          {row.shellStatus && (
            <p className="text-xs text-zinc-400">
              shell status: <span className="font-mono text-zinc-200">{row.shellStatus}</span>
              {row.exitCode !== undefined && (
                <span className="ml-2">exit <span className="font-mono">{row.exitCode}</span></span>
              )}
            </p>
          )}
          {preview && (
            <div>
              <pre className="max-h-60 overflow-y-auto overflow-x-hidden whitespace-pre-wrap break-all rounded-lg bg-black/40 p-3 font-mono text-xs text-zinc-200">
                <AnsiPre text={shown} />
              </pre>
              {truncated && !expanded && (
                <button
                  type="button"
                  onClick={() => setExpanded(true)}
                  className="mt-2 text-xs font-medium text-emerald-400 hover:text-emerald-300"
                >
                  Show more ({lines} lines)
                </button>
              )}
            </div>
          )}
          {row.status === 'waiting_input' && row.processId && (
            <InputPrompt processId={row.processId} apiKey={apiKey} hint={row.hint} mockApi={mockApi} />
          )}
        </div>
      )}
    </article>
  )
}
