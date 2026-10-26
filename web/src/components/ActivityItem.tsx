import { useState } from 'react'
import type { ActivityRow } from '../types'
import { InputPrompt } from './InputPrompt'
import { extractShellOutput, parseAnsi } from '../ansi'

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

  // A shell result carries a JSON envelope: show only its output.
  const raw = row.outputPreview ?? ''
  const output = row.shellStatus ? extractShellOutput(raw) : raw

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
    <article className="shrink-0 overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900/60">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-start gap-3 px-4 py-3 text-left"
      >
        <span className="mt-1 shrink-0 font-mono text-xs text-zinc-500">
          {new Date(row.createdAt).toLocaleTimeString()}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-sm font-semibold text-emerald-400">{row.tool}</span>
            <span className={`rounded px-2 py-0.5 text-xs font-medium ${badgeClass}`}>{statusLabel}</span>
            {row.durationMs !== undefined && (
              <span className="text-xs text-zinc-500">{row.durationMs}ms</span>
            )}
          </div>
          {row.detail && (
            <p
              data-testid="command"
              className="mt-1 overflow-x-auto whitespace-nowrap pb-1 font-mono text-xs text-zinc-400"
            >
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
          {output && (
            <pre
              data-testid="output"
              className="max-h-80 overflow-auto whitespace-pre rounded-lg bg-black/40 p-3 font-mono text-xs text-zinc-200"
            >
              <AnsiPre text={output} />
            </pre>
          )}
          {row.status === 'waiting_input' && row.processId && (
            <InputPrompt processId={row.processId} apiKey={apiKey} hint={row.hint} mockApi={mockApi} />
          )}
        </div>
      )}
    </article>
  )
}
