import { useMutation, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useRef, useState } from 'react'
import { ApiError, apiRequest } from '../shared/api/client'

export function SettingsPage() {
  const client = useQueryClient()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [validation, setValidation] = useState('')
  const passwordInFlight = useRef(false)
  const logoutInFlight = useRef(false)
  const password = useMutation({
    mutationFn: () => apiRequest<void>('/auth/password', { method: 'PUT', body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) }),
    onSuccess: () => { setCurrentPassword(''); setNewPassword(''); setConfirmation('') },
    onSettled: () => { passwordInFlight.current = false },
  })
  const logoutAll = useMutation({
    mutationFn: () => apiRequest<void>('/auth/logout-all', { method: 'POST' }),
    onSuccess: () => { client.setQueryData(['session'], null); client.removeQueries({ predicate: (query) => query.queryKey[0] !== 'session' }) },
    onSettled: () => { logoutInFlight.current = false },
  })
  function submit(event: FormEvent) {
    event.preventDefault()
    if (passwordInFlight.current) return
    setValidation('')
    if (newPassword !== confirmation) { setValidation('Пароли не совпадают'); return }
    if (new TextEncoder().encode(newPassword).length > 1024) { setValidation('Пароль слишком длинный'); return }
    passwordInFlight.current = true
    password.mutate()
  }
  return <main>
    <section className="intro"><div><p className="eyebrow">Аккаунт</p><h1>Настройки</h1></div></section>
    <section className="settings-panel" aria-labelledby="password-title"><h2 id="password-title">Смена пароля</h2><p>После смены пароля остальные сессии будут завершены.</p>
      <form className="login-form" onSubmit={submit}>
        <label>Текущий пароль<input type="password" autoComplete="current-password" required minLength={12} value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} /></label>
        <label>Новый пароль<input type="password" autoComplete="new-password" required minLength={12} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></label>
        <label>Повторите новый пароль<input type="password" autoComplete="new-password" required value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label>
        {validation && <p role="alert">{validation}</p>}
        {password.isError && <p role="alert">{password.error instanceof ApiError && password.error.code === 'invalid_credentials' ? 'Неверный текущий пароль' : 'Не удалось сменить пароль. Попробуйте ещё раз.'}</p>}
        {password.isSuccess && <p role="status">Пароль изменён.</p>}
        <button type="submit" disabled={password.isPending}>{password.isPending ? 'Сохраняем…' : 'Сменить пароль'}</button>
      </form>
    </section>
    <section className="settings-panel" aria-labelledby="sessions-title"><h2 id="sessions-title">Сессии</h2><p>Завершить все сессии, включая текущую.</p><button type="button" disabled={logoutAll.isPending} onClick={() => { if (logoutInFlight.current) return; logoutInFlight.current = true; logoutAll.mutate() }}>Выйти на всех устройствах</button>{logoutAll.isError && <p role="alert">Не удалось завершить сессии. Попробуйте ещё раз.</p>}</section>
  </main>
}
