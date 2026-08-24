import { useState } from 'react'

type Props = {
  commands: string[]
  onClose: () => void
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }
  return (
    <button
      type="button"
      onClick={copy}
      title="Copy to clipboard"
      className="shrink-0 rounded p-1 text-zinc-400 transition hover:bg-zinc-700 hover:text-zinc-100"
    >
      {copied ? (
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M2 8l4 4 8-8" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      ) : (
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
          <rect x="5" y="5" width="9" height="9" rx="1.5" />
          <path d="M4 11H3a1 1 0 01-1-1V3a1 1 0 011-1h7a1 1 0 011 1v1" />
        </svg>
      )}
    </button>
  )
}

export function PrivilegedModal({ commands, onClose }: Props) {
  const allText = commands.join('\n')

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div className="w-full max-w-lg rounded-2xl border border-zinc-700 bg-zinc-900 shadow-2xl">
        <header className="flex items-center justify-between border-b border-zinc-800 px-5 py-4">
          <div>
            <h2 className="font-semibold text-zinc-100">Privileged commands</h2>
            <p className="mt-0.5 text-xs text-zinc-400">
              These commands require elevated privileges. Copy and run them manually on the remote host.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="ml-4 rounded-md p-1.5 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M2 2l12 12M14 2L2 14" />
            </svg>
          </button>
        </header>

        <div className="space-y-2 p-5">
          {commands.map((cmd, i) => (
            <div key={i} className="flex items-start gap-2 rounded-lg bg-black/50 px-3 py-2.5">
              <pre className="min-w-0 flex-1 overflow-x-auto whitespace-pre font-mono text-xs text-zinc-200">
                {cmd}
              </pre>
              <CopyButton text={cmd} />
            </div>
          ))}
        </div>

        <footer className="flex items-center justify-end gap-3 border-t border-zinc-800 px-5 py-3">
          <CopyButton text={allText} />
          <span className="text-xs text-zinc-500">Copy all</span>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border border-zinc-700 px-4 py-1.5 text-xs font-medium text-zinc-300 hover:bg-zinc-800"
          >
            Dismiss
          </button>
        </footer>
      </div>
    </div>
  )
}
