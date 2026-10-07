import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, expect, test, vi } from 'vitest'
import { TopicListPage } from './TopicListPage'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

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
