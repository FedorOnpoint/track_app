import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'

import { apiRequest } from '../shared/api/client'
import type { ApiEnvelope } from '../shared/api/types'
import { SafeMarkdown } from '../shared/ui/SafeMarkdown'

type TopicSection = { id: string; heading: string; body_markdown: string; sort_order: number }
type ChecklistItem = { id: string; stable_key: string; text: string; sort_order: number }
type Question = { id: string; stable_key: string; question: string; short_answer: string; detailed_answer: string; pitfalls: string; sort_order: number }
type FollowUp = { id: string; interview_question_id: string; question: string; expected_answer: string; sort_order: number }
type Resource = { id: string; title: string; url: string; resource_type: string; source_name: string; sort_order: number }
type Practice = { id: string; title: string; description: string; url: string | null; expected_outcome: string; sort_order: number }
type TopicDetail = {
  id: string
  slug: string
  title: string
  summary: string
  difficulty: 'basic' | 'intermediate' | 'advanced'
  estimated_minutes: number
  category_slug: string
  category_title: string
  sections: TopicSection[]
  checklist: ChecklistItem[]
  questions: Question[]
  follow_ups: FollowUp[]
  resources: Resource[]
  practice: Practice[]
}

const difficultyLabel = { basic: 'База', intermediate: 'Middle', advanced: 'Продвинутая' }

export function TopicPage() {
  const { slug = '' } = useParams()
  const topic = useQuery({
    queryKey: ['topic', slug],
    queryFn: async ({ signal }) => (await apiRequest<ApiEnvelope<TopicDetail>>(`/topics/${encodeURIComponent(slug)}`, { signal })).data,
  })

  if (topic.isPending) return <main><div className="state" aria-live="polite"><span className="loader" />Загружаем тему…</div></main>
  if (topic.isError) return <main><div className="state error" role="alert"><strong>Не удалось загрузить тему</strong><span>Тема недоступна или была удалена.</span><button type="button" onClick={() => void topic.refetch()}>Повторить</button></div></main>

  const data = topic.data
  return <main className="topic-page">
    <nav className="breadcrumbs" aria-label="Хлебные крошки"><Link to="/topics">Карта знаний</Link><span aria-hidden="true">/</span><Link to={`/topics?category=${data.category_slug}`}>{data.category_title}</Link><span aria-hidden="true">/</span><span aria-current="page">{data.title}</span></nav>
    <header className="topic-hero"><div><p className="eyebrow">{data.category_title}</p><h1>{data.title}</h1><p className="lead">{data.summary}</p></div><dl><div><dt>Сложность</dt><dd>{difficultyLabel[data.difficulty]}</dd></div><div><dt>Время</dt><dd>{data.estimated_minutes} мин</dd></div></dl></header>
    <div className="topic-layout">
      <nav className="topic-toc" aria-label="Содержание"><strong>Содержание</strong><a href="#theory">Теория</a><a href="#checklist">Что проверить</a><a href="#questions">Вопросы</a><a href="#practice">Практика</a><a href="#resources">Источники</a></nav>
      <article className="topic-content">
        <section id="theory" aria-labelledby="theory-title"><p className="eyebrow">Материал</p><h2 id="theory-title">Теория</h2>{data.sections.length === 0 ? <p className="topic-empty">Разделы пока не добавлены.</p> : data.sections.map((section) => <section className="theory-section" key={section.id}><h3>{section.heading}</h3><SafeMarkdown>{section.body_markdown}</SafeMarkdown></section>)}</section>
        <section id="checklist" aria-labelledby="checklist-title"><p className="eyebrow">Самопроверка</p><h2 id="checklist-title">Что нужно уметь объяснить</h2>{data.checklist.length === 0 ? <p className="topic-empty">Чек-лист пока не добавлен.</p> : <ul className="learning-checklist">{data.checklist.map((item) => <li key={item.id}>{item.text}</li>)}</ul>}</section>
        <section id="questions" aria-labelledby="questions-title"><p className="eyebrow">Собеседование</p><h2 id="questions-title">Типовые вопросы</h2>{data.questions.length === 0 ? <p className="topic-empty">Вопросы пока не добавлены.</p> : <div className="question-list">{data.questions.map((question) => <details key={question.id}><summary>{question.question}</summary><div className="question-answer"><strong>Короткий ответ</strong><SafeMarkdown>{question.short_answer}</SafeMarkdown><strong>Подробно</strong><SafeMarkdown>{question.detailed_answer}</SafeMarkdown>{question.pitfalls && <aside className="pitfall"><strong>Частая ошибка</strong><SafeMarkdown>{question.pitfalls}</SafeMarkdown></aside>}{data.follow_ups.filter((item) => item.interview_question_id === question.id).map((followUp) => <div className="follow-up" key={followUp.id}><strong>{followUp.question}</strong><SafeMarkdown>{followUp.expected_answer}</SafeMarkdown></div>)}</div></details>)}</div>}</section>
        <section id="practice" aria-labelledby="practice-title"><p className="eyebrow">Закрепление</p><h2 id="practice-title">Практика</h2>{data.practice.length === 0 ? <p className="topic-empty">Практика пока не добавлена.</p> : <ul className="practice-list">{data.practice.map((item) => <li key={item.id}><h3>{item.title}</h3><SafeMarkdown>{item.description}</SafeMarkdown><p><strong>Результат:</strong> {item.expected_outcome}</p>{safeExternalURL(item.url) && <a href={safeExternalURL(item.url) ?? undefined} target="_blank" rel="noreferrer noopener">Открыть задание ↗</a>}</li>)}</ul>}</section>
        <section id="resources" aria-labelledby="resources-title"><p className="eyebrow">Первоисточники</p><h2 id="resources-title">Материалы</h2>{data.resources.length === 0 ? <p className="topic-empty">Источники пока не добавлены.</p> : <ul className="resource-list">{data.resources.map((resource) => <li key={resource.id}>{safeExternalURL(resource.url) ? <a href={resource.url} target="_blank" rel="noreferrer noopener"><span><strong>{resource.title}</strong><small>{resource.source_name} · {resource.resource_type}</small></span><span aria-hidden="true">↗</span></a> : <span><strong>{resource.title}</strong><small>Недоступная ссылка</small></span>}</li>)}</ul>}</section>
      </article>
    </div>
  </main>
}

function safeExternalURL(value: string | null) {
  if (!value) return null
  try {
    const url = new URL(value)
    return url.protocol === 'https:' || url.protocol === 'http:' ? value : null
  } catch {
    return null
  }
}
