package progress

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/requestcontext"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func MountRoutes(router chi.Router, store Store) {
	router.Put("/topics/{slug}/progress", setTopicProgress(store))
	router.Put("/topics/{slug}/checklist/{item_id}", setChecklistProgress(store))
}

func setTopicProgress(store Store) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		userID, ok := auth.UserIDFromContext(request.Context())
		if !ok {
			writeError(writer, request, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		var input struct {
			Status         string `json:"status"`
			ReviewRequired bool   `json:"review_required"`
		}
		if !decodeJSON(writer, request, &input) || !validStatus(input.Status) {
			writeError(writer, request, http.StatusBadRequest, "validation_error", "Некорректное состояние прогресса")
			return
		}
		result, err := store.SetTopicProgress(request.Context(), userID, chi.URLParam(request, "slug"), input.Status, input.ReviewRequired, time.Now().UTC())
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(writer, request, http.StatusNotFound, "topic_not_found", "Тема не найдена")
			return
		}
		if err != nil {
			writeError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось сохранить прогресс")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"data": topicProgressDTO(result)})
	}
}

func setChecklistProgress(store Store) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		userID, ok := auth.UserIDFromContext(request.Context())
		itemID, err := uuid.Parse(chi.URLParam(request, "item_id"))
		if !ok {
			writeError(writer, request, http.StatusUnauthorized, "unauthorized", "Требуется вход")
			return
		}
		if err != nil {
			writeError(writer, request, http.StatusBadRequest, "validation_error", "Некорректный идентификатор checklist")
			return
		}
		var input struct {
			Completed *bool `json:"completed"`
		}
		if !decodeJSON(writer, request, &input) || input.Completed == nil {
			writeError(writer, request, http.StatusBadRequest, "validation_error", "Некорректное состояние checklist")
			return
		}
		result, err := store.SetChecklistProgress(request.Context(), userID, chi.URLParam(request, "slug"), itemID, *input.Completed, time.Now().UTC())
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(writer, request, http.StatusNotFound, "checklist_item_not_found", "Пункт checklist не найден")
			return
		}
		if err != nil {
			writeError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось сохранить checklist")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"data": map[string]any{"checklist_item_id": result.ChecklistItemID.String(), "completed": result.Completed, "updated_at": result.UpdatedAt}})
	}
}

func validStatus(value string) bool {
	return value == "not_started" || value == "learning" || value == "review" || value == "mastered"
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 4096)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target) == nil && decoder.Decode(&struct{}{}) == io.EOF
}

func topicProgressDTO(value TopicProgress) map[string]any {
	return map[string]any{"topic_id": value.TopicID.String(), "status": value.Status, "review_required": value.ReviewRequired, "last_opened_at": value.LastOpenedAt, "updated_at": value.UpdatedAt}
}

func writeError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	writeJSON(writer, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": requestcontext.RequestID(request.Context())}})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
