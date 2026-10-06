export type ApiEnvelope<T> = { data: T }
export type PaginatedEnvelope<T> = ApiEnvelope<T[]> & {
  pagination: { page: number; page_size: number; total_items: number; total_pages: number }
}
export type ApiErrorEnvelope = {
  error: { code: string; message: string; fields?: Record<string, string>; request_id: string }
}
