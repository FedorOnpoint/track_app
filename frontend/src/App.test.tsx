import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { App } from './App'

afterEach(() => vi.restoreAllMocks())

test('renders catalog topics returned by the API', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/categories')) {
      return new Response(JSON.stringify({ data: [{ id: '1', slug: 'concurrency', title: 'Конкурентность', description: 'Горутины и каналы' }] }))
    }
    return new Response(JSON.stringify({
      data: [{ id: '2', slug: 'goroutines', title: 'Горутины и планировщик', summary: 'GMP и утечки', difficulty: 'intermediate', estimated_minutes: 35, category_slug: 'concurrency', category_title: 'Конкурентность' }],
      pagination: { page: 1, page_size: 20, total_items: 1, total_pages: 1 },
    }))
  }))

  render(<App />)

  expect(screen.getByRole('heading', { name: 'Карта знаний' })).toBeInTheDocument()
  await waitFor(() => expect(screen.getByText('Горутины и планировщик')).toBeInTheDocument())
  expect(screen.getByText('1 тема')).toBeInTheDocument()
})

test('shows a useful error state', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 503 })))
  render(<App />)
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Не удалось загрузить каталог'))
})
