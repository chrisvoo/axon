import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MCPConfigPanel } from './MCPConfigPanel'

beforeEach(() => {
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  })
})

describe('MCPConfigPanel', () => {
  it('shows the MCP JSON snippet containing the API key', () => {
    render(<MCPConfigPanel apiKey="axon_k_test" />)
    expect(screen.getByText(/axon_k_test/)).toBeInTheDocument()
  })

  it('includes the /mcp path in the JSON snippet', () => {
    render(<MCPConfigPanel apiKey="k" />)
    // getAllByText because the path also appears in the install link pre
    const matches = screen.getAllByText(/\/mcp/)
    expect(matches.length).toBeGreaterThan(0)
  })

  it('renders the "Copy JSON snippet" button', () => {
    render(<MCPConfigPanel apiKey="k" />)
    expect(screen.getByRole('button', { name: /copy json snippet/i })).toBeInTheDocument()
  })

  it('renders the "Copy install link" button', () => {
    render(<MCPConfigPanel apiKey="k" />)
    expect(screen.getByRole('button', { name: /copy install link/i })).toBeInTheDocument()
  })

  it('renders the "Open in Cursor" link', () => {
    render(<MCPConfigPanel apiKey="k" />)
    expect(screen.getByRole('link', { name: /open in cursor/i })).toBeInTheDocument()
  })

  it('copies the JSON snippet to clipboard when the button is clicked', async () => {
    render(<MCPConfigPanel apiKey="axon_k_xyz" />)
    await userEvent.click(screen.getByRole('button', { name: /copy json snippet/i }))
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      expect.stringContaining('axon_k_xyz'),
    )
  })

  it('changes the JSON copy button label to "Copied JSON" after clicking', async () => {
    render(<MCPConfigPanel apiKey="k" />)
    await userEvent.click(screen.getByRole('button', { name: /copy json snippet/i }))
    expect(await screen.findByRole('button', { name: /copied json/i })).toBeInTheDocument()
  })

  it('copies the install link to clipboard when the button is clicked', async () => {
    render(<MCPConfigPanel apiKey="k" />)
    await userEvent.click(screen.getByRole('button', { name: /copy install link/i }))
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(expect.stringContaining('cursor://'))
  })

  it('changes the link copy button label to "Copied link" after clicking', async () => {
    render(<MCPConfigPanel apiKey="k" />)
    await userEvent.click(screen.getByRole('button', { name: /copy install link/i }))
    expect(await screen.findByRole('button', { name: /copied link/i })).toBeInTheDocument()
  })

  it('shows a security warning about the API key in the install link', () => {
    render(<MCPConfigPanel apiKey="k" />)
    expect(screen.getByText(/security/i)).toBeInTheDocument()
  })
})
