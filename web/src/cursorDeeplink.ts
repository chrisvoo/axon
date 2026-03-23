/** Same shape as Cursor’s MCP install links: https://cursor.com/docs/context/mcp/install-links */

const CURSOR_MCP_INSTALL_BASE = 'cursor://anysphere.cursor-deeplink/mcp/install'

function utf8ToBase64(str: string): string {
  const bytes = new TextEncoder().encode(str)
  let binary = ''
  for (const b of bytes) {
    binary += String.fromCharCode(b)
  }
  return btoa(binary)
}

/**
 * Builds a Cursor `cursor://…/mcp/install` link. The `config` query param embeds the API key — treat as secret.
 */
export function cursorMcpInstallDeeplink(serverKey: string, mcpUrl: string, apiKey: string): string {
  const key = serverKey.trim() || 'axon'
  const payload = {
    [key]: {
      url: mcpUrl,
      headers: {
        Authorization: `Bearer ${apiKey}`,
      },
    },
  }
  const config = utf8ToBase64(JSON.stringify(payload))
  const params = new URLSearchParams({ name: key, config })
  return `${CURSOR_MCP_INSTALL_BASE}?${params.toString()}`
}

export const CURSOR_MCP_INSTALL_DOCS = 'https://cursor.com/docs/context/mcp/install-links'
