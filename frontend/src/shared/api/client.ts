import type { ApiErrorEnvelope } from './types'

export const unauthorizedEvent = 'track-app:unauthorized'

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    public readonly fields?: Record<string, string>,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body !== undefined && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const response = await fetch(`/api/v1${path}`, { ...init, headers })
  if (!response.ok) {
    const error = await toApiError(response)
    if (response.status === 401 && error.code !== 'invalid_credentials') window.dispatchEvent(new Event(unauthorizedEvent))
    throw error
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

async function toApiError(response: Response): Promise<ApiError> {
  const fallback = new ApiError(response.status, 'request_failed', 'Не удалось выполнить запрос')
  try {
    const body = await response.json() as Partial<ApiErrorEnvelope>
    const error = body.error
    if (!error || typeof error.code !== 'string' || typeof error.message !== 'string') return fallback
    return new ApiError(response.status, error.code, error.message, error.fields)
  } catch {
    return fallback
  }
}

export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}
