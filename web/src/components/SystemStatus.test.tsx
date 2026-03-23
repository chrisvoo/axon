import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SystemStatus } from './SystemStatus'

describe('SystemStatus', () => {
  it('shows a loading message while stats are null', () => {
    render(<SystemStatus stats={null} />)
    expect(screen.getByText(/collecting system metrics/i)).toBeInTheDocument()
  })

  it('renders hostname and OS details', () => {
    render(
      <SystemStatus
        stats={{
          hostname: 'my-server',
          os: 'linux',
          platform: 'ubuntu',
          platform_ver: '22.04',
        }}
      />,
    )
    expect(screen.getByText('my-server')).toBeInTheDocument()
    expect(screen.getByText(/ubuntu/)).toBeInTheDocument()
  })

  it('renders a memory usage bar with the correct percentage', () => {
    render(<SystemStatus stats={{ mem_used_pct: 75 }} />)
    expect(screen.getByText('Memory')).toBeInTheDocument()
    expect(screen.getByText('75%')).toBeInTheDocument()
  })

  it('clamps the bar percentage to 100 when the value exceeds it', () => {
    render(<SystemStatus stats={{ mem_used_pct: 120 }} />)
    expect(screen.getByText('100%')).toBeInTheDocument()
  })

  it('clamps the bar percentage to 0 when the value is negative', () => {
    render(<SystemStatus stats={{ mem_used_pct: -5 }} />)
    expect(screen.getByText('0%')).toBeInTheDocument()
  })

  it('renders the first disk usage bar', () => {
    render(
      <SystemStatus
        stats={{
          disk: [{ mountpoint: '/', total_gb: '500.00', used_pct: 40 }],
        }}
      />,
    )
    expect(screen.getByText('Disk /')).toBeInTheDocument()
    expect(screen.getByText('40%')).toBeInTheDocument()
  })

  it('shows CPU core count when provided', () => {
    render(<SystemStatus stats={{ cpu_cores: 8 }} />)
    expect(screen.getByText('8')).toBeInTheDocument()
  })

  it('shows total RAM when provided', () => {
    render(<SystemStatus stats={{ mem_total_gb: '32.00' }} />)
    expect(screen.getByText(/32\.00/)).toBeInTheDocument()
  })
})
