import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../api'
import { InputPrompt } from './InputPrompt'

vi.mock('../api')

describe('InputPrompt', () => {
  beforeEach(() => {
    vi.mocked(api.apiPostJson).mockResolvedValue({})
  })

  it('renders the interactive-input warning banner', () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    expect(screen.getByText(/command needs interactive input/i)).toBeInTheDocument()
  })

  it('renders the hint when provided', () => {
    render(<InputPrompt processId="p1" apiKey="k" hint="Enter your sudo password" />)
    expect(screen.getByText('Enter your sudo password')).toBeInTheDocument()
  })

  it('does not render a hint paragraph when hint is omitted', () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    expect(screen.queryByText(/enter your sudo/i)).not.toBeInTheDocument()
  })

  it('masks the input by default', () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    const field = screen.getByLabelText('stdin')
    expect(field).toHaveAttribute('type', 'password')
    expect(field).toHaveAttribute('autocomplete', 'new-password')
    expect(screen.getByRole('checkbox', { name: /secret/i })).toBeChecked()
  })

  it('shows a plain textarea when Secret is cleared', async () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    await userEvent.click(screen.getByRole('checkbox', { name: /secret/i }))
    expect(screen.getByRole('textbox')).toBeInTheDocument()
    expect(screen.getByLabelText('stdin').tagName).toBe('TEXTAREA')
  })

  it('clears the typed text when the mode changes', async () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    await userEvent.type(screen.getByLabelText('stdin'), 'my-password')
    await userEvent.click(screen.getByRole('checkbox', { name: /secret/i }))
    expect(screen.getByLabelText('stdin')).toHaveValue('')
  })

  it('renders Send and Cancel buttons', () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    expect(screen.getByRole('button', { name: /send/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument()
  })

  it('sends a secret with a trailing newline and the secret flag by default', async () => {
    render(<InputPrompt processId="proc-42" apiKey="test_key" />)
    await userEvent.type(screen.getByLabelText('stdin'), 'my-password')
    await userEvent.click(screen.getByRole('button', { name: /send/i }))
    expect(api.apiPostJson).toHaveBeenCalledWith('/api/input', 'test_key', {
      process_id: 'proc-42',
      data: 'my-password\n',
      secret: true,
    })
  })

  it('sends a plain response as typed, without the secret flag', async () => {
    render(<InputPrompt processId="proc-42" apiKey="test_key" />)
    await userEvent.click(screen.getByRole('checkbox', { name: /secret/i }))
    await userEvent.type(screen.getByRole('textbox'), 'yes{Enter}')
    await userEvent.click(screen.getByRole('button', { name: /send/i }))
    expect(api.apiPostJson).toHaveBeenCalledWith('/api/input', 'test_key', {
      process_id: 'proc-42',
      data: 'yes\n',
      secret: false,
    })
  })

  it('does not keep the secret in the page after sending', async () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    await userEvent.type(screen.getByLabelText('stdin'), 'my-password')
    await userEvent.click(screen.getByRole('button', { name: /send/i }))
    expect(await screen.findByText(/input sent/i)).toBeInTheDocument()
    expect(screen.queryByDisplayValue('my-password')).not.toBeInTheDocument()
    expect(document.body.textContent).not.toContain('my-password')
  })

  it('shows the "Input sent" confirmation after a successful send', async () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    await userEvent.click(screen.getByRole('button', { name: /send/i }))
    expect(await screen.findByText(/input sent/i)).toBeInTheDocument()
  })

  it('shows an error message when the send fails', async () => {
    vi.mocked(api.apiPostJson).mockRejectedValueOnce(new Error('Network error'))
    render(<InputPrompt processId="p1" apiKey="k" />)
    await userEvent.click(screen.getByRole('button', { name: /send/i }))
    expect(await screen.findByText(/network error/i)).toBeInTheDocument()
  })

  it('calls /api/cancel with the process ID on Cancel', async () => {
    render(<InputPrompt processId="proc-99" apiKey="test_key" />)
    await userEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(api.apiPostJson).toHaveBeenCalledWith('/api/cancel', 'test_key', {
      process_id: 'proc-99',
    })
  })

  it('shows an error message when the cancel fails', async () => {
    vi.mocked(api.apiPostJson).mockRejectedValueOnce(new Error('Failed to cancel'))
    render(<InputPrompt processId="p1" apiKey="k" />)
    await userEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(await screen.findByText(/failed to cancel/i)).toBeInTheDocument()
  })
})
