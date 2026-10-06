import { createBrowserRouter } from 'react-router'

import { App } from '../App'

function NotFoundPage() {
  return <main className="state"><h1>Страница не найдена</h1><a href="/">Вернуться на главную</a></main>
}

export const router = createBrowserRouter([
  { path: '/', Component: App },
  { path: '*', Component: NotFoundPage },
])
