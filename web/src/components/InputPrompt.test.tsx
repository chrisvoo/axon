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

  it('renders the stdin textarea', () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    expect(screen.getByRole('textbox')).toBeInTheDocument()
  })

  it('renders Send and Cancel buttons', () => {
    render(<InputPrompt processId="p1" apiKey="k" />)
    expect(screen.getByRole('button', { name: /send/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument()
  })

  it('calls /api/input with the typed value on Send', async () => {
    render(<InputPrompt processId="proc-42" apiKey="test_key" />)
    await userEvent.type(screen.getByRole('textbox'), 'my-password')
    await userEvent.click(screen.getByRole('button', { name: /send/i }))
    expect(api.apiPostJson).toHaveBeenCalledWith('/api/input', 'test_key', {
      process_id: 'proc-42',
      data: 'my-password',
    })
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
