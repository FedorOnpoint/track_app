import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, expect, test, vi } from 'vitest'
import { AuthLayout } from '../app/auth'
import { RegisterPage } from './RegisterPage'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function setup(failure = false) {
  const fetch = vi.fn(async (input: RequestInfo | URL) => {
    if (String(input).endsWith('/auth/register')) return failure
      ? new Response(JSON.stringify({ error: { code: 'login_unavailable', message: 'Этот логин уже занят. Выберите другой' } }), { status: 409 })
      : new Response(JSON.stringify({ data: { id: 'new-user', login: 'alice' } }), { status: 201 })
    return new Response(JSON.stringify({ error: { code: 'unauthorized', message: 'Требуется вход' } }), { status: 401 })
  })
  vi.stubGlobal('fetch', fetch)
  const router = createMemoryRouter([{ Component: AuthLayout, children: [{ path: '/register', Component: RegisterPage }, { path: '/', element: <h1>Обзор подготовки</h1> }] }], { initialEntries: ['/register'] })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
  return { router, client, fetch }
}

async function fill() {
  await screen.findByLabelText('Логин')
  fireEvent.change(screen.getByLabelText('Логин'), { target: { value: 'alice' } })
  fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'safe-password-123' } })
  fireEvent.change(screen.getByLabelText('Повторите пароль'), { target: { value: 'safe-password-123' } })
  fireEvent.change(screen.getByLabelText('Код приглашения'), { target: { value: 'a'.repeat(43) } })
}

test('invited registration establishes the session and opens the dashboard', async () => {
  const { router, client, fetch } = setup()
  await fill()
  fireEvent.click(screen.getByRole('button', { name: 'Создать аккаунт' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  expect(client.getQueryData(['session'])).toEqual({ id: 'new-user', login: 'alice' })
  const registration = fetch.mock.calls.find(([url]) => String(url).endsWith('/auth/register'))
  expect(registration).toBeDefined()
})

test('password mismatch never submits registration', async () => {
  const { fetch } = setup()
  await fill()
  fireEvent.change(screen.getByLabelText('Повторите пароль'), { target: { value: 'another-password' } })
  fireEvent.click(screen.getByRole('button', { name: 'Создать аккаунт' }))
  await screen.findByText('Пароли не совпадают')
  expect(fetch.mock.calls.some(([url]) => String(url).endsWith('/auth/register'))).toBe(false)
})

test('server validation stays on registration and preserves the invitation', async () => {
  const { router } = setup(true)
  await fill()
  fireEvent.click(screen.getByRole('button', { name: 'Создать аккаунт' }))
  await screen.findByRole('alert')
  expect(screen.getByRole('alert')).toHaveTextContent('Этот логин уже занят')
  expect(router.state.location.pathname).toBe('/register')
  expect(screen.getByLabelText('Код приглашения')).toHaveValue('a'.repeat(43))
})
