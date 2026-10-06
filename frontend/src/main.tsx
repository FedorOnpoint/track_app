import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router/dom'

import { queryClient } from './app/query-client'
import { router } from './app/router'
import './styles.css'

const root = document.getElementById('root')

if (!root) {
  throw new Error('Root element was not found')
}

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
