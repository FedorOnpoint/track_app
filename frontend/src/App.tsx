import { useQuery } from '@tanstack/react-query'
import { FormEvent, type PointerEvent, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'

import { apiRequest } from './shared/api/client'
import type { ApiEnvelope, PaginatedEnvelope } from './shared/api/types'

type Category = { id: string; slug: string; title: string; description: string }
type Topic = { id: string; slug: string; title: string; summary: string; difficulty: 'basic' | 'intermediate' | 'advanced'; estimated_minutes: number; category_slug: string; category_title: string }
type Pagination = { page: number; page_size: number; total_items: number; total_pages: number }
const difficultyLabel = { basic: 'База', intermediate: 'Middle', advanced: 'Продвинутая' }

export function App() {
  const [searchParams, setSearchParams] = useSearchParams()
  const category = searchParams.get('category') ?? ''
  const difficulty = searchParams.get('difficulty') ?? ''
  const query = searchParams.get('query') ?? ''
  const page = positivePage(searchParams.get('page'))
  const [searchDraft, setSearchDraft] = useState('')
  const categoriesQuery = useQuery({
    queryKey: ['categories'],
    queryFn: async ({ signal }) => (await apiRequest<ApiEnvelope<Category[]>>('/categories', { signal })).data,
  })
  const topicsQuery = useQuery({
    queryKey: ['topics', { category, difficulty, page, query }],
    queryFn: async ({ signal }) => {
      const params = new URLSearchParams({ page: String(page), page_size: '20', sort: 'category' })
      if (category) params.set('category', category)
      if (difficulty) params.set('difficulty', difficulty)
      if (query) params.set('query', query)
      return apiRequest<PaginatedEnvelope<Topic>>(`/topics?${params}`, { signal })
    },
  })
  const categories = categoriesQuery.data ?? []
  const topics = topicsQuery.data?.data ?? []
  const pagination: Pagination = topicsQuery.data?.pagination ?? { page, page_size: 20, total_items: 0, total_pages: 0 }
  const loading = categoriesQuery.isPending || topicsQuery.isPending
  const error = categoriesQuery.isError || topicsQuery.isError

  useEffect(() => {
    setSearchDraft(query)
  }, [query])

  function updateFilters(updates: Record<string, string>) {
    const next = new URLSearchParams(searchParams)
    for (const [name, value] of Object.entries(updates)) {
      if (value) next.set(name, value)
      else next.delete(name)
    }
    if (!('page' in updates)) next.delete('page')
    setSearchParams(next)
  }

  function submitSearch(event: FormEvent) {
    event.preventDefault()
    updateFilters({ query: searchDraft.trim() })
  }
  const totalMinutes = topics.reduce((sum, topic) => sum + topic.estimated_minutes, 0)

  return <main id="top">
      <section className="intro" aria-labelledby="map-title">
        <div><p className="eyebrow">Трек подготовки · Middle Go Backend</p><h1 id="map-title">Карта знаний</h1><p className="lead">Системный путь от устройства Go до проектирования production-сервисов.</p></div>
        <div className="study-summary" aria-label="Сводка каталога"><span><strong>{pagination.total_items}</strong> {topicWord(pagination.total_items)}</span><span><strong>{totalMinutes}</strong> минут в выборке</span></div>
      </section>
      <form className="search" onSubmit={submitSearch} role="search">
        <label className="search-field"><span aria-hidden="true">⌕</span><span className="sr-only">Поиск по темам</span><input value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} placeholder="Найти тему, например goroutines" /></label>
        <button type="submit">Найти</button>
      </form>
      <div className="content-grid" id="map">
        <aside className="filters" aria-label="Фильтры каталога">
          <label className="mobile-category-filter">Раздел<select value={category} onChange={(event) => updateFilters({ category: event.target.value })}><option value="">Все темы</option>{categories.map((item) => <option key={item.id} value={item.slug}>{item.title}</option>)}</select></label>
          <div className="filter-heading"><h2>Разделы</h2><span>{categories.length}</span></div>
          <button className={!category ? 'filter-option selected' : 'filter-option'} onClick={() => updateFilters({ category: '' })} type="button"><span>Все темы</span><span>↗</span></button>
          {categories.map((item) => <button className={category === item.slug ? 'filter-option selected' : 'filter-option'} onClick={() => updateFilters({ category: item.slug })} type="button" key={item.id}><span>{item.title}</span><span>↗</span></button>)}
          <div className="difficulty-filter"><h2>Сложность</h2><select value={difficulty} onChange={(event) => updateFilters({ difficulty: event.target.value })} aria-label="Сложность"><option value="">Любая</option><option value="basic">База</option><option value="intermediate">Middle</option><option value="advanced">Продвинутая</option></select></div>
        </aside>
        <section className="topic-area" aria-live="polite" aria-busy={loading}>
          <div className="list-heading"><div><p className="eyebrow">Учебный каталог</p><h2>{category ? categories.find((item) => item.slug === category)?.title : 'Все темы'}</h2></div><span className="result-count">{pagination.total_items} {topicWord(pagination.total_items)}</span></div>
          {loading && <div className="state"><span className="loader" />Загружаем карту знаний…</div>}
          {error && <div className="state error" role="alert"><strong>Не удалось загрузить каталог</strong><span>Проверьте, запущен ли API, и попробуйте ещё раз.</span><button type="button" onClick={() => { void categoriesQuery.refetch(); void topicsQuery.refetch() }}>Повторить</button></div>}
          {!loading && !error && topics.length === 0 && <div className="state"><strong>Темы не найдены</strong><span>Измените поиск или сбросьте фильтры.</span><button type="button" onClick={() => { setSearchDraft(''); setSearchParams({}) }}>Сбросить фильтры</button></div>}
          {!loading && !error && topics.length > 0 && <><ol className="topic-list">{topics.map((topic, index) => <li key={topic.id}><Link className="topic-card" onPointerMove={tiltTopic} onPointerLeave={resetTopicTilt} onPointerCancel={resetTopicTilt} to={`/topics/${topic.slug}`}><span className="topic-number">{String((pagination.page - 1) * pagination.page_size + index + 1).padStart(2, '0')}</span><span className="topic-copy"><span className="topic-meta"><span>{topic.category_title}</span><span className={`difficulty ${topic.difficulty}`}>{difficultyLabel[topic.difficulty]}</span></span><strong>{topic.title}</strong><span>{topic.summary}</span></span><span className="topic-time"><strong>{topic.estimated_minutes}</strong><span>мин</span></span><span className="topic-arrow" aria-hidden="true">→</span></Link></li>)}</ol><nav className="pagination" aria-label="Страницы каталога"><button type="button" disabled={pagination.page <= 1} onClick={() => updateFilters({ page: String(pagination.page - 1) })}>Назад</button><span>Страница {pagination.page} из {pagination.total_pages}</span><button type="button" disabled={pagination.page >= pagination.total_pages} onClick={() => updateFilters({ page: String(pagination.page + 1) })}>Далее</button></nav></>}
        </section>
      </div>
  </main>
}

function positivePage(value: string | null) {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 1
}

function topicWord(count: number) {
  const mod100 = count % 100, mod10 = count % 10
  if (mod100 >= 11 && mod100 <= 14) return 'тем'
  if (mod10 === 1) return 'тема'
  if (mod10 >= 2 && mod10 <= 4) return 'темы'
  return 'тем'
}

function resetTopicTilt(event: PointerEvent<HTMLAnchorElement>) {
  event.currentTarget.style.removeProperty('--tilt-x')
  event.currentTarget.style.removeProperty('--tilt-y')
}

function tiltTopic(event: PointerEvent<HTMLAnchorElement>) {
  if (event.pointerType !== 'mouse' || !window.matchMedia('(hover: hover) and (pointer: fine) and (prefers-reduced-motion: no-preference)').matches) {
    resetTopicTilt(event)
    return
  }
  // Measure the stationary list item to avoid feedback from the card's transform.
  const bounds = event.currentTarget.parentElement!.getBoundingClientRect()
  const x = Math.max(-1, Math.min(1, (event.clientX - bounds.left) / bounds.width * 2 - 1))
  const y = Math.max(-1, Math.min(1, (event.clientY - bounds.top) / bounds.height * 2 - 1))
  event.currentTarget.style.setProperty('--tilt-x', `${-y * 2}deg`)
  event.currentTarget.style.setProperty('--tilt-y', `${x * 0.8}deg`)
}
