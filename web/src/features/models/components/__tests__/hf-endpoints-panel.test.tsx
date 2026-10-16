import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
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

async function renderPanel() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const rootRoute = createRootRoute()
  const modelsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/_authenticated/models/$section',
    component: () => (
      <I18nextProvider i18n={i18n}>
        <HfEndpointsPanel />
      </I18nextProvider>
    ),
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([modelsRoute]),
    history: createMemoryHistory({
      initialEntries: ['/_authenticated/models/deployments'],
    }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
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
      {
        name: 'other-ep',
        namespace: 'acme',
        status: { state: 'paused' },
        model: { repository: 'org/other' },
      },
    ],
  })
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('HfEndpointsPanel', () => {
  test('lists endpoints in the table', async () => {
    renderPanel()
    expect(await screen.findByText('acme/my-llm')).toBeInTheDocument()
    expect(screen.getByText(/meta-llama\/Llama-3-8B/)).toBeInTheDocument()
  })

  test('search filters the table', async () => {
    renderPanel()
    await screen.findByText('acme/my-llm')
    fireEvent.change(screen.getByPlaceholderText('Search deployments...'), {
      target: { value: 'other-ep' },
    })
    await waitFor(() => {
      expect(screen.queryByText('acme/my-llm')).not.toBeInTheDocument()
    })
    expect(screen.getByText('acme/other-ep')).toBeInTheDocument()
  })

  test('create opens a dialog and stays disabled until required fields are filled', async () => {
    renderPanel()
    await screen.findByText('acme/my-llm')

    fireEvent.click(screen.getByRole('button', { name: 'Create endpoint' }))
    const dialog = await screen.findByRole('dialog')
    const dialogButton = within(dialog).getByRole('button', {
      name: 'Create endpoint',
    })
    expect(dialogButton).toBeDisabled()

    fireEvent.change(within(dialog).getByPlaceholderText('my-llm'), {
      target: { value: 'new-ep' },
    })
    fireEvent.change(
      within(dialog).getByPlaceholderText('meta-llama/Llama-3-8B'),
      { target: { value: 'org/model' } }
    )
    expect(dialogButton).not.toBeDisabled()

    vi.mocked(createHfEndpoint).mockResolvedValue({ success: true })
    fireEvent.click(dialogButton)
    await waitFor(() => {
      expect(createHfEndpoint).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'new-ep',
          repository: 'org/model',
        })
      )
    })
  })

  test('delete asks for confirmation and calls the API with the namespace', async () => {
    vi.mocked(deleteHfEndpoint).mockResolvedValue({ success: true })
    renderPanel()
    await screen.findByText('acme/my-llm')

    const menus = screen.getAllByRole('button', { name: 'Open menu' })
    fireEvent.click(menus[0])
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Delete' }))
    expect(deleteHfEndpoint).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => {
      expect(deleteHfEndpoint).toHaveBeenCalledWith('my-llm', 'acme')
    })
  })
})
