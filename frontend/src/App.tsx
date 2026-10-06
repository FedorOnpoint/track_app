import { FormEvent, useCallback, useEffect, useState } from 'react'

import { useAuth } from './app/auth'
import { apiRequest } from './shared/api/client'
import type { ApiEnvelope, PaginatedEnvelope } from './shared/api/types'

type Category = { id: string; slug: string; title: string; description: string }
type Topic = { id: string; slug: string; title: string; summary: string; difficulty: 'basic' | 'intermediate' | 'advanced'; estimated_minutes: number; category_slug: string; category_title: string }
type Pagination = { page: number; page_size: number; total_items: number; total_pages: number }
const difficultyLabel = { basic: 'База', intermediate: 'Middle', advanced: 'Продвинутая' }

export function App() {
  const { user, logout } = useAuth()
  const [categories, setCategories] = useState<Category[]>([])
  const [topics, setTopics] = useState<Topic[]>([])
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total_items: 0, total_pages: 0 })
  const [category, setCategory] = useState('')
  const [difficulty, setDifficulty] = useState('')
  const [searchDraft, setSearchDraft] = useState('')
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  const loadCatalog = useCallback((signal: AbortSignal) => {
    setLoading(true); setError(false)
    const params = new URLSearchParams({ page_size: '20', sort: 'category' })
    if (category) params.set('category', category)
    if (difficulty) params.set('difficulty', difficulty)
    if (query) params.set('query', query)
    return Promise.all([
      apiRequest<ApiEnvelope<Category[]>>('/categories', { signal }),
      apiRequest<PaginatedEnvelope<Topic>>(`/topics?${params}`, { signal }),
    ]).then(([categoryResponse, topicResponse]) => {
      setCategories(categoryResponse.data); setTopics(topicResponse.data); setPagination(topicResponse.pagination)
    }).catch((requestError: unknown) => {
      if (!(requestError instanceof DOMException && requestError.name === 'AbortError')) setError(true)
    }).finally(() => { if (!signal.aborted) setLoading(false) })
  }, [category, difficulty, query])

  useEffect(() => {
		if (!user) return
    const controller = new AbortController(); void loadCatalog(controller.signal)
    return () => controller.abort()
  }, [loadCatalog, user])

  function submitSearch(event: FormEvent) { event.preventDefault(); setQuery(searchDraft.trim()) }
  const totalMinutes = topics.reduce((sum, topic) => sum + topic.estimated_minutes, 0)

  if (!user) return null

  return <div className="app-shell">
    <header className="topbar">
      <a className="brand" href="#top" aria-label="Go Interview — на главную"><span className="brand-mark">Go</span><span>Interview</span></a>
      <nav className="main-nav" aria-label="Основная навигация"><a className="active" href="#map">Карта знаний</a><a href="#review">Повторение</a><a href="#progress">Прогресс</a></nav>
      <button className="profile" type="button" aria-label="Выйти" onClick={() => void logout()}>{user.login.slice(0, 1).toUpperCase()}</button>
    </header>
    <main id="top">
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
          <div className="filter-heading"><h2>Разделы</h2><span>{categories.length}</span></div>
          <button className={!category ? 'filter-option selected' : 'filter-option'} onClick={() => setCategory('')} type="button"><span>Все темы</span><span>↗</span></button>
          {categories.map((item) => <button className={category === item.slug ? 'filter-option selected' : 'filter-option'} onClick={() => setCategory(item.slug)} type="button" key={item.id}><span>{item.title}</span><span>↗</span></button>)}
          <div className="difficulty-filter"><h2>Сложность</h2><select value={difficulty} onChange={(event) => setDifficulty(event.target.value)} aria-label="Сложность"><option value="">Любая</option><option value="basic">База</option><option value="intermediate">Middle</option><option value="advanced">Продвинутая</option></select></div>
        </aside>
        <section className="topic-area" aria-live="polite" aria-busy={loading}>
          <div className="list-heading"><div><p className="eyebrow">Учебный каталог</p><h2>{category ? categories.find((item) => item.slug === category)?.title : 'Все темы'}</h2></div><span className="result-count">{pagination.total_items} {topicWord(pagination.total_items)}</span></div>
          {loading && <div className="state"><span className="loader" />Загружаем карту знаний…</div>}
          {error && <div className="state error" role="alert"><strong>Не удалось загрузить каталог</strong><span>Проверьте, запущен ли API, и попробуйте ещё раз.</span><button type="button" onClick={() => { const controller = new AbortController(); void loadCatalog(controller.signal) }}>Повторить</button></div>}
          {!loading && !error && topics.length === 0 && <div className="state"><strong>Темы не найдены</strong><span>Измените поиск или сбросьте фильтры.</span><button type="button" onClick={() => { setCategory(''); setDifficulty(''); setQuery(''); setSearchDraft('') }}>Сбросить фильтры</button></div>}
          {!loading && !error && topics.length > 0 && <ol className="topic-list">{topics.map((topic, index) => <li key={topic.id}><a className="topic-card" href={`/topics/${topic.slug}`}><span className="topic-number">{String(index + 1).padStart(2, '0')}</span><span className="topic-copy"><span className="topic-meta"><span>{topic.category_title}</span><span className={`difficulty ${topic.difficulty}`}>{difficultyLabel[topic.difficulty]}</span></span><strong>{topic.title}</strong><span>{topic.summary}</span></span><span className="topic-time"><strong>{topic.estimated_minutes}</strong><span>мин</span></span><span className="topic-arrow" aria-hidden="true">→</span></a></li>)}</ol>}
        </section>
      </div>
    </main>
    <footer><span>Go Interview Tracker</span><span>Собрано для системной подготовки</span></footer>
  </div>
}

function topicWord(count: number) {
  const mod100 = count % 100, mod10 = count % 10
  if (mod100 >= 11 && mod100 <= 14) return 'тем'
  if (mod10 === 1) return 'тема'
  if (mod10 >= 2 && mod10 <= 4) return 'темы'
  return 'тем'
}
