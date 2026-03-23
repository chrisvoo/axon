import { useMemo, useState } from 'react'
import { CURSOR_MCP_INSTALL_DOCS, cursorMcpInstallDeeplink } from '../cursorDeeplink'

type Props = {
  apiKey: string
}

export function MCPConfigPanel({ apiKey }: Props) {
  const [copiedJson, setCopiedJson] = useState(false)
  const [copiedLink, setCopiedLink] = useState(false)

  const mcpUrl = useMemo(() => new URL('/mcp', window.location.origin).href, [])

  const snippet = useMemo(() => {
    return JSON.stringify(
      {
        mcpServers: {
          axon: {
            url: mcpUrl,
            headers: {
              Authorization: `Bearer ${apiKey}`,
            },
          },
        },
      },
      null,
      2,
    )
  }, [apiKey, mcpUrl])

  const installLink = useMemo(() => cursorMcpInstallDeeplink('axon', mcpUrl, apiKey), [apiKey, mcpUrl])

  const copyJson = async () => {
    await navigator.clipboard.writeText(snippet)
    setCopiedJson(true)
    window.setTimeout(() => setCopiedJson(false), 2000)
  }

  const copyLink = async () => {
    await navigator.clipboard.writeText(installLink)
    setCopiedLink(true)
    window.setTimeout(() => setCopiedLink(false), 2000)
  }

  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/40 p-4">
      <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-zinc-500">MCP config (Cursor)</h3>

      <p className="mb-3 text-xs leading-relaxed text-zinc-500">
        <strong className="font-medium text-zinc-300">JSON file</strong> — paste into{' '}
        <span className="font-mono">~/.cursor/mcp.json</span> or{' '}
        <span className="font-mono">&lt;project&gt;/.cursor/mcp.json</span>. See the repo README for paths and TLS
        notes.
      </p>
      <pre className="max-h-40 overflow-auto rounded-lg bg-black/50 p-3 font-mono text-[11px] leading-relaxed text-zinc-300">
        {snippet}
      </pre>
      <button
        type="button"
        onClick={() => void copyJson()}
        className="mt-2 w-full rounded-md border border-zinc-700 px-3 py-2 text-xs font-medium text-zinc-200 hover:bg-zinc-800"
      >
        {copiedJson ? 'Copied JSON' : 'Copy JSON snippet'}
      </button>

      <div className="my-4 border-t border-zinc-800" />

      <p className="mb-2 text-xs leading-relaxed text-zinc-500">
        <strong className="font-medium text-zinc-300">One-click install</strong> — Cursor can register this server from
        a <span className="font-mono">cursor://</span> link (official format in{' '}
        <a
          href={CURSOR_MCP_INSTALL_DOCS}
          target="_blank"
          rel="noopener noreferrer"
          className="text-emerald-400 underline hover:text-emerald-300"
        >
          Cursor’s MCP install links docs
        </a>
        , including a link generator if you need to tweak the payload).
      </p>
      <p className="mb-2 rounded-md border border-amber-500/30 bg-amber-500/10 px-2 py-1.5 text-[11px] text-amber-100/90">
        <strong className="text-amber-200">Security:</strong> this URL embeds your API key in Base64 query parameters.
        Anyone with the full link can use your agent. Do not paste it in public chats, tickets, or screen shares.
      </p>
      <pre className="max-h-24 overflow-auto break-all rounded-lg bg-black/50 p-2 font-mono text-[10px] leading-snug text-zinc-400">
        {installLink}
      </pre>
      <div className="mt-2 flex flex-col gap-2 sm:flex-row">
        <a
          href={installLink}
          className="flex flex-1 items-center justify-center rounded-md bg-emerald-600 px-3 py-2 text-center text-xs font-semibold text-white hover:bg-emerald-500"
        >
          Open in Cursor
        </a>
        <button
          type="button"
          onClick={() => void copyLink()}
          className="flex-1 rounded-md border border-zinc-700 px-3 py-2 text-xs font-medium text-zinc-200 hover:bg-zinc-800"
        >
          {copiedLink ? 'Copied link' : 'Copy install link'}
        </button>
      </div>
    </div>
  )
}
