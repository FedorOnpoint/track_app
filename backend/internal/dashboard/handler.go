package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/requestcontext"
	"github.com/go-chi/chi/v5"
)

func MountRoutes(router chi.Router, store Store) { router.Get("/dashboard", get(store)) }
func get(store Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		value, err := store.Get(r.Context(), userID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось загрузить главную")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": dashboardDTO(value)})
	}
}
func dashboardDTO(value Dashboard) map[string]any {
	percent := func(mastered, total int64) float64 {
		if total == 0 {
			return 0
		}
		return float64(mastered) * 100 / float64(total)
	}
	categories := make([]map[string]any, 0, len(value.Categories))
	for _, item := range value.Categories {
		categories = append(categories, map[string]any{"slug": item.Slug, "title": item.Title, "total": item.Total, "mastered": item.Mastered, "percent": percent(item.Mastered, item.Total)})
	}
	topics := func(items []TopicItem) []map[string]any {
		result := make([]map[string]any, 0, len(items))
		for _, item := range items {
			result = append(result, topicDTO(item))
		}
		return result
	}
	notes := make([]map[string]any, 0, len(value.RecentNotes))
	for _, item := range value.RecentNotes {
		notes = append(notes, map[string]any{"id": item.ID.String(), "topic_slug": item.TopicSlug, "topic_title": item.TopicTitle, "body_markdown": item.BodyMarkdown, "updated_at": item.UpdatedAt})
	}
	var continuation any
	if value.ContinueTopic != nil {
		continuation = topicDTO(*value.ContinueTopic)
	}
	return map[string]any{"overall": map[string]any{"total": value.Overall.Total, "mastered": value.Overall.Mastered, "percent": percent(value.Overall.Mastered, value.Overall.Total)}, "categories": categories, "review_topics": topics(value.ReviewTopics), "recent_topics": topics(value.RecentTopics), "recent_notes": notes, "continue_topic": continuation}
}
func topicDTO(item TopicItem) map[string]any {
	return map[string]any{"slug": item.Slug, "title": item.Title, "status": item.Status, "review_required": item.ReviewRequired, "updated_at": item.UpdatedAt, "last_opened_at": item.LastOpenedAt}
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": requestcontext.RequestID(r.Context())}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
