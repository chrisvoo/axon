import { describe, expect, it } from 'vitest'
import { CURSOR_MCP_INSTALL_DOCS, cursorMcpInstallDeeplink } from './cursorDeeplink'

describe('cursorMcpInstallDeeplink', () => {
  it('returns a cursor:// URL', () => {
    const link = cursorMcpInstallDeeplink('axon', 'https://example.com/mcp', 'key123')
    expect(link).toMatch(/^cursor:\/\//)
  })

  it('includes the server name as the name query param', () => {
    const link = cursorMcpInstallDeeplink('my-server', 'https://example.com/mcp', 'k')
    expect(link).toContain('name=my-server')
  })

  it('falls back to "axon" when serverKey is empty', () => {
    const link = cursorMcpInstallDeeplink('', 'https://example.com/mcp', 'k')
    expect(link).toContain('name=axon')
  })

  it('embeds the MCP URL and API key in the Base64 config param', () => {
    const link = cursorMcpInstallDeeplink('axon', 'https://my.host/mcp', 'axon_k_secret')
    const rawUrl = link.replace('cursor://', 'https://')
    const params = new URL(rawUrl).searchParams
    const decoded = JSON.parse(atob(params.get('config')!)) as {
      axon: { url: string; headers: { Authorization: string } }
    }
    expect(decoded.axon.url).toBe('https://my.host/mcp')
    expect(decoded.axon.headers.Authorization).toBe('Bearer axon_k_secret')
  })

  it('URL-encodes the config param', () => {
    const link = cursorMcpInstallDeeplink('axon', 'https://x.com/mcp', 'k')
    expect(link).not.toContain('=={')
  })
})

describe('CURSOR_MCP_INSTALL_DOCS', () => {
  it('points to the Cursor docs domain', () => {
    expect(CURSOR_MCP_INSTALL_DOCS).toMatch(/cursor\.com/)
  })
})
