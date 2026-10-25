import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { API_KEY_STORAGE } from '../types'
import { ConnectScreen } from './ConnectScreen'

const makeSessionStorage = () => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = value
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
}

const sessionStorageMock = makeSessionStorage()
Object.defineProperty(window, 'sessionStorage', {
  value: sessionStorageMock,
  writable: true,
})

describe('ConnectScreen', () => {
  beforeEach(() => sessionStorageMock.clear())

  it('renders the operator key input', () => {
    render(<ConnectScreen onConnect={vi.fn()} />)
    expect(screen.getByPlaceholderText(/axon_o_/i)).toBeInTheDocument()
  })

  it('renders the Connect button', () => {
    render(<ConnectScreen onConnect={vi.fn()} />)
    expect(screen.getByRole('button', { name: /connect/i })).toBeInTheDocument()
  })

  it('shows a validation error when the key is empty on submit', async () => {
    render(<ConnectScreen onConnect={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: /connect/i }))
    expect(screen.getByText(/enter your operator key/i)).toBeInTheDocument()
  })

  it('does not call onConnect when the key is empty', async () => {
    const onConnect = vi.fn()
    render(<ConnectScreen onConnect={onConnect} />)
    await userEvent.click(screen.getByRole('button', { name: /connect/i }))
    expect(onConnect).not.toHaveBeenCalled()
  })

  it('calls onConnect with the trimmed key on valid submit', async () => {
    const onConnect = vi.fn()
    render(<ConnectScreen onConnect={onConnect} />)
    await userEvent.type(screen.getByPlaceholderText(/axon_o_/i), 'axon_o_abc123')
    await userEvent.click(screen.getByRole('button', { name: /connect/i }))
    expect(onConnect).toHaveBeenCalledWith('axon_o_abc123')
  })

  it('pre-fills the input from sessionStorage', () => {
    sessionStorageMock.setItem(API_KEY_STORAGE, 'axon_o_stored')
    render(<ConnectScreen onConnect={vi.fn()} />)
    expect(screen.getByPlaceholderText(/axon_o_/i)).toHaveValue('axon_o_stored')
  })

  it('saves the key to sessionStorage on connect', async () => {
    render(<ConnectScreen onConnect={vi.fn()} />)
    await userEvent.type(screen.getByPlaceholderText(/axon_o_/i), 'axon_o_new')
    await userEvent.click(screen.getByRole('button', { name: /connect/i }))
    expect(sessionStorageMock.getItem(API_KEY_STORAGE)).toBe('axon_o_new')
  })
})
