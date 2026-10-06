package bookmarks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/requestcontext"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func MountRoutes(router chi.Router, store Store) {
	router.Get("/bookmarks", list(store))
	router.Put("/topics/{slug}/bookmark", set(store))
	router.Delete("/topics/{slug}/bookmark", remove(store))
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
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось загрузить закладки")
			return
		}
		data := make([]map[string]any, 0, len(items))
		for _, item := range items {
			data = append(data, map[string]any{"topic_id": item.TopicID.String(), "slug": item.Slug, "title": item.Title, "summary": item.Summary, "difficulty": item.Difficulty, "estimated_minutes": item.EstimatedMinutes, "category_slug": item.CategorySlug, "category_title": item.CategoryTitle, "created_at": item.CreatedAt})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	}
}

func set(store Store) http.HandlerFunc {
	return mutate(store.Set, "Не удалось добавить закладку")
}
func remove(store Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		err := store.Delete(r.Context(), userID, chi.URLParam(r, "slug"))
		finishMutation(w, r, err, "Не удалось удалить закладку")
	}
}
func mutate(operation func(context.Context, uuid.UUID, string, time.Time) error, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		finishMutation(w, r, operation(r.Context(), userID, chi.URLParam(r, "slug"), time.Now().UTC()), message)
	}
}

func finishMutation(w http.ResponseWriter, r *http.Request, err error, message string) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, r, http.StatusNotFound, "topic_not_found", "Тема не найдена")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", message)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": requestcontext.RequestID(r.Context())}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
