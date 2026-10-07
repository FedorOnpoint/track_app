import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { TopicNotes } from './TopicNotes'

afterEach(() => { cleanup(); vi.restoreAllMocks() })

test('creates, edits and confirms deletion with keyboard cancellation', async () => {
  let notes = [note('n1', 'Исходная заметка')]
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url.endsWith('/notes') && !init?.method) return envelope(notes)
    if (init?.method === 'POST') {
      const created = note('n2', bodyFrom(init))
      notes = [created, ...notes]
      return envelope(created, 201)
    }
    if (init?.method === 'PUT') {
      const updated = note('n1', bodyFrom(init))
      notes = notes.map((item) => item.id === 'n1' ? updated : item)
      return envelope(updated)
    }
    if (init?.method === 'DELETE') {
      notes = notes.filter((item) => item.id !== 'n1')
      return new Response(null, { status: 204 })
    }
    return new Response('', { status: 404 })
  }))
  renderNotes()
  await screen.findByText('Исходная заметка')

  fireEvent.click(screen.getByRole('button', { name: 'Редактировать' }))
  fireEvent.change(screen.getByLabelText('Текст заметки'), { target: { value: 'Обновлённая заметка' } })
  fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
  await screen.findByText('Обновлённая заметка')

  fireEvent.change(screen.getByLabelText('Новая заметка'), { target: { value: 'Новая запись' } })
  fireEvent.click(screen.getByRole('button', { name: 'Добавить заметку' }))
  await screen.findByText('Новая запись')

  const updatedItem = screen.getByText('Обновлённая заметка').closest('li')
  expect(updatedItem).not.toBeNull()
  fireEvent.click(within(updatedItem!).getByRole('button', { name: 'Удалить' }))
  const dialog = screen.getByRole('alertdialog', { name: 'Удалить заметку?' })
  expect(within(dialog).getByRole('button', { name: 'Да, удалить' })).toHaveFocus()
  fireEvent.keyDown(dialog, { key: 'Escape' })
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  expect(screen.getByText('Обновлённая заметка')).toBeInTheDocument()

  fireEvent.click(within(updatedItem!).getByRole('button', { name: 'Удалить' }))
  fireEvent.click(screen.getByRole('button', { name: 'Да, удалить' }))
  await waitFor(() => expect(screen.queryByText('Обновлённая заметка')).not.toBeInTheDocument())
})

test('restores the exact note after an optimistic delete failure', async () => {
  const pending = deferred<Response>()
  vi.stubGlobal('fetch', vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === 'DELETE') return pending.promise
    return envelope([note('n1', '**Важная** заметка')])
  }))
  renderNotes()
  const text = await screen.findByText('Важная')
  const item = text.closest('li')
  fireEvent.click(within(item!).getByRole('button', { name: 'Удалить' }))
  fireEvent.click(screen.getByRole('button', { name: 'Да, удалить' }))
  await waitFor(() => expect(screen.queryByText('Важная')).not.toBeInTheDocument())

  pending.resolve(new Response('', { status: 503 }))

  expect(await screen.findByText('Важная')).toBeInTheDocument()
  expect(screen.getByRole('alert')).toHaveTextContent('Предыдущее состояние восстановлено')
})

function renderNotes() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={queryClient}><TopicNotes slug="goroutines" /></QueryClientProvider>)
}

function note(id: string, body: string) {
  return { id, topic_id: 't1', topic_slug: 'goroutines', topic_title: 'Горутины', body_markdown: body, created_at: '2026-10-07T00:00:00Z', updated_at: '2026-10-07T00:00:00Z' }
}

function envelope(data: object, status = 200) {
  return new Response(JSON.stringify({ data }), { status })
}

function bodyFrom(init: RequestInit) {
  return (JSON.parse(String(init.body)) as { body_markdown: string }).body_markdown
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}
