import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { HfEndpointsPanel } from '../hf-endpoints-panel'

const { I18nextProvider, initReactI18next } = await import('react-i18next')
const i18n = (await import('i18next')).default

await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {},
})

vi.mock('../../api', () => ({
  listHfEndpoints: vi.fn(),
  createHfEndpoint: vi.fn(),
  deleteHfEndpoint: vi.fn(),
}))

const { createHfEndpoint, deleteHfEndpoint, listHfEndpoints } = await import(
  '../../api'
)

function renderPanel() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <HfEndpointsPanel />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.mocked(listHfEndpoints).mockResolvedValue({
    success: true,
    data: [
      {
        name: 'my-llm',
        namespace: 'acme',
        status: { state: 'running' },
        model: { repository: 'meta-llama/Llama-3-8B' },
      },
    ],
  })
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('HfEndpointsPanel', () => {
  test('lists endpoints from the API', async () => {
    renderPanel()
    expect(await screen.findByText('acme/my-llm')).toBeInTheDocument()
    expect(screen.getByText(/meta-llama\/Llama-3-8B/)).toBeInTheDocument()
  })

  test('create stays disabled until name and repository are filled', async () => {
    renderPanel()
    await screen.findByText('acme/my-llm')
    const createButton = screen.getByRole('button', {
      name: 'Create endpoint',
    })
    expect(createButton).toBeDisabled()

    fireEvent.change(screen.getByPlaceholderText('my-llm'), {
      target: { value: 'new-ep' },
    })
    fireEvent.change(screen.getByPlaceholderText('meta-llama/Llama-3-8B'), {
      target: { value: 'org/model' },
    })
    expect(createButton).not.toBeDisabled()

    vi.mocked(createHfEndpoint).mockResolvedValue({ success: true })
    fireEvent.click(createButton)
    await waitFor(() => {
      expect(createHfEndpoint).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'new-ep',
          repository: 'org/model',
        })
      )
    })
  })

  test('delete confirms first and then calls the API', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderPanel()
    await screen.findByText('acme/my-llm')
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(confirmSpy).toHaveBeenCalled()
    expect(deleteHfEndpoint).not.toHaveBeenCalled()

    confirmSpy.mockReturnValue(true)
    vi.mocked(deleteHfEndpoint).mockResolvedValue({ success: true })
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => {
      expect(deleteHfEndpoint).toHaveBeenCalledWith('my-llm')
    })
    confirmSpy.mockRestore()
  })
})
