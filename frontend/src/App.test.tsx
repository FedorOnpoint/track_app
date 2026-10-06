import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { App } from './App'

afterEach(() => { cleanup(); vi.restoreAllMocks() })

test('renders catalog topics returned by the API', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/auth/me')) return new Response(JSON.stringify({ data: { id: 'u1', login: 'fedor' } }))
    if (url.includes('/categories')) {
      return new Response(JSON.stringify({ data: [{ id: '1', slug: 'concurrency', title: 'Конкурентность', description: 'Горутины и каналы' }] }))
    }
    return new Response(JSON.stringify({
      data: [{ id: '2', slug: 'goroutines', title: 'Горутины и планировщик', summary: 'GMP и утечки', difficulty: 'intermediate', estimated_minutes: 35, category_slug: 'concurrency', category_title: 'Конкурентность' }],
      pagination: { page: 1, page_size: 20, total_items: 1, total_pages: 1 },
    }))
  }))

  render(<App />)

  await waitFor(() => expect(screen.getByRole('heading', { name: 'Карта знаний' })).toBeInTheDocument())
  await waitFor(() => expect(screen.getByText('Горутины и планировщик')).toBeInTheDocument())
  expect(screen.getByText('1 тема')).toBeInTheDocument()
})

test('shows a useful error state', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => String(input).includes('/auth/me')
    ? new Response(JSON.stringify({ data: { id: 'u1', login: 'fedor' } }))
    : new Response('', { status: 503 })))
  render(<App />)
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Не удалось загрузить каталог'))
})

test('authenticates before opening the protected catalog', async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/auth/me')) return new Response('', { status: 401 })
    if (url.includes('/auth/login')) return new Response(JSON.stringify({ data: { id: 'u1', login: 'fedor' } }))
    if (url.includes('/categories')) return new Response(JSON.stringify({ data: [] }))
    return new Response(JSON.stringify({ data: [], pagination: { page: 1, page_size: 20, total_items: 0, total_pages: 0 } }))
  })
  vi.stubGlobal('fetch', fetchMock)
  render(<App />)
  await waitFor(() => expect(screen.getByRole('heading', { name: 'С возвращением' })).toBeInTheDocument())
  fireEvent.change(screen.getByLabelText('Логин'), { target: { value: 'fedor' } })
  fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'secret-password' } })
  fireEvent.click(screen.getByRole('button', { name: 'Войти' }))
  await waitFor(() => expect(screen.getByRole('heading', { name: 'Карта знаний' })).toBeInTheDocument())
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/auth/login', expect.objectContaining({ method: 'POST' }))
})

test('returns to login when any API request receives a 401', async () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/auth/me')) return new Response(JSON.stringify({ data: { id: 'u1', login: 'fedor' } }))
    return new Response(JSON.stringify({
      error: { code: 'unauthorized', message: 'Требуется вход', request_id: 'request-1' },
    }), { status: 401 })
  }))

  render(<App />)

  await waitFor(() => expect(screen.getByRole('heading', { name: 'С возвращением' })).toBeInTheDocument())
})
