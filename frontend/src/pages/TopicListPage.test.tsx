import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, expect, test, vi } from 'vitest'
import { TopicListPage } from './TopicListPage'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

test('keeps status controls locked until the saved progress has been reloaded', async () => {
  let reload: ((response: Response) => void) | undefined
  let reads = 0
  vi.stubGlobal('fetch', vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === 'PUT') return new Response(JSON.stringify({ data: {} }))
    reads++
    if (reads > 1) return new Promise<Response>((resolve) => { reload = resolve })
    return topicListResponse('learning')
  }))
  renderList()
  const control = await screen.findByLabelText('Статус: Горутины')
  fireEvent.change(control, { target: { value: 'mastered' } })
  await waitFor(() => expect(reload).toBeDefined())
  expect(control).toBeDisabled()
  await act(async () => { reload?.(topicListResponse('mastered')) })
  await waitFor(() => expect(control).toBeEnabled())
  expect(control).toHaveValue('mastered')
})

test('preserves progress after a failed save and allows retry without clearing review', async () => {
  let writes = 0
  let status = 'learning'
  const saved: unknown[] = []
  vi.stubGlobal('fetch', vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === 'PUT') {
      const body = JSON.parse(String(init.body)) as { status: string; review_required: boolean }
      saved.push(body)
      if (++writes === 1) return new Response('', { status: 503 })
      status = body.status
      return new Response(JSON.stringify({ data: {} }))
    }
    return topicListResponse(status)
  }))
  renderList()
  const control = await screen.findByLabelText('Статус: Горутины')
  fireEvent.change(control, { target: { value: 'mastered' } })
  expect(await screen.findByRole('alert')).toHaveTextContent('Не удалось сохранить статус')
  expect(control).toHaveValue('learning')
  expect(control).toBeEnabled()
  fireEvent.change(control, { target: { value: 'mastered' } })
  await waitFor(() => expect(control).toHaveValue('mastered'))
  expect(saved).toEqual([{ status: 'mastered', review_required: true }, { status: 'mastered', review_required: true }])
})

function topicListResponse(status: string) {
  return new Response(JSON.stringify({ data: [{ id: 't1', slug: 'goroutines', title: 'Горутины', category_title: 'Go', difficulty: 'intermediate', status, review_required: true }], pagination: { page: 1, page_size: 20, total_items: 1, total_pages: 1 } }))
}

function renderList(initialEntry = '/?query=go', mode: 'track' | 'bookmarks' | 'review' = 'track') {
  const router = createMemoryRouter([{ path: '/', element: <TopicListPage mode={mode} /> }], { initialEntries: [initialEntry] })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
  return router
}

test('keeps search and status filters when paging and resets the page when filters change', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const page = Number(new URL(String(input), 'https://example.test').searchParams.get('page') ?? 1)
    return new Response(JSON.stringify({ data: [], pagination: { page, page_size: 20, total_items: 41, total_pages: 3 } }))
  }))
  const router = renderList('/?query=go&status=learning&page=2')
  await screen.findByText('2 / 3')
  fireEvent.click(screen.getByRole('button', { name: 'Далее' }))
  await waitFor(() => expect(router.state.location.search).toBe('?query=go&status=learning&page=3'))
  await screen.findByText('3 / 3')
  expect(screen.getByRole('button', { name: 'Далее' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('Статус'), { target: { value: 'mastered' } })
  await waitFor(() => expect(router.state.location.search).toBe('?query=go&status=mastered'))
  fireEvent.change(screen.getByRole('textbox', { name: 'Поиск по темам' }), { target: { value: '  channels  ' } })
  fireEvent.click(screen.getByRole('button', { name: 'Найти' }))
  await waitFor(() => expect(router.state.location.search).toBe('?query=channels&status=mastered'))
  fireEvent.click(screen.getByRole('button', { name: 'Сбросить фильтры' }))
  await waitFor(() => expect(router.state.location.search).toBe(''))
  expect(screen.getByRole('textbox', { name: 'Поиск по темам' })).toHaveValue('')
})

test('announces loading and allows recovery from a failed list request', async () => {
  let respond: ((response: Response) => void) | undefined
  let reads = 0
  vi.stubGlobal('fetch', vi.fn(async () => {
    if (++reads === 1) return new Promise<Response>((resolve) => { respond = resolve })
    return topicListResponse('learning')
  }))
  renderList()
  expect(await screen.findByRole('status')).toHaveTextContent('Загружаем темы')
  await act(async () => { respond?.(new Response('', { status: 503 })) })
  expect(await screen.findByRole('alert')).toHaveTextContent('Не удалось загрузить темы')
  fireEvent.click(screen.getByRole('button', { name: 'Повторить' }))
  expect(await screen.findByRole('link', { name: 'Горутины' })).toHaveAttribute('href', '/topics/goroutines')
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test.each(['track', 'bookmarks', 'review'] as const)('%s uses its own filters and saves the individual topic status', async (mode) => {
  const requests: string[] = []
  let status = 'learning'
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    requests.push(String(input))
    if (init?.method === 'PUT') {
      status = (JSON.parse(String(init.body)) as { status: string }).status
      return new Response(JSON.stringify({ data: {} }))
    }
    return new Response(JSON.stringify({ data: [{ id: 't1', slug: 'goroutines', title: 'Горутины', category_title: 'Go', difficulty: 'intermediate', status }], pagination: { page: 1, page_size: 20, total_items: 1, total_pages: 1 } }))
  }))
  const router = createMemoryRouter([{ path: '/', element: <TopicListPage mode={mode} /> }], { initialEntries: ['/?query=go'] })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
  await screen.findByRole('link', { name: 'Горутины' })
  expect(requests[0]).toContain(mode === 'review' ? '/review?' : '/topics?')
  if (mode === 'bookmarks') expect(requests[0]).toContain('bookmarked=true')
  fireEvent.change(screen.getByLabelText('Статус: Горутины'), { target: { value: 'mastered' } })
  await waitFor(() => expect(screen.getByLabelText('Статус: Горутины')).toHaveValue('mastered'))
  expect(requests).toContain('/api/v1/topics/goroutines/progress')
  fireEvent.click(screen.getByRole('button', { name: 'Сбросить фильтры' }))
  await waitFor(() => expect(router.state.location.search).toBe(''))
})
