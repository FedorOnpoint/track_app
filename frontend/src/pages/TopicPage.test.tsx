import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, expect, test, vi } from 'vitest'

import { appRoutes } from '../app/router'

afterEach(() => { cleanup(); vi.restoreAllMocks() })

test('renders complete topic content with semantic navigation', async () => {
  mockTopic(topicFixture())

  renderTopic()

  await waitFor(() => expect(screen.getByRole('heading', { name: 'Горутины' })).toBeInTheDocument())
  expect(screen.getByRole('navigation', { name: 'Хлебные крошки' })).toBeInTheDocument()
  expect(screen.getByRole('navigation', { name: 'Содержание' })).toBeInTheDocument()
  expect(screen.getByRole('article')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'Планировщик' })).toBeInTheDocument()
  expect(screen.getByText('Что такое goroutine?')).toBeInTheDocument()
  expect(screen.getByText('Что произойдёт при блокировке?')).toBeInTheDocument()
  expect(screen.getByText('Написать worker pool')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /Go memory model/ })).toHaveAttribute('href', 'https://go.dev/ref/mem')
})

test('does not render raw HTML or unsafe markdown and resource links', async () => {
  const fixture = topicFixture()
  fixture.sections[0].body_markdown = '[Документация](https://go.dev/doc)\n\n[Опасная ссылка](javascript:alert(1))\n\n<img src=x onerror="alert(1)">'
  fixture.resources.push({ id: 'r2', title: 'Опасный ресурс', url: 'javascript:alert(1)', resource_type: 'article', source_name: 'Unknown', sort_order: 2 })
  mockTopic(fixture)

  renderTopic()

  const article = await screen.findByRole('article')
  expect(within(article).getByRole('link', { name: 'Документация' })).toHaveAttribute('href', 'https://go.dev/doc')
  expect(within(article).queryByRole('link', { name: 'Опасная ссылка' })).not.toBeInTheDocument()
  expect(within(article).queryByRole('link', { name: /Опасный ресурс/ })).not.toBeInTheDocument()
  expect(within(article).queryByRole('img')).not.toBeInTheDocument()
  expect(article.innerHTML).not.toContain('onerror')
  expect(article.innerHTML).not.toContain('javascript:')
})

test('renders a safe retryable topic error', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => String(input).includes('/auth/me')
    ? sessionResponse()
    : new Response(JSON.stringify({ error: { code: 'topic_not_found', message: 'Тема не найдена', request_id: 'r1' } }), { status: 404 })))

  renderTopic('/topics/missing')

  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Не удалось загрузить тему'))
  expect(screen.queryByText('topic_not_found')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
})

function mockTopic(data: ReturnType<typeof topicFixture>) {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => String(input).includes('/auth/me')
    ? sessionResponse()
    : new Response(JSON.stringify({ data }))))
}

function sessionResponse() {
  return new Response(JSON.stringify({ data: { id: 'u1', login: 'fedor' } }))
}

function renderTopic(path = '/topics/goroutines') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(appRoutes, { initialEntries: [path] })
  render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)
}

function topicFixture() {
  return {
    id: 't1', slug: 'goroutines', title: 'Горутины', summary: 'Лёгкие потоки выполнения', difficulty: 'intermediate' as const, estimated_minutes: 35, category_slug: 'concurrency', category_title: 'Конкурентность',
    sections: [{ id: 's1', heading: 'Планировщик', body_markdown: '**GMP** распределяет работу.', sort_order: 1 }],
    checklist: [{ id: 'c1', stable_key: 'explain-gmp', text: 'Объяснить модель GMP', sort_order: 1 }],
    questions: [{ id: 'q1', stable_key: 'what-is-goroutine', question: 'Что такое goroutine?', short_answer: 'Лёгкая единица выполнения.', detailed_answer: 'Планируется runtime.', pitfalls: 'Не является системным потоком.', sort_order: 1 }],
    follow_ups: [{ id: 'f1', interview_question_id: 'q1', question: 'Что произойдёт при блокировке?', expected_answer: 'Runtime перепланирует работу.', sort_order: 1 }],
    resources: [{ id: 'r1', title: 'Go memory model', url: 'https://go.dev/ref/mem', resource_type: 'documentation', source_name: 'Go', sort_order: 1 }],
    practice: [{ id: 'p1', title: 'Написать worker pool', description: 'Ограничить число **worker**.', url: 'https://go.dev/', expected_outcome: 'Нет утечек', sort_order: 1 }],
  }
}
