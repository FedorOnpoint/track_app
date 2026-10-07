import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, expect, test, vi } from 'vitest'
import { appRoutes } from '../app/router'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

test('wrong current password stays on settings and logout-all redirects to login', async () => {
  let authenticated = true
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const path = String(input)
    if (path.endsWith('/auth/password')) return new Response(JSON.stringify({ error: { code: 'invalid_credentials', message: 'Неверный текущий пароль' } }), { status: 401 })
    if (path.endsWith('/auth/logout-all')) { authenticated = false; return new Response(null, { status: 204 }) }
    return authenticated ? new Response(JSON.stringify({ data: { id: 'u1', login: 'fedor' } })) : new Response('', { status: 401 })
  }))
  const router = createMemoryRouter(appRoutes, { initialEntries: ['/settings'] })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
  await screen.findByLabelText('Текущий пароль')
  fireEvent.change(screen.getByLabelText('Текущий пароль'), { target: { value: 'wrong-password' } })
  fireEvent.change(screen.getByLabelText('Новый пароль'), { target: { value: 'new-password-123' } })
  fireEvent.change(screen.getByLabelText('Повторите новый пароль'), { target: { value: 'new-password-123' } })
  fireEvent.click(screen.getByRole('button', { name: 'Сменить пароль' }))
  await screen.findByText('Неверный текущий пароль')
  expect(router.state.location.pathname).toBe('/settings')
  fireEvent.click(screen.getByRole('button', { name: 'Выйти на всех устройствах' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
})
