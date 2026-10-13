import { useEffect, useRef } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router'

import { useAuth } from './auth'

export function AppLayout() {
  const { user, logout } = useAuth()
  const location = useLocation()
  const content = useRef<HTMLDivElement>(null)
  useEffect(() => { content.current?.focus({ preventScroll: true }) }, [location.pathname])
  if (!user) return null

  return <div className="app-shell">
    <a className="skip-link" href="#page-content">Перейти к содержимому</a>
    <header className="topbar">
      <NavLink className="brand" to="/" aria-label="Go Interview — на главную"><span className="brand-mark">Go</span><span>Interview</span></NavLink>
      <nav className="main-nav" aria-label="Основная навигация">
        <NavLink to="/" end>Главная</NavLink>
        <NavLink to="/topics">Карта знаний</NavLink>
        <NavLink to="/track">Трек</NavLink>
        <NavLink to="/bookmarks">Закладки</NavLink>
        <NavLink to="/review">Повторение</NavLink>
        <NavLink to="/settings">Настройки</NavLink>
      </nav>
      <button className="profile" type="button" aria-label="Выйти" onClick={() => void logout()}>{user.login.slice(0, 1).toUpperCase()}</button>
    </header>
    <div id="page-content" ref={content} tabIndex={-1}><Outlet /></div>
    <footer><span>Go Interview Tracker</span><span>Собрано для системной подготовки</span></footer>
  </div>
}
