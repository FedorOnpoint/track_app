import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useEffect, useRef, useState } from 'react'

import { apiRequest } from '../../shared/api/client'
import type { ApiEnvelope } from '../../shared/api/types'
import { SafeMarkdown } from '../../shared/ui/SafeMarkdown'

type Note = {
  id: string
  topic_id: string
  topic_slug: string
  topic_title: string
  body_markdown: string
  created_at: string
  updated_at: string
}

const notesKey = ['notes'] as const

export function TopicNotes({ slug }: { slug: string }) {
  const queryClient = useQueryClient()
  const [newBody, setNewBody] = useState('')
  const [editingID, setEditingID] = useState<string | null>(null)
  const [editBody, setEditBody] = useState('')
  const [confirmID, setConfirmID] = useState<string | null>(null)
  const confirmButton = useRef<HTMLButtonElement>(null)
  const deleteTrigger = useRef<HTMLButtonElement | null>(null)
  const notes = useQuery({
    queryKey: notesKey,
    queryFn: async ({ signal }) => (await apiRequest<ApiEnvelope<Note[]>>('/notes', { signal })).data,
  })

  useEffect(() => {
    if (confirmID) confirmButton.current?.focus()
  }, [confirmID])

  const createMutation = useMutation({
    mutationFn: async (body: string) => (await apiRequest<ApiEnvelope<Note>>(`/topics/${encodeURIComponent(slug)}/notes`, { method: 'POST', body: JSON.stringify({ body_markdown: body }) })).data,
    onSuccess: (created) => {
      queryClient.setQueryData<Note[]>(notesKey, (current = []) => [created, ...current])
      setNewBody('')
      void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })
  const updateMutation = useMutation({
    mutationFn: async ({ id, body }: { id: string; body: string }) => (await apiRequest<ApiEnvelope<Note>>(`/notes/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify({ body_markdown: body }) })).data,
    onMutate: async ({ id, body }) => {
      await queryClient.cancelQueries({ queryKey: notesKey })
      const snapshot = queryClient.getQueryData<Note[]>(notesKey)
      queryClient.setQueryData<Note[]>(notesKey, (current = []) => current.map((note) => note.id === id ? { ...note, body_markdown: body } : note))
      return snapshot
    },
    onError: (_error, _variables, snapshot) => restoreNotes(queryClient, snapshot),
    onSuccess: (updated) => {
      queryClient.setQueryData<Note[]>(notesKey, (current = []) => current.map((note) => note.id === updated.id ? updated : note))
      setEditingID(null)
      void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })
  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiRequest<void>(`/notes/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: notesKey })
      const snapshot = queryClient.getQueryData<Note[]>(notesKey)
      queryClient.setQueryData<Note[]>(notesKey, (current = []) => current.filter((note) => note.id !== id))
      setConfirmID(null)
      return snapshot
    },
    onError: (_error, _id, snapshot) => restoreNotes(queryClient, snapshot),
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['dashboard'] }) },
  })

  function createNote(event: FormEvent) {
    event.preventDefault()
    const body = newBody.trim()
    if (body && !createMutation.isPending) createMutation.mutate(body)
  }

  function updateNote(event: FormEvent, id: string) {
    event.preventDefault()
    const body = editBody.trim()
    if (body && !updateMutation.isPending) updateMutation.mutate({ id, body })
  }

  function cancelDelete() {
    setConfirmID(null)
    deleteTrigger.current?.focus()
  }

  const topicNotes = (notes.data ?? []).filter((note) => note.topic_slug === slug)
  return <section id="notes" className="topic-notes" aria-labelledby="notes-heading">
    <p className="eyebrow">Личный конспект</p><h2 id="notes-heading">Заметки</h2>
    <form className="note-form" onSubmit={createNote}><label htmlFor="new-note">Новая заметка</label><textarea id="new-note" value={newBody} maxLength={20_000} onChange={(event) => setNewBody(event.target.value)} placeholder="Запишите формулировку, пример или вопрос…" required /><button type="submit" disabled={createMutation.isPending}>{createMutation.isPending ? 'Добавляем…' : 'Добавить заметку'}</button></form>
    {notes.isPending && <div className="notes-state" aria-live="polite">Загружаем заметки…</div>}
    {notes.isError && <div className="notes-state" role="alert">Не удалось загрузить заметки. <button type="button" onClick={() => void notes.refetch()}>Повторить</button></div>}
    {(createMutation.isError || updateMutation.isError || deleteMutation.isError) && <div className="mutation-error" role="alert">Не удалось сохранить заметку. Предыдущее состояние восстановлено.</div>}
    {!notes.isPending && !notes.isError && topicNotes.length === 0 && <p className="topic-empty">У этой темы пока нет заметок.</p>}
    <ul className="note-list">{topicNotes.map((note) => <li key={note.id}>
      {editingID === note.id
        ? <form className="note-form" onSubmit={(event) => updateNote(event, note.id)}><label htmlFor={`note-${note.id}`}>Текст заметки</label><textarea id={`note-${note.id}`} value={editBody} maxLength={20_000} onChange={(event) => setEditBody(event.target.value)} required /><div><button type="submit" disabled={updateMutation.isPending}>Сохранить</button><button type="button" onClick={() => setEditingID(null)}>Отмена</button></div></form>
        : <><SafeMarkdown>{note.body_markdown}</SafeMarkdown><div className="note-actions"><button type="button" onClick={() => { setEditingID(note.id); setEditBody(note.body_markdown) }}>Редактировать</button><button type="button" onClick={(event) => { deleteTrigger.current = event.currentTarget; setConfirmID(note.id) }}>Удалить</button></div></>}
      {confirmID === note.id && <div className="delete-confirm" role="alertdialog" aria-labelledby={`delete-title-${note.id}`} onKeyDown={(event) => { if (event.key === 'Escape') cancelDelete() }}><strong id={`delete-title-${note.id}`}>Удалить заметку?</strong><span>Это действие нельзя отменить.</span><div><button ref={confirmButton} type="button" onClick={() => deleteMutation.mutate(note.id)}>Да, удалить</button><button type="button" onClick={cancelDelete}>Отмена</button></div></div>}
    </li>)}</ul>
  </section>
}

type NotesQueryClient = ReturnType<typeof useQueryClient>

function restoreNotes(client: NotesQueryClient, snapshot: Note[] | undefined) {
  if (snapshot) client.setQueryData(notesKey, snapshot)
}
