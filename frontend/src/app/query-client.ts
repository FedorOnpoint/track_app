import { ApiError } from '../shared/api/client'
import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failures, error) => !(error instanceof ApiError && error.status < 500) && failures < 2,
      staleTime: 30_000,
    },
    mutations: { retry: false },
  },
})
