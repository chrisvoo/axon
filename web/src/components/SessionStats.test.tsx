import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SessionStats } from './SessionStats'

describe('SessionStats', () => {
  it('shows "Connected" dot when connected is true', () => {
    render(<SessionStats connected requests={0} errors={0} since={null} />)
    expect(screen.getByText('Connected')).toBeInTheDocument()
  })

  it('shows "Reconnecting…" when not connected', () => {
    render(<SessionStats connected={false} requests={0} errors={0} since={null} />)
    expect(screen.getByText('Reconnecting…')).toBeInTheDocument()
  })

  it('shows agent version with "v" prefix when provided', () => {
    render(<SessionStats connected version="1.2.3" requests={0} errors={0} since={null} />)
    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
  })

  it('hides the agent version row when not provided', () => {
    render(<SessionStats connected={false} requests={0} errors={0} since={null} />)
    expect(screen.queryByText(/^v\d/)).not.toBeInTheDocument()
  })

  it('renders the tool call count', () => {
    render(<SessionStats connected requests={42} errors={0} since={null} />)
    expect(screen.getByText('42')).toBeInTheDocument()
  })

  it('renders the error count', () => {
    render(<SessionStats connected requests={0} errors={7} since={null} />)
    expect(screen.getByText('7')).toBeInTheDocument()
  })

  it('shows the "Since" row when a timestamp is provided', () => {
    render(<SessionStats connected requests={0} errors={0} since={Date.now()} />)
    expect(screen.getByText('Since')).toBeInTheDocument()
  })

  it('hides the "Since" row when since is null', () => {
    render(<SessionStats connected requests={0} errors={0} since={null} />)
    expect(screen.queryByText('Since')).not.toBeInTheDocument()
  })
})
