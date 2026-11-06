import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { SiteStatistics } from '../index'

const translate = (key: string) => key
vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: translate }) }))
vi.mock('../api', () => ({
  getSiteStatistics: vi.fn(),
  getSiteStatsGroupOptions: vi.fn(),
  searchSiteStatsOptions: vi.fn(),
}))
vi.mock('@/components/layout', () => {
  const Component = ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  )
  return {
    SectionPageLayout: Object.assign(Component, {
      Title: Component,
      Actions: Component,
      Content: Component,
    }),
  }
})
vi.mock('recharts', () => {
  const Component = ({ children }: { children?: React.ReactNode }) => (
    <div>{children}</div>
  )
  return Object.fromEntries(
    [
      'Area',
      'AreaChart',
      'Bar',
      'BarChart',
      'CartesianGrid',
      'Cell',
      'Legend',
      'Pie',
      'PieChart',
      'ResponsiveContainer',
      'Tooltip',
      'XAxis',
      'YAxis',
    ].map((name) => [name, Component])
  )
})
import {
  getSiteStatistics,
  getSiteStatsGroupOptions,
  searchSiteStatsOptions,
} from '../api'

const data = {
  range: {
    start: 1700000000,
    end: 1700001000,
    granularity: 'day' as const,
    bucket_size: 86400,
    tz_offset: 0,
    preset: '30d',
  },
  summary: {
    requests: 2,
    tokens: 200,
    prompt_tokens: 150,
    completion_tokens: 50,
    quota: 1000,
    unique_users: 2,
    avg_use_time_seconds: 1,
    byog_requests: 1,
    upstream_requests: 1,
    errors: 0,
    logins: 0,
    registrations: 0,
  },
  counters: {
    total_users: 2,
    active_users: 2,
    total_channels: 1,
    active_channels: 1,
    total_api_tokens: 0,
    active_api_tokens: 0,
    total_redemptions: 0,
    unused_redemptions: 0,
    byog_nodes_total: 0,
    byog_nodes_online: 0,
  },
  series: [
    {
      bucket: 1700000000,
      requests: 2,
      tokens: 200,
      prompt_tokens: 150,
      completion_tokens: 50,
      quota: 1000,
      unique_users: 2,
      byog_requests: 1,
      errors: 0,
      registrations: 0,
    },
  ],
  distributions: {
    hour_of_day: Array(24).fill(0),
    weekday: Array(7).fill(0),
    byog_device_types: [],
    byog_countries: [],
  },
  rankings: {
    users: [],
    models: [],
    channels: [],
    api_keys: [],
    clients: [],
    groups: [],
    ips: [],
    byog_nodes: [],
    log_types: [],
  },
}
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(getSiteStatistics).mockResolvedValue(data)
  vi.mocked(getSiteStatsGroupOptions).mockResolvedValue(['default'])
  vi.mocked(searchSiteStatsOptions).mockImplementation(async (kind) =>
    kind === 'user'
      ? [{ id: 17, name: 'alice' }]
      : [{ id: 42, name: 'Main channel' }]
  )
})
describe('statistics', () => {
  test('shows loading and every chart after the request', async () => {
    render(<SiteStatistics />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading...')
    await waitFor(() =>
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    )
    expect(screen.getAllByText('Traffic Over Time')).toHaveLength(7)
    expect(screen.getByText('BYOG Device Types')).toBeInTheDocument()
    expect(screen.getByText('BYOG Countries')).toBeInTheDocument()
    const controls = screen.getAllByRole('combobox')
    expect(controls.map((c) => c.textContent)).toEqual([
      'Last 30 Days',
      'Auto',
      'Requests',
      'Descending',
      '20',
      'All Groups',
    ])
  })
  test('selects named users and channels and submits their IDs', async () => {
    render(<SiteStatistics />)
    await waitFor(() =>
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByRole('button', { name: 'Include Users' }))
    const userOption = await screen.findByRole('button', { name: /alice/ })
    fireEvent.click(userOption)
    fireEvent.click(screen.getByRole('button', { name: 'alice' }))
    expect(screen.getByRole('button', { name: 'alice' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Channel' }))
    fireEvent.click(await screen.findByRole('button', { name: /Main channel/ }))
    expect(
      screen.getByRole('button', { name: /Main channel/ })
    ).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))
    await waitFor(() =>
      expect(getSiteStatistics).toHaveBeenLastCalledWith(
        expect.objectContaining({ user_ids: '17', channel_id: 42 })
      )
    )
  })
  test('shows separate client and API key rankings', async () => {
    vi.mocked(getSiteStatistics).mockResolvedValue({
      ...data,
      rankings: {
        ...data.rankings,
        api_keys: [
          { token_name: 'private-key-label', requests: 1, tokens: 2, quota: 3 },
        ],
        clients: [
          { client_name: 'Roo Code', requests: 1, tokens: 2, quota: 3 },
        ],
      },
    })
    render(<SiteStatistics />)
    expect(await screen.findByText('private-key-label')).toBeInTheDocument()
    expect(screen.getByText('Roo Code')).toBeInTheDocument()
    expect(screen.getByText('Clients')).toBeInTheDocument()
  })
  test('sends numeric search input to the statistics user lookup', async () => {
    render(<SiteStatistics />)
    await waitFor(() =>
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    )
    fireEvent.click(screen.getByRole('button', { name: 'Include Users' }))
    fireEvent.change(screen.getByRole('textbox', { name: 'Include Users' }), {
      target: { value: '17' },
    })
    await waitFor(() =>
      expect(searchSiteStatsOptions).toHaveBeenCalledWith('user', '17')
    )
    expect(
      await screen.findByRole('button', { name: /alice/ })
    ).toBeInTheDocument()
  })
  test('shows an error and allows a retry', async () => {
    vi.mocked(getSiteStatistics).mockRejectedValueOnce(new Error('failed'))
    render(<SiteStatistics />)
    const alert = await screen.findByRole('alert')
    fireEvent.click(within(alert).getByRole('button', { name: 'Retry' }))
    await waitFor(() =>
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    )
  })
})
