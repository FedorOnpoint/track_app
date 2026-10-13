import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useRef } from 'react'
import { Link, useSearchParams } from 'react-router'

import { apiRequest } from '../shared/api/client'
import type { PaginatedEnvelope } from '../shared/api/types'

type Status = 'not_started' | 'learning' | 'review' | 'mastered'
type Topic = { id: string; slug: string; title: string; category_title: string; difficulty: string; status: Status; review_required?: boolean }
type Mode = 'track' | 'bookmarks' | 'review'
const labels: Record<Status, string> = { not_started: 'Не начато', learning: 'В изучении', review: 'На повторении', mastered: 'Освоено' }
const titles: Record<Mode, string> = { track: 'Трек подготовки', bookmarks: 'Закладки', review: 'Повторение' }

export function TopicListPage({ mode }: { mode: Mode }) {
  const [params, setParams] = useSearchParams()
  const client = useQueryClient()
  const inFlight = useRef(false)
  const queryKey = ['topics', mode, params.toString()] as const
  const topics = useQuery({
    queryKey,
    queryFn: ({ signal }) => {
      const query = new URLSearchParams(params)
      query.set('page_size', '20')
      query.set('sort', 'category')
      if (mode === 'bookmarks') query.set('bookmarked', 'true')
      return apiRequest<PaginatedEnvelope<Topic>>(`${mode === 'review' ? '/review' : '/topics'}?${query}`, { signal })
    },
  })
  const mutation = useMutation({
    mutationFn: ({ topic, status }: { topic: Topic; status: Status }) => apiRequest(`/topics/${encodeURIComponent(topic.slug)}/progress`, { method: 'PUT', body: JSON.stringify({ status, review_required: topic.review_required ?? false }) }),
    onSuccess: () => Promise.all([
      client.invalidateQueries({ queryKey: ['topics'] }),
      client.invalidateQueries({ queryKey: ['topic'] }),
      client.invalidateQueries({ queryKey: ['dashboard'] }),
    ]),
    onSettled: () => { inFlight.current = false },
  })
  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const next = new URLSearchParams(params)
    const value = String(new FormData(event.currentTarget).get('query') ?? '').trim()
    if (value) next.set('query', value)
    else next.delete('query')
    next.delete('page')
    setParams(next)
  }
  function setStatusFilter(value: string) {
    const next = new URLSearchParams(params)
    if (value) next.set('status', value)
    else next.delete('status')
    next.delete('page')
    setParams(next)
  }
  const page = topics.data?.pagination
  return <main>
    <section className="intro"><div><p className="eyebrow">Персональная подготовка</p><h1>{titles[mode]}</h1><p className="lead">Выбирайте тему и отмечайте освоенный материал.</p></div></section>
    <form className="search" role="search" onSubmit={search} key={params.get('query') ?? ''}><label className="search-field"><span className="sr-only">Поиск по темам</span><input name="query" defaultValue={params.get('query') ?? ''} placeholder="Найти тему" /></label><button type="submit">Найти</button></form>
    <div className="list-controls"><label>Статус<select value={params.get('status') ?? ''} onChange={(event) => setStatusFilter(event.target.value)}><option value="">Все статусы</option>{Object.entries(labels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><button type="button" onClick={() => setParams({})}>Сбросить фильтры</button></div>
    {mutation.isError && <p role="alert">Не удалось сохранить статус. Попробуйте ещё раз.</p>}
    {topics.isPending && <div className="state" role="status">Загружаем темы…</div>}
    {topics.isError && <div className="state" role="alert">Не удалось загрузить темы.<button type="button" onClick={() => void topics.refetch()}>Повторить</button></div>}
    {topics.data && <><table className="track-table"><caption className="sr-only">{titles[mode]}</caption><thead><tr><th scope="col">Тема</th><th scope="col">Категория</th><th scope="col">Статус</th></tr></thead><tbody>{topics.data.data.map((topic) => <tr key={topic.id}><td><Link to={`/topics/${topic.slug}`}>{topic.title}</Link></td><td>{topic.category_title}</td><td><select aria-label={`Статус: ${topic.title}`} value={topic.status} disabled={mutation.isPending} onChange={(event) => { if (inFlight.current) return; inFlight.current = true; mutation.mutate({ topic, status: event.target.value as Status }) }}>{Object.entries(labels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></td></tr>)}</tbody></table>{topics.data.data.length === 0 && <div className="state">Темы не найдены.</div>}{page && page.total_pages > 1 && <nav className="pagination" aria-label="Страницы списка"><button type="button" disabled={page.page <= 1} onClick={() => { const next = new URLSearchParams(params); next.set('page', String(page.page - 1)); setParams(next) }}>Назад</button><span>{page.page} / {page.total_pages}</span><button type="button" disabled={page.page >= page.total_pages} onClick={() => { const next = new URLSearchParams(params); next.set('page', String(page.page + 1)); setParams(next) }}>Далее</button></nav>}</>}
  </main>
}
