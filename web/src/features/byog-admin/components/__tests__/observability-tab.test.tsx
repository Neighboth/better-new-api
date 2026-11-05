import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { ByogObservabilityTab } from '../observability-tab'

vi.mock('@/features/byog/api', () => ({
  getAdminObservabilitySummary: vi.fn(),
  getAdminCrashReports: vi.fn(),
  getAdminDownloadErrors: vi.fn(),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
import {
  getAdminCrashReports,
  getAdminDownloadErrors,
  getAdminObservabilitySummary,
} from '@/features/byog/api'

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(getAdminObservabilitySummary).mockResolvedValue({
    node_health: {
      total: 2,
      online: 1,
      stale: 1,
      offline: 0,
      paused: 0,
      banned: 0,
      downloading: 0,
      errors: 1,
    },
    crash_reports_24h: 1,
    crash_reports_7d: 1,
    crash_reports_total: 1,
    top_error_codes: [{ name: 'OOM', count: 1 }],
  })
  vi.mocked(getAdminCrashReports).mockResolvedValue({
    page: 1,
    page_size: 20,
    total: 1,
    items: [
      {
        id: 1,
        node_id: 'node-1',
        user_id: 1,
        username: 'alice',
        device_name: 'Laptop',
        error_code: 'OOM',
        error_message: 'out of memory',
        stack_trace: 'stack-line',
        model_id: 'model-a',
        recommendation: 'restart',
        memory_usage_mb: 512,
        created_at: 1700000000,
      },
    ],
  })
  vi.mocked(getAdminDownloadErrors).mockResolvedValue({ items: [] })
})

describe('BYOG observability', () => {
  test('shows health and crash details', async () => {
    render(<ByogObservabilityTab />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading...')
    expect(await screen.findByText('Laptop')).toBeInTheDocument()
    expect(screen.getByText('alice')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Details' }))
    expect(await screen.findByText('stack-line')).toBeInTheDocument()
    expect(screen.getByText(/restart/)).toBeInTheDocument()
  })
  test('shows failure and retries', async () => {
    vi.mocked(getAdminObservabilitySummary).mockRejectedValueOnce(
      new Error('failed'),
    )
    render(<ByogObservabilityTab />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() =>
      expect(screen.queryByRole('alert')).not.toBeInTheDocument(),
    )
    expect(await screen.findByText('Laptop')).toBeInTheDocument()
  })
})
