import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router'

import { useAuth, type User } from '../app/auth'
import { ApiError, apiRequest } from '../shared/api/client'
import type { ApiEnvelope } from '../shared/api/types'

export function RegisterPage() {
  const { user } = useAuth()
  const client = useQueryClient()
  const navigate = useNavigate()
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [invitation, setInvitation] = useState('')
  const [formError, setFormError] = useState('')
  const mutation = useMutation({
    mutationFn: async () => (await apiRequest<ApiEnvelope<User>>('/auth/register', {
      method: 'POST', body: JSON.stringify({ login, password, invitation_code: invitation.trim() }),
    })).data,
    onSuccess: (registeredUser) => {
      client.setQueryData(['session'], registeredUser)
      setPassword(''); setConfirmation(''); setInvitation('')
      navigate('/', { replace: true })
    },
    retry: false,
  })

  if (user) return <Navigate to="/" replace />

  function submit(event: FormEvent) {
    event.preventDefault()
    if (mutation.isPending) return
    mutation.reset(); setFormError('')
    if (password !== confirmation) { setFormError('Пароли не совпадают'); return }
    if (new TextEncoder().encode(password).length > 1024) { setFormError('Пароль слишком длинный'); return }
    mutation.mutate()
  }

  const error = formError || (mutation.error instanceof ApiError ? mutation.error.message : mutation.isError ? 'Не удалось создать аккаунт. Попробуйте ещё раз' : '')
  return <main className="login-page">
    <section className="login-aside">
      <Link className="brand login-brand" to="/"><span className="brand-mark">Go</span><span>Interview</span></Link>
      <div><p className="eyebrow">Middle Go Backend</p><h2>Ваш путь<br />к следующему<br />уровню.</h2><p>Личный прогресс, заметки и темы для повторения — в одном месте.</p></div>
      <span className="login-note">Персональное пространство подготовки</span>
    </section>
    <section className="login-panel">
      <form className="login-form registration-form" onSubmit={submit}>
        <p className="eyebrow">По приглашению</p><h1>Создать аккаунт</h1>
        <p>Введите код приглашения и начните свой трек подготовки.</p>
        <label>Логин<input autoComplete="username" autoCapitalize="none" spellCheck={false} value={login} onChange={e => setLogin(e.target.value)} minLength={3} maxLength={64} pattern="[a-z0-9][a-z0-9._\-]{2,63}" aria-describedby="login-hint" required /></label>
        <small id="login-hint" className="field-hint">3–64 символа: строчные латинские буквы, цифры, точка, дефис или подчёркивание.</small>
        <label>Пароль<input type="password" autoComplete="new-password" value={password} onChange={e => setPassword(e.target.value)} minLength={12} maxLength={1024} aria-describedby="password-hint" required /></label>
        <small id="password-hint" className="field-hint">Минимум 12 символов. Используйте отдельный пароль для этого сайта.</small>
        <label>Повторите пароль<input type="password" autoComplete="new-password" value={confirmation} onChange={e => setConfirmation(e.target.value)} minLength={12} maxLength={1024} required /></label>
        <label>Код приглашения<input autoComplete="off" autoCapitalize="none" spellCheck={false} value={invitation} onChange={e => setInvitation(e.target.value)} maxLength={64} aria-describedby="invite-hint" required /></label>
        <small id="invite-hint" className="field-hint">Получите одноразовый код у владельца приложения.</small>
        {error && <div className="login-error" role="alert">{error}</div>}
        <button type="submit" disabled={mutation.isPending}>{mutation.isPending ? 'Создаём аккаунт…' : 'Создать аккаунт'}</button>
        <p className="auth-switch">Уже есть аккаунт? <Link to="/login">Войти</Link></p>
      </form>
    </section>
  </main>
}
