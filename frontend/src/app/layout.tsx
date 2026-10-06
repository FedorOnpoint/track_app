import { NavLink, Outlet } from 'react-router'

import { useAuth } from './auth'

export function AppLayout() {
  const { user, logout } = useAuth()
  if (!user) return null

  return <div className="app-shell">
    <header className="topbar">
      <NavLink className="brand" to="/" aria-label="Go Interview — на главную"><span className="brand-mark">Go</span><span>Interview</span></NavLink>
      <nav className="main-nav" aria-label="Основная навигация">
        <NavLink to="/" end>Главная</NavLink>
        <NavLink to="/topics">Карта знаний</NavLink>
        <NavLink to="/review">Повторение</NavLink>
      </nav>
      <button className="profile" type="button" aria-label="Выйти" onClick={() => void logout()}>{user.login.slice(0, 1).toUpperCase()}</button>
    </header>
    <Outlet />
    <footer><span>Go Interview Tracker</span><span>Собрано для системной подготовки</span></footer>
  </div>
}
