import { createBrowserRouter, type RouteObject } from 'react-router'

import { App } from '../App'
import { DashboardPage } from '../pages/DashboardPage'
import { AuthLayout, LoginPage, ProtectedRoute } from './auth'
import { AppLayout } from './layout'

function NotFoundPage() {
  return <main className="state"><h1>Страница не найдена</h1><a href="/">Вернуться на главную</a></main>
}

export const appRoutes: RouteObject[] = [{
  Component: AuthLayout,
  children: [
    { path: '/login', Component: LoginPage },
    {
      Component: ProtectedRoute,
      children: [{
        Component: AppLayout,
        children: [
          { index: true, Component: DashboardPage },
          { path: '/topics', Component: App },
        ],
      }],
    },
    { path: '*', Component: NotFoundPage },
  ],
}]

export const router = createBrowserRouter(appRoutes)
