package notes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/requestcontext"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	maxNoteRunes     = 20_000
	maxNoteBodyBytes = 128 << 10
)

func MountRoutes(router chi.Router, store Store) {
	router.Get("/notes", list(store))
	router.Post("/topics/{slug}/notes", create(store))
	router.Put("/notes/{id}", update(store))
	router.Delete("/notes/{id}", remove(store))
}

func list(store Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		items, err := store.List(r.Context(), userID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось загрузить заметки")
			return
		}
		data := make([]map[string]any, 0, len(items))
		for _, item := range items {
			data = append(data, noteDTO(item))
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	}
}

func create(store Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		body, ok := decodeBody(w, r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Некорректный текст заметки")
			return
		}
		note, err := store.Create(r.Context(), userID, chi.URLParam(r, "slug"), body, time.Now().UTC())
		if !handleMutationError(w, r, err, "topic_not_found", "Тема не найдена") {
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": noteDTO(note)})
	}
}

func update(store Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		noteID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Некорректный идентификатор заметки")
			return
		}
		body, ok := decodeBody(w, r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Некорректный текст заметки")
			return
		}
		note, err := store.Update(r.Context(), userID, noteID, body, time.Now().UTC())
		if !handleMutationError(w, r, err, "note_not_found", "Заметка не найдена") {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": noteDTO(note)})
	}
}

func remove(store Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		noteID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Некорректный идентификатор заметки")
			return
		}
		if !handleMutationError(w, r, store.Delete(r.Context(), userID, noteID), "note_not_found", "Заметка не найдена") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func decodeBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxNoteBodyBytes)
	var input struct {
		BodyMarkdown string `json:"body_markdown"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || strings.TrimSpace(input.BodyMarkdown) == "" || utf8.RuneCountInString(input.BodyMarkdown) > maxNoteRunes {
		return "", false
	}
	return input.BodyMarkdown, true
}

func handleMutationError(w http.ResponseWriter, r *http.Request, err error, code, message string) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, r, http.StatusNotFound, code, message)
		return false
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось сохранить заметку")
		return false
	}
	return true
}

func noteDTO(note Note) map[string]any {
	return map[string]any{"id": note.ID.String(), "topic_id": note.TopicID.String(), "topic_slug": note.TopicSlug, "topic_title": note.TopicTitle, "body_markdown": note.BodyMarkdown, "created_at": note.CreatedAt, "updated_at": note.UpdatedAt}
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": requestcontext.RequestID(r.Context())}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
