import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createContext, type FormEvent, type ReactNode, useContext, useEffect, useState } from 'react'
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router'

import { ApiError, apiRequest, unauthorizedEvent } from '../shared/api/client'
import type { ApiEnvelope } from '../shared/api/types'

export type User = { id: string; login: string }

const sessionKey = ['session'] as const

type AuthContextValue = {
  user: User | null | undefined
  sessionError: boolean
  retrySession: () => void
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const session = useQuery({
    queryKey: sessionKey,
    queryFn: async ({ signal }) => {
      try {
        return (await apiRequest<ApiEnvelope<User>>('/auth/me', { signal })).data
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) return null
        throw error
      }
    },
    retry: false,
  })

  useEffect(() => {
    const handleUnauthorized = () => queryClient.setQueryData<User | null>(sessionKey, null)
    window.addEventListener(unauthorizedEvent, handleUnauthorized)
    return () => window.removeEventListener(unauthorizedEvent, handleUnauthorized)
  }, [queryClient])

  async function logout() {
    try {
      await apiRequest<void>('/auth/logout', { method: 'POST' })
    } finally {
      queryClient.setQueryData<User | null>(sessionKey, null)
    }
  }

  return <AuthContext.Provider value={{
    user: session.data,
    sessionError: session.isError,
    retrySession: () => { void session.refetch() },
    logout,
  }}>{children}</AuthContext.Provider>
}

export function AuthLayout() {
  return <AuthProvider><Outlet /></AuthProvider>
}

export function ProtectedRoute() {
  const { user, sessionError, retrySession } = useAuth()
  const location = useLocation()

  if (user === undefined && !sessionError) {
    return <main className="auth-loading"><span className="loader" /><span>Проверяем сессию…</span></main>
  }
  if (sessionError) {
    return <main className="auth-loading" role="alert"><strong>Не удалось проверить сессию</strong><button type="button" onClick={retrySession}>Повторить</button></main>
  }
  if (user === null) {
    return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  }
  return <Outlet />
}

export function LoginPage() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const location = useLocation()
  const navigate = useNavigate()
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const mutation = useMutation({
    mutationFn: async () => (await apiRequest<ApiEnvelope<User>>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ login, password }),
    })).data,
    onSuccess: (authenticatedUser) => {
      queryClient.setQueryData(sessionKey, authenticatedUser)
      setPassword('')
      navigate(getReturnPath(location.state), { replace: true })
    },
  })

  if (user) return <Navigate to="/" replace />

  function submitLogin(event: FormEvent) {
    event.preventDefault()
    mutation.reset()
    mutation.mutate()
  }

  return <main className="login-page">
    <section className="login-aside"><a className="brand login-brand" href="/"><span className="brand-mark">Go</span><span>Interview</span></a><div><p className="eyebrow">Middle Go Backend</p><h1>Готовьтесь<br />системно.</h1><p>Теория, вопросы и практика — в одном персональном треке.</p></div><span className="login-note">Локальное пространство подготовки</span></section>
    <section className="login-panel"><form className="login-form" onSubmit={submitLogin}><p className="eyebrow">Защищённый доступ</p><h2>С возвращением</h2><p>Войдите, чтобы продолжить с того места, где остановились.</p><label>Логин<input autoComplete="username" value={login} onChange={(event) => setLogin(event.target.value)} required /></label><label>Пароль<input type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} minLength={12} required /></label>{mutation.isError && <div className="login-error" role="alert">Неверный логин или пароль</div>}<button type="submit" disabled={mutation.isPending}>{mutation.isPending ? 'Входим…' : 'Войти'}</button></form></section>
  </main>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}

function getReturnPath(state: unknown): string {
  if (!state || typeof state !== 'object' || !('from' in state)) return '/'
  const from = (state as { from?: unknown }).from
  return typeof from === 'string' && from.startsWith('/') && !from.startsWith('//') ? from : '/'
}
