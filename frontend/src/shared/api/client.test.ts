import { afterEach, expect, test, vi } from 'vitest'

import { ApiError, apiRequest, unauthorizedEvent } from './client'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

test('prefixes API paths and forwards an abort signal', async () => {
  const fetchMock = vi.fn(async () => new Response(JSON.stringify({ data: [] })))
  vi.stubGlobal('fetch', fetchMock)
  const controller = new AbortController()

  await apiRequest('/categories', { signal: controller.signal })

  expect(fetchMock).toHaveBeenCalledWith('/api/v1/categories', expect.objectContaining({ signal: controller.signal }))
})

test('maps a structured API error without losing field errors', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
    error: {
      code: 'validation_failed',
      message: 'Проверьте поля',
      fields: { login: 'Обязательное поле' },
      request_id: 'request-1',
    },
  }), { status: 400 })))

  await expect(apiRequest('/session')).rejects.toMatchObject({
    name: 'ApiError',
    status: 400,
    code: 'validation_failed',
    message: 'Проверьте поля',
    fields: { login: 'Обязательное поле' },
  } satisfies Partial<ApiError>)
})

test('does not expose an unstructured server response', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('database password leaked', { status: 500 })))

  await expect(apiRequest('/dashboard')).rejects.toMatchObject({
    status: 500,
    code: 'request_failed',
    message: 'Не удалось выполнить запрос',
  })
})

test('publishes a global unauthorized event for a 401 response', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
    error: { code: 'unauthorized', message: 'Требуется вход', request_id: 'request-1' },
  }), { status: 401 })))
  const listener = vi.fn()
  window.addEventListener(unauthorizedEvent, listener)

  await expect(apiRequest('/dashboard')).rejects.toMatchObject({ status: 401 })

  expect(listener).toHaveBeenCalledOnce()
  window.removeEventListener(unauthorizedEvent, listener)
})

test('supports successful responses without a body', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 204 })))

  await expect(apiRequest<void>('/auth/logout', { method: 'POST' })).resolves.toBeUndefined()
})
