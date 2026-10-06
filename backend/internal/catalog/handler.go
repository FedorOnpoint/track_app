package catalog

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/FedorOnpoint/track_app/backend/internal/catalog/catalogdb"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/requestcontext"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Store interface {
	ListCategories(context.Context) ([]catalogdb.Category, error)
	CountTopics(context.Context, catalogdb.CountTopicsParams) (int64, error)
	ListTopics(context.Context, catalogdb.ListTopicsParams) ([]catalogdb.ListTopicsRow, error)
	GetTopic(context.Context, string) (catalogdb.GetTopicRow, error)
	ListTopicSections(context.Context, pgtype.UUID) ([]catalogdb.ListTopicSectionsRow, error)
	ListChecklistItems(context.Context, pgtype.UUID) ([]catalogdb.ListChecklistItemsRow, error)
	ListInterviewQuestions(context.Context, pgtype.UUID) ([]catalogdb.ListInterviewQuestionsRow, error)
	ListFollowUpQuestions(context.Context, []pgtype.UUID) ([]catalogdb.FollowUpQuestion, error)
	ListResources(context.Context, pgtype.UUID) ([]catalogdb.ListResourcesRow, error)
	ListPracticeRecommendations(context.Context, pgtype.UUID) ([]catalogdb.ListPracticeRecommendationsRow, error)
}

type responseError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

type categoryDTO struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type topicSummaryDTO struct {
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	Title            string `json:"title"`
	Summary          string `json:"summary"`
	Difficulty       string `json:"difficulty"`
	EstimatedMinutes int32  `json:"estimated_minutes"`
	CategorySlug     string `json:"category_slug"`
	CategoryTitle    string `json:"category_title"`
}

func MountRoutes(router chi.Router, store Store) {
	router.Get("/categories", listCategories(store))
	router.Get("/topics", listTopics(store))
	router.Get("/topics/{slug}", getTopic(store))
}

func getTopic(store Store) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		row, err := store.GetTopic(request.Context(), chi.URLParam(request, "slug"))
		if err != nil {
			if err == pgx.ErrNoRows {
				writeCatalogError(writer, request, http.StatusNotFound, "topic_not_found", "Тема не найдена")
				return
			}
			writeCatalogError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось загрузить тему")
			return
		}
		sections, checklist, questions, followUps, resources, practice, err := loadTopicParts(request.Context(), store, row.ID)
		if err != nil {
			writeCatalogError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось загрузить тему")
			return
		}
		writeCatalogJSON(writer, http.StatusOK, map[string]any{"data": map[string]any{
			"id": uuidString(row.ID), "slug": row.Slug, "title": row.Title, "summary": row.Summary,
			"difficulty": row.Difficulty, "estimated_minutes": row.EstimatedMinutes,
			"category_slug": row.CategorySlug, "category_title": row.CategoryTitle,
			"sections": sectionDTOs(sections), "checklist": checklistDTOs(checklist), "questions": questionDTOs(questions),
			"follow_ups": followUpDTOs(followUps), "resources": resourceDTOs(resources), "practice": practiceDTOs(practice),
		}})
	}
}

func sectionDTOs(rows []catalogdb.ListTopicSectionsRow) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": uuidString(row.ID), "heading": row.Heading, "body_markdown": row.BodyMarkdown, "sort_order": row.SortOrder})
	}
	return items
}
func checklistDTOs(rows []catalogdb.ListChecklistItemsRow) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": uuidString(row.ID), "stable_key": row.StableKey, "text": row.Text, "sort_order": row.SortOrder})
	}
	return items
}
func questionDTOs(rows []catalogdb.ListInterviewQuestionsRow) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": uuidString(row.ID), "stable_key": row.StableKey, "question": row.Question, "short_answer": row.ShortAnswer, "detailed_answer": row.DetailedAnswer, "pitfalls": row.Pitfalls, "sort_order": row.SortOrder})
	}
	return items
}
func followUpDTOs(rows []catalogdb.FollowUpQuestion) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": uuidString(row.ID), "interview_question_id": uuidString(row.InterviewQuestionID), "question": row.Question, "expected_answer": row.ExpectedAnswer, "sort_order": row.SortOrder})
	}
	return items
}
func resourceDTOs(rows []catalogdb.ListResourcesRow) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": uuidString(row.ID), "title": row.Title, "url": row.Url, "resource_type": row.ResourceType, "source_name": row.SourceName, "sort_order": row.SortOrder})
	}
	return items
}
func practiceDTOs(rows []catalogdb.ListPracticeRecommendationsRow) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		var url any
		if row.Url.Valid {
			url = row.Url.String
		}
		items = append(items, map[string]any{"id": uuidString(row.ID), "title": row.Title, "description": row.Description, "url": url, "expected_outcome": row.ExpectedOutcome, "sort_order": row.SortOrder})
	}
	return items
}

func loadTopicParts(ctx context.Context, store Store, topicID pgtype.UUID) (
	[]catalogdb.ListTopicSectionsRow,
	[]catalogdb.ListChecklistItemsRow,
	[]catalogdb.ListInterviewQuestionsRow,
	[]catalogdb.FollowUpQuestion,
	[]catalogdb.ListResourcesRow,
	[]catalogdb.ListPracticeRecommendationsRow,
	error,
) {
	sections, err := store.ListTopicSections(ctx, topicID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	checklist, err := store.ListChecklistItems(ctx, topicID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	questions, err := store.ListInterviewQuestions(ctx, topicID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	questionIDs := make([]pgtype.UUID, 0, len(questions))
	for _, question := range questions {
		questionIDs = append(questionIDs, question.ID)
	}
	followUps, err := store.ListFollowUpQuestions(ctx, questionIDs)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	resources, err := store.ListResources(ctx, topicID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	practice, err := store.ListPracticeRecommendations(ctx, topicID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	return sections, checklist, questions, followUps, resources, practice, nil
}

func listCategories(store Store) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		rows, err := store.ListCategories(request.Context())
		if err != nil {
			writeCatalogError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось загрузить категории")
			return
		}
		data := make([]categoryDTO, 0, len(rows))
		for _, row := range rows {
			data = append(data, categoryDTO{ID: uuidString(row.ID), Slug: row.Slug, Title: row.Title, Description: row.Description})
		}
		writeCatalogJSON(writer, http.StatusOK, map[string]any{"data": data})
	}
}

func listTopics(store Store) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		params, page, pageSize, ok := parseListParams(writer, request)
		if !ok {
			return
		}
		countParams := catalogdb.CountTopicsParams{Query: params.Query, Category: params.Category, Difficulty: params.Difficulty}
		total, err := store.CountTopics(request.Context(), countParams)
		if err != nil {
			writeCatalogError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось загрузить темы")
			return
		}
		rows, err := store.ListTopics(request.Context(), params)
		if err != nil {
			writeCatalogError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось загрузить темы")
			return
		}
		data := make([]topicSummaryDTO, 0, len(rows))
		for _, row := range rows {
			data = append(data, topicSummaryDTO{ID: uuidString(row.ID), Slug: row.Slug, Title: row.Title, Summary: row.Summary, Difficulty: row.Difficulty, EstimatedMinutes: row.EstimatedMinutes, CategorySlug: row.CategorySlug, CategoryTitle: row.CategoryTitle})
		}
		totalPages := int64(0)
		if total > 0 {
			totalPages = int64(math.Ceil(float64(total) / float64(pageSize)))
		}
		writeCatalogJSON(writer, http.StatusOK, map[string]any{
			"data":       data,
			"pagination": map[string]any{"page": page, "page_size": pageSize, "total_items": total, "total_pages": totalPages},
		})
	}
}

func parseListParams(writer http.ResponseWriter, request *http.Request) (catalogdb.ListTopicsParams, int, int, bool) {
	query := request.URL.Query()
	for _, unsupported := range []string{"status", "bookmarked", "review_required"} {
		if query.Has(unsupported) {
			writeCatalogError(writer, request, http.StatusBadRequest, "validation_error", "Фильтр станет доступен после входа")
			return catalogdb.ListTopicsParams{}, 0, 0, false
		}
	}
	page, ok := positiveInt(query.Get("page"), 1)
	if !ok {
		writeCatalogError(writer, request, http.StatusBadRequest, "validation_error", "Некорректный номер страницы")
		return catalogdb.ListTopicsParams{}, 0, 0, false
	}
	pageSize, ok := positiveInt(query.Get("page_size"), 20)
	if !ok || pageSize > 100 {
		writeCatalogError(writer, request, http.StatusBadRequest, "validation_error", "Размер страницы должен быть от 1 до 100")
		return catalogdb.ListTopicsParams{}, 0, 0, false
	}
	difficulty := strings.TrimSpace(query.Get("difficulty"))
	if difficulty != "" && difficulty != "basic" && difficulty != "intermediate" && difficulty != "advanced" {
		writeCatalogError(writer, request, http.StatusBadRequest, "validation_error", "Неизвестная сложность")
		return catalogdb.ListTopicsParams{}, 0, 0, false
	}
	sort := query.Get("sort")
	if sort == "" {
		sort = "category"
	}
	if sort != "category" && sort != "title" && sort != "difficulty" {
		writeCatalogError(writer, request, http.StatusBadRequest, "validation_error", "Неизвестная сортировка")
		return catalogdb.ListTopicsParams{}, 0, 0, false
	}
	return catalogdb.ListTopicsParams{
		Query: nullableText(strings.TrimSpace(query.Get("query"))), Category: nullableText(strings.TrimSpace(query.Get("category"))),
		Difficulty: nullableText(difficulty), Sort: sort, PageOffset: int32((page - 1) * pageSize), PageSize: int32(pageSize),
	}, page, pageSize, true
}

func positiveInt(raw string, fallback int) (int, bool) {
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value > 0
}

func nullableText(value string) pgtype.Text { return pgtype.Text{String: value, Valid: value != ""} }

func uuidString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func writeCatalogError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	response := responseError{}
	response.Error.Code = code
	response.Error.Message = message
	response.Error.RequestID = requestcontext.RequestID(request.Context())
	writeCatalogJSON(writer, status, response)
}

func writeCatalogJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
