import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

import { apiRequest } from '../shared/api/client'
import type { ApiEnvelope } from '../shared/api/types'

type ProgressSummary = { total: number; mastered: number; percent: number }
type CategoryProgress = ProgressSummary & { slug: string; title: string }
type DashboardTopic = {
  slug: string
  title: string
  status: 'not_started' | 'learning' | 'review' | 'mastered'
  review_required: boolean
  updated_at: string
  last_opened_at: string | null
}
type RecentNote = { id: string; topic_slug: string; topic_title: string; body_markdown: string; updated_at: string }
type Dashboard = {
  overall: ProgressSummary
  categories: CategoryProgress[]
  review_topics: DashboardTopic[]
  recent_topics: DashboardTopic[]
  recent_notes: RecentNote[]
  continue_topic: DashboardTopic | null
}

export function DashboardPage() {
  const dashboard = useQuery({
    queryKey: ['dashboard'],
    queryFn: async ({ signal }) => (await apiRequest<ApiEnvelope<Dashboard>>('/dashboard', { signal })).data,
  })

  if (dashboard.isPending) {
    return <main><div className="state" aria-live="polite"><span className="loader" />Загружаем ваш прогресс…</div></main>
  }
  if (dashboard.isError) {
    return <main><div className="state error" role="alert"><strong>Не удалось загрузить главную</strong><span>Попробуйте повторить запрос.</span><button type="button" onClick={() => void dashboard.refetch()}>Повторить</button></div></main>
  }

  const data = dashboard.data
  return <main>
    <section className="intro dashboard-intro">
      <div><p className="eyebrow">Персональный трек</p><h1>Обзор подготовки</h1><p className="lead">Продолжайте с последней темы или выберите следующий пробел в знаниях.</p></div>
      <div className="overall-progress" aria-label={`Освоено ${formatPercent(data.overall.percent)}%`}>
        <strong>{formatPercent(data.overall.percent)}%</strong>
        <span>{data.overall.mastered} из {data.overall.total} тем освоено</span>
        <progress max="100" value={data.overall.percent}>{formatPercent(data.overall.percent)}%</progress>
      </div>
    </section>

    {data.continue_topic
      ? <section className="continue-card" aria-labelledby="continue-title"><div><p className="eyebrow">Продолжить обучение</p><h2 id="continue-title">{data.continue_topic.title}</h2><span>{statusLabel[data.continue_topic.status]}</span></div><Link to={`/topics/${data.continue_topic.slug}`}>Открыть тему <span aria-hidden="true">→</span></Link></section>
      : <section className="continue-card empty-card"><div><p className="eyebrow">Начало пути</p><h2>Выберите первую тему</h2><span>Прогресс появится после открытия темы.</span></div><Link to="/topics">Открыть карту знаний <span aria-hidden="true">→</span></Link></section>}

    <div className="dashboard-grid">
      <section className="dashboard-section" aria-labelledby="categories-title">
        <div className="section-heading"><div><p className="eyebrow">По направлениям</p><h2 id="categories-title">Прогресс категорий</h2></div></div>
        {data.categories.length === 0
          ? <EmptyState>Каталог пока пуст.</EmptyState>
          : <ul className="category-progress-list">{data.categories.map((category) => <li key={category.slug}><div><Link to={`/topics?category=${category.slug}`}>{category.title}</Link><span>{category.mastered}/{category.total}</span></div><progress aria-label={`${category.title}: ${formatPercent(category.percent)}%`} max="100" value={category.percent} /></li>)}</ul>}
      </section>
      <section className="dashboard-section" aria-labelledby="review-title">
        <div className="section-heading"><div><p className="eyebrow">На повторение</p><h2 id="review-title">Закрепить сегодня</h2></div><Link to="/review">Все темы</Link></div>
        {data.review_topics.length === 0
          ? <EmptyState>Тем для повторения пока нет.</EmptyState>
          : <TopicLinks topics={data.review_topics} />}
      </section>
      <section className="dashboard-section" aria-labelledby="recent-title">
        <div className="section-heading"><div><p className="eyebrow">История</p><h2 id="recent-title">Недавние темы</h2></div></div>
        {data.recent_topics.length === 0
          ? <EmptyState>Вы ещё не открывали темы.</EmptyState>
          : <TopicLinks topics={data.recent_topics} />}
      </section>
      <section className="dashboard-section" aria-labelledby="notes-title">
        <div className="section-heading"><div><p className="eyebrow">Ваши записи</p><h2 id="notes-title">Последние заметки</h2></div></div>
        {data.recent_notes.length === 0
          ? <EmptyState>Заметок пока нет.</EmptyState>
          : <ul className="recent-notes">{data.recent_notes.map((note) => <li key={note.id}><Link to={`/topics/${note.topic_slug}`}><strong>{note.topic_title}</strong><span>{note.body_markdown}</span></Link></li>)}</ul>}
      </section>
    </div>
  </main>
}

const statusLabel: Record<DashboardTopic['status'], string> = {
  not_started: 'Не начато',
  learning: 'В изучении',
  review: 'На повторении',
  mastered: 'Освоено',
}

function TopicLinks({ topics }: { topics: DashboardTopic[] }) {
  return <ul className="dashboard-topic-list">{topics.map((topic) => <li key={topic.slug}><Link to={`/topics/${topic.slug}`}><span><strong>{topic.title}</strong><small>{statusLabel[topic.status]}</small></span><span aria-hidden="true">→</span></Link></li>)}</ul>
}

function EmptyState({ children }: { children: string }) {
  return <p className="dashboard-empty">{children}</p>
}

function formatPercent(value: number) {
  return Math.round(value)
}
