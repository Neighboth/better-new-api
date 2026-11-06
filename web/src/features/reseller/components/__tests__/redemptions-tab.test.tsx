import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { RedemptionsTab } from '../redemptions-tab'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('../../api', () => ({
  createResellerRedemption: vi.fn(),
  deleteResellerRedemption: vi.fn(),
  fetchResellerRedemptions: vi.fn().mockResolvedValue({ items: [], total: 0 }),
}))

describe('reseller code creation', () => {
  test('offers only balance when opening the creation dialog', async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={client}>
        <RedemptionsTab />
      </QueryClientProvider>
    )
    fireEvent.click(screen.getByRole('button', { name: 'New Codes' }))
    expect(
      await screen.findByLabelText('Amount per Code ($)')
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Requests' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Tokens' })
    ).not.toBeInTheDocument()
  })
})
