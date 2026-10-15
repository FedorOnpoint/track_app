import { test as base, expect } from '@playwright/test'

const topic = {
  id: 't1', slug: 'goroutines', title: 'Горутины и планировщик', summary: 'GMP, runtime.GOMAXPROCS и управление временем жизни горутин',
  category_slug: 'concurrency', category_title: 'Конкурентность', difficulty: 'intermediate', estimated_minutes: 35,
  status: 'learning', review_required: true,
  sections: [{ id: 's1', heading: 'Планировщик', body_markdown: '**GMP** распределяет работу.\n\n```go\nfmt.Println("goroutines")\n```', sort_order: 1 }],
  checklist: [{ id: 'c1', stable_key: 'explain-gmp', text: 'Объяснить модель GMP', sort_order: 1 }],
  questions: [{ id: 'q1', stable_key: 'goroutine', question: 'Что такое goroutine?', short_answer: 'Единица выполнения.', detailed_answer: 'Планируется runtime.', pitfalls: 'Не является системным потоком.', sort_order: 1 }],
  follow_ups: [],
  resources: [{ id: 'r1', title: 'Go memory model', url: 'https://go.dev/ref/mem', resource_type: 'documentation', source_name: 'Go', sort_order: 1 }],
  practice: [{ id: 'p1', title: 'Написать worker pool', description: 'Ограничить число worker.', url: null, expected_outcome: 'Нет утечек', sort_order: 1 }],
  user_state: { status: 'learning', review_required: true, bookmarked: false, checklist: { c1: false } },
}

export const test = base.extend<{ mockAPI: void }>({
  mockAPI: [async ({ page }, use) => {
    await page.route('https://fonts.googleapis.com/**', route => route.abort())
    await page.route('**/api/v1/**', async route => {
      const path = new URL(route.request().url()).pathname
      const note = { id: 'n1', topic_id: 't1', topic_slug: 'goroutines', topic_title: topic.title, body_markdown: 'Проверить отмену через context.', created_at: '2026-10-07T00:00:00Z', updated_at: '2026-10-07T00:00:00Z' }
      if (path.endsWith('/auth/me')) return route.fulfill({ status: ['/login', '/register'].includes(new URL(page.url()).pathname) ? 401 : 200, json: { data: { id: 'u1', login: 'fedor' } } })
      if (path.endsWith('/dashboard')) return route.fulfill({ json: { data: { overall: { total: 12, mastered: 5, percent: 42 }, categories: [{ slug: 'concurrency', title: 'Конкурентность', total: 4, mastered: 2, percent: 50 }], review_topics: [topic], recent_topics: [topic], recent_notes: [note], continue_topic: topic } } })
      if (path.endsWith('/categories')) return route.fulfill({ json: { data: [{ id: 'cat1', slug: 'concurrency', title: 'Конкурентность', description: 'Горутины и каналы' }] } })
      if (path.endsWith('/topics/goroutines')) return route.fulfill({ json: { data: topic } })
      if (path.endsWith('/notes')) return route.fulfill({ json: { data: [note] } })
      if (path.endsWith('/topics') || path.endsWith('/review')) return route.fulfill({ json: { data: [topic], pagination: { page: 1, page_size: 20, total_items: 1, total_pages: 1 } } })
      return route.fulfill({ status: 404, json: { error: { code: 'not_found', message: 'Неизвестный тестовый маршрут', request_id: 'test' } } })
    })
    await use()
  }, { auto: true }],
})

export { expect }
