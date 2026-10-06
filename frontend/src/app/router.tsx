import { createBrowserRouter, type RouteObject } from 'react-router'

import { App } from '../App'
import { AuthLayout, LoginPage, ProtectedRoute } from './auth'

function NotFoundPage() {
  return <main className="state"><h1>Страница не найдена</h1><a href="/">Вернуться на главную</a></main>
}

export const appRoutes: RouteObject[] = [{
  Component: AuthLayout,
  children: [
    { path: '/login', Component: LoginPage },
    {
      Component: ProtectedRoute,
      children: [{ path: '/', Component: App }],
    },
    { path: '*', Component: NotFoundPage },
  ],
}]

export const router = createBrowserRouter(appRoutes)
