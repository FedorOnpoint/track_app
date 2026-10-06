import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, expect, test, vi } from 'vitest'

import { appRoutes } from '../app/router'

afterEach(() => { cleanup(); vi.restoreAllMocks() })

test('renders aggregate, category, review, recent and continuation data', async () => {
  mockDashboard({
    overall: { total: 12, mastered: 5, percent: 41.6667 },
    categories: [{ slug: 'go-core', title: 'Go Core', total: 4, mastered: 2, percent: 50 }],
    review_topics: [topic('interfaces', 'Интерфейсы', 'review')],
    recent_topics: [topic('goroutines', 'Горутины', 'learning')],
    recent_notes: [{ id: 'n1', topic_slug: 'context', topic_title: 'Context', body_markdown: 'Не забыть про cancel', updated_at: '2026-10-07T00:00:00Z' }],
    continue_topic: topic('goroutines', 'Горутины', 'learning'),
  })

  renderDashboard()

  await waitFor(() => expect(screen.getByRole('heading', { name: 'Обзор подготовки' })).toBeInTheDocument())
  expect(screen.getByLabelText('Освоено 42%')).toHaveTextContent('42%')
  expect(screen.getByText('Go Core')).toBeInTheDocument()
  expect(screen.getByText('Интерфейсы')).toBeInTheDocument()
  expect(screen.getByText('Не забыть про cancel')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /Открыть тему/ })).toHaveAttribute('href', '/topics/goroutines')
})

test('renders useful empty states and a catalog call to action', async () => {
  mockDashboard({ overall: { total: 0, mastered: 0, percent: 0 }, categories: [], review_topics: [], recent_topics: [], recent_notes: [], continue_topic: null })

  renderDashboard()

  await waitFor(() => expect(screen.getByRole('heading', { name: 'Выберите первую тему' })).toBeInTheDocument())
  expect(screen.getByText('Каталог пока пуст.')).toBeInTheDocument()
  expect(screen.getByText('Тем для повторения пока нет.')).toBeInTheDocument()
  expect(screen.getByText('Вы ещё не открывали темы.')).toBeInTheDocument()
  expect(screen.getByText('Заметок пока нет.')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /Открыть карту знаний/ })).toHaveAttribute('href', '/topics')
})

test('renders a retryable dashboard error', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => String(input).includes('/auth/me')
    ? sessionResponse()
    : new Response('', { status: 503 })))

  renderDashboard()

  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Не удалось загрузить главную'))
  expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
})

test('announces dashboard loading while its request is pending', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    if (String(input).includes('/auth/me')) return sessionResponse()
    return new Promise<Response>(() => undefined)
  }))

  renderDashboard()

  expect(await screen.findByText('Загружаем ваш прогресс…')).toBeInTheDocument()
})

function mockDashboard(data: object) {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => String(input).includes('/auth/me')
    ? sessionResponse()
    : new Response(JSON.stringify({ data }))))
}

function sessionResponse() {
  return new Response(JSON.stringify({ data: { id: 'u1', login: 'fedor' } }))
}

function topic(slug: string, title: string, status: string) {
  return { slug, title, status, review_required: status === 'review', updated_at: '2026-10-07T00:00:00Z', last_opened_at: '2026-10-07T00:00:00Z' }
}

function renderDashboard() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(appRoutes, { initialEntries: ['/'] })
  render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)
}
