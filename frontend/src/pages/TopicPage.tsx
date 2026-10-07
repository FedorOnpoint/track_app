import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router'

import { TopicNotes } from '../features/notes/TopicNotes'
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
  user_state: TopicUserState
}

type TopicStatus = 'not_started' | 'learning' | 'review' | 'mastered'
type TopicUserState = { status: TopicStatus; review_required: boolean; bookmarked: boolean; checklist: Record<string, boolean> }

const difficultyLabel = { basic: 'База', intermediate: 'Middle', advanced: 'Продвинутая' }

export function TopicPage() {
  const { slug = '' } = useParams()
  const queryClient = useQueryClient()
  const queryKey = ['topic', slug] as const
  const [statusDraft, setStatusDraft] = useState<TopicStatus>('not_started')
  const [reviewDraft, setReviewDraft] = useState(false)
  const progressInFlight = useRef(false)
  const bookmarkInFlight = useRef(false)
  const checklistInFlight = useRef(false)
  const topic = useQuery({
    queryKey,
    queryFn: async ({ signal }) => (await apiRequest<ApiEnvelope<TopicDetail>>(`/topics/${encodeURIComponent(slug)}`, { signal })).data,
  })
  useEffect(() => {
    if (!topic.data) return
    setStatusDraft(topic.data.user_state.status)
    setReviewDraft(topic.data.user_state.review_required)
  }, [topic.data])

  const progressMutation = useMutation({
    mutationFn: (next: { status: TopicStatus; reviewRequired: boolean }) => apiRequest(`/topics/${encodeURIComponent(slug)}/progress`, { method: 'PUT', body: JSON.stringify({ status: next.status, review_required: next.reviewRequired }) }),
    onMutate: async (next) => optimisticTopicUpdate(queryClient, queryKey, (state) => ({ ...state, status: next.status, review_required: next.reviewRequired })),
    onError: (_error, _next, snapshot) => restoreTopic(queryClient, queryKey, snapshot),
    onSuccess: () => invalidateTopicLists(queryClient),
    onSettled: () => { progressInFlight.current = false },
  })
  const bookmarkMutation = useMutation({
    mutationFn: (bookmarked: boolean) => apiRequest<void>(`/topics/${encodeURIComponent(slug)}/bookmark`, { method: bookmarked ? 'PUT' : 'DELETE' }),
    onMutate: async (bookmarked) => optimisticTopicUpdate(queryClient, queryKey, (state) => ({ ...state, bookmarked })),
    onError: (_error, _next, snapshot) => restoreTopic(queryClient, queryKey, snapshot),
    onSuccess: () => invalidateTopicLists(queryClient),
    onSettled: () => { bookmarkInFlight.current = false },
  })
  const checklistMutation = useMutation({
    mutationFn: ({ itemID, completed }: { itemID: string; completed: boolean }) => apiRequest(`/topics/${encodeURIComponent(slug)}/checklist/${encodeURIComponent(itemID)}`, { method: 'PUT', body: JSON.stringify({ completed }) }),
    onMutate: async ({ itemID, completed }) => optimisticTopicUpdate(queryClient, queryKey, (state) => ({ ...state, checklist: { ...state.checklist, [itemID]: completed } })),
    onError: (_error, _next, snapshot) => restoreTopic(queryClient, queryKey, snapshot),
    onSuccess: () => invalidateTopicLists(queryClient),
    onSettled: () => { checklistInFlight.current = false },
  })

  function saveProgress() {
    if (progressInFlight.current) return
    progressInFlight.current = true
    progressMutation.mutate({ status: statusDraft, reviewRequired: reviewDraft })
  }

  function toggleBookmark(bookmarked: boolean) {
    if (bookmarkInFlight.current) return
    bookmarkInFlight.current = true
    bookmarkMutation.mutate(bookmarked)
  }

  function setChecklistItem(itemID: string, completed: boolean) {
    if (checklistInFlight.current) return
    checklistInFlight.current = true
    checklistMutation.mutate({ itemID, completed })
  }

  if (topic.isPending) return <main><div className="state" aria-live="polite"><span className="loader" />Загружаем тему…</div></main>
  if (topic.isError) return <main><div className="state error" role="alert"><strong>Не удалось загрузить тему</strong><span>Тема недоступна или была удалена.</span><button type="button" onClick={() => void topic.refetch()}>Повторить</button></div></main>

  const data = topic.data
  return <main className="topic-page">
    <nav className="breadcrumbs" aria-label="Хлебные крошки"><Link to="/topics">Карта знаний</Link><span aria-hidden="true">/</span><Link to={`/topics?category=${data.category_slug}`}>{data.category_title}</Link><span aria-hidden="true">/</span><span aria-current="page">{data.title}</span></nav>
    <header className="topic-hero"><div><p className="eyebrow">{data.category_title}</p><h1>{data.title}</h1><p className="lead">{data.summary}</p></div><dl><div><dt>Сложность</dt><dd>{difficultyLabel[data.difficulty]}</dd></div><div><dt>Время</dt><dd>{data.estimated_minutes} мин</dd></div></dl></header>
    <section className="topic-actions" aria-label="Состояние темы">
      <label>Статус<select value={statusDraft} disabled={progressMutation.isPending} onChange={(event) => setStatusDraft(event.target.value as TopicStatus)}><option value="not_started">Не начато</option><option value="learning">В изучении</option><option value="review">На повторении</option><option value="mastered">Освоено</option></select></label>
      <label className="review-toggle"><input type="checkbox" checked={reviewDraft} disabled={progressMutation.isPending} onChange={(event) => setReviewDraft(event.target.checked)} />Требует повторения</label>
      <button type="button" disabled={progressMutation.isPending} onClick={saveProgress}>{progressMutation.isPending ? 'Сохраняем…' : 'Сохранить прогресс'}</button>
      <button type="button" disabled={bookmarkMutation.isPending} aria-pressed={data.user_state.bookmarked} onClick={() => toggleBookmark(!data.user_state.bookmarked)}>{data.user_state.bookmarked ? 'Убрать из закладок' : 'Добавить в закладки'}</button>
      {(progressMutation.isError || bookmarkMutation.isError || checklistMutation.isError) && <span className="mutation-error" role="alert">Не удалось сохранить изменение. Предыдущее состояние восстановлено.</span>}
    </section>
    <div className="topic-layout">
      <nav className="topic-toc" aria-label="Содержание"><strong>Содержание</strong><a href="#theory">Теория</a><a href="#checklist">Что проверить</a><a href="#questions">Вопросы</a><a href="#practice">Практика</a><a href="#resources">Источники</a></nav>
      <article className="topic-content">
        <section id="theory" aria-labelledby="theory-title"><p className="eyebrow">Материал</p><h2 id="theory-title">Теория</h2>{data.sections.length === 0 ? <p className="topic-empty">Разделы пока не добавлены.</p> : data.sections.map((section) => <section className="theory-section" key={section.id}><h3>{section.heading}</h3><SafeMarkdown>{section.body_markdown}</SafeMarkdown></section>)}</section>
        <section id="checklist" aria-labelledby="checklist-title"><p className="eyebrow">Самопроверка</p><h2 id="checklist-title">Что нужно уметь объяснить</h2>{data.checklist.length === 0 ? <p className="topic-empty">Чек-лист пока не добавлен.</p> : <ul className="learning-checklist">{data.checklist.map((item) => <li key={item.id}><label><input type="checkbox" checked={data.user_state.checklist[item.id] ?? false} disabled={checklistMutation.isPending} onChange={(event) => setChecklistItem(item.id, event.target.checked)} />{item.text}</label></li>)}</ul>}</section>
        <section id="questions" aria-labelledby="questions-title"><p className="eyebrow">Собеседование</p><h2 id="questions-title">Типовые вопросы</h2>{data.questions.length === 0 ? <p className="topic-empty">Вопросы пока не добавлены.</p> : <div className="question-list">{data.questions.map((question) => <details key={question.id}><summary>{question.question}</summary><div className="question-answer"><strong>Короткий ответ</strong><SafeMarkdown>{question.short_answer}</SafeMarkdown><strong>Подробно</strong><SafeMarkdown>{question.detailed_answer}</SafeMarkdown>{question.pitfalls && <aside className="pitfall"><strong>Частая ошибка</strong><SafeMarkdown>{question.pitfalls}</SafeMarkdown></aside>}{data.follow_ups.filter((item) => item.interview_question_id === question.id).map((followUp) => <div className="follow-up" key={followUp.id}><strong>{followUp.question}</strong><SafeMarkdown>{followUp.expected_answer}</SafeMarkdown></div>)}</div></details>)}</div>}</section>
        <section id="practice" aria-labelledby="practice-title"><p className="eyebrow">Закрепление</p><h2 id="practice-title">Практика</h2>{data.practice.length === 0 ? <p className="topic-empty">Практика пока не добавлена.</p> : <ul className="practice-list">{data.practice.map((item) => <li key={item.id}><h3>{item.title}</h3><SafeMarkdown>{item.description}</SafeMarkdown><p><strong>Результат:</strong> {item.expected_outcome}</p>{safeExternalURL(item.url) && <a href={safeExternalURL(item.url) ?? undefined} target="_blank" rel="noreferrer noopener">Открыть задание ↗</a>}</li>)}</ul>}</section>
        <section id="resources" aria-labelledby="resources-title"><p className="eyebrow">Первоисточники</p><h2 id="resources-title">Материалы</h2>{data.resources.length === 0 ? <p className="topic-empty">Источники пока не добавлены.</p> : <ul className="resource-list">{data.resources.map((resource) => <li key={resource.id}>{safeExternalURL(resource.url) ? <a href={resource.url} target="_blank" rel="noreferrer noopener"><span><strong>{resource.title}</strong><small>{resource.source_name} · {resource.resource_type}</small></span><span aria-hidden="true">↗</span></a> : <span><strong>{resource.title}</strong><small>Недоступная ссылка</small></span>}</li>)}</ul>}</section>
        <TopicNotes slug={data.slug} />
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

type TopicQueryClient = ReturnType<typeof useQueryClient>

async function optimisticTopicUpdate(client: TopicQueryClient, queryKey: readonly ['topic', string], update: (state: TopicUserState) => TopicUserState) {
  await client.cancelQueries({ queryKey })
  const snapshot = client.getQueryData<TopicDetail>(queryKey)
  if (snapshot) client.setQueryData<TopicDetail>(queryKey, { ...snapshot, user_state: update(snapshot.user_state) })
  return snapshot
}

function restoreTopic(client: TopicQueryClient, queryKey: readonly ['topic', string], snapshot: TopicDetail | undefined) {
  if (snapshot) client.setQueryData(queryKey, snapshot)
}

function invalidateTopicLists(client: TopicQueryClient) {
  void client.invalidateQueries({ queryKey: ['topics'] })
  void client.invalidateQueries({ queryKey: ['dashboard'] })
}
