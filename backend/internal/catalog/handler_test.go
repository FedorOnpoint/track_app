package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/catalog/catalogdb"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeStore struct {
	listParams catalogdb.ListTopicsParams
	found      bool
}

type recordingVisit struct {
	userID uuid.UUID
	slug   string
}

func (visit *recordingVisit) RecordTopicVisit(_ context.Context, userID uuid.UUID, slug string, _ time.Time) error {
	visit.userID = userID
	visit.slug = slug
	return nil
}

func (store *fakeStore) ListCategories(context.Context) ([]catalogdb.Category, error) {
	return []catalogdb.Category{{Slug: "concurrency", Title: "Конкурентность", Description: "Горутины", SortOrder: 10}}, nil
}
func (store *fakeStore) CountTopics(context.Context, catalogdb.CountTopicsParams) (int64, error) {
	return 1, nil
}
func (store *fakeStore) ListTopics(_ context.Context, params catalogdb.ListTopicsParams) ([]catalogdb.ListTopicsRow, error) {
	store.listParams = params
	return []catalogdb.ListTopicsRow{{Slug: "goroutines", Title: "Горутины", Difficulty: "intermediate", EstimatedMinutes: 35, CategorySlug: "concurrency", CategoryTitle: "Конкурентность"}}, nil
}
func (store *fakeStore) GetTopic(context.Context, string) (catalogdb.GetTopicRow, error) {
	if store.found {
		return catalogdb.GetTopicRow{Slug: "goroutines", Title: "Горутины", Difficulty: "intermediate", CategorySlug: "concurrency"}, nil
	}
	return catalogdb.GetTopicRow{}, pgx.ErrNoRows
}
func (store *fakeStore) ListTopicSections(context.Context, pgtype.UUID) ([]catalogdb.ListTopicSectionsRow, error) {
	return []catalogdb.ListTopicSectionsRow{{Heading: "Теория", BodyMarkdown: "Текст"}}, nil
}
func (store *fakeStore) ListChecklistItems(context.Context, pgtype.UUID) ([]catalogdb.ListChecklistItemsRow, error) {
	return nil, nil
}
func (store *fakeStore) ListInterviewQuestions(context.Context, pgtype.UUID) ([]catalogdb.ListInterviewQuestionsRow, error) {
	return []catalogdb.ListInterviewQuestionsRow{{Question: "Вопрос", Pitfalls: "Ловушка"}}, nil
}
func (store *fakeStore) ListFollowUpQuestions(context.Context, []pgtype.UUID) ([]catalogdb.FollowUpQuestion, error) {
	return nil, nil
}
func (store *fakeStore) ListResources(context.Context, pgtype.UUID) ([]catalogdb.ListResourcesRow, error) {
	return nil, nil
}
func (store *fakeStore) ListPracticeRecommendations(context.Context, pgtype.UUID) ([]catalogdb.ListPracticeRecommendationsRow, error) {
	return nil, nil
}

func TestListTopicsValidatesAndCapsPagination(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want int
	}{
		{name: "invalid page", url: "/topics?page=0", want: http.StatusBadRequest},
		{name: "invalid page size", url: "/topics?page_size=101", want: http.StatusBadRequest},
		{name: "invalid difficulty", url: "/topics?difficulty=expert", want: http.StatusBadRequest},
		{name: "invalid sort", url: "/topics?sort=newest", want: http.StatusBadRequest},
		{name: "unsupported authenticated filter", url: "/topics?bookmarked=true", want: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := chi.NewRouter()
			MountRoutes(router, &fakeStore{})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.url, nil))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestUnknownTopicReturnsSafeNotFound(t *testing.T) {
	router := chi.NewRouter()
	MountRoutes(router, &fakeStore{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/topics/unknown", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "no rows") {
		t.Fatalf("response exposes internal error: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"code":"topic_not_found"`) {
		t.Fatalf("response has no safe error envelope: %s", response.Body.String())
	}
}

func TestTopicDetailContainsCompleteLearningPayload(t *testing.T) {
	router := chi.NewRouter()
	MountRoutes(router, &fakeStore{found: true})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/topics/goroutines", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	for _, field := range []string{`"sections"`, `"checklist"`, `"questions"`, `"follow_ups"`, `"resources"`, `"practice"`, `"pitfalls"`} {
		if !strings.Contains(response.Body.String(), field) {
			t.Errorf("response missing %s: %s", field, response.Body.String())
		}
	}
}

func TestTopicDetailRecordsAuthenticatedVisit(t *testing.T) {
	visitor := &recordingVisit{}
	userID := uuid.New()
	router := chi.NewRouter()
	MountRoutes(router, &fakeStore{found: true}, visitor)
	request := httptest.NewRequest(http.MethodGet, "/topics/goroutines", nil)
	request = request.WithContext(auth.ContextWithUserID(request.Context(), userID))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || visitor.userID != userID || visitor.slug != "goroutines" {
		t.Fatalf("status=%d visit=%+v", response.Code, visitor)
	}
}

func TestListTopicsReturnsEnvelope(t *testing.T) {
	store := &fakeStore{}
	router := chi.NewRouter()
	MountRoutes(router, store)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/topics?query=goroutine&page=2&page_size=10&sort=title", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if store.listParams.PageOffset != 10 || store.listParams.PageSize != 10 || store.listParams.Sort != "title" {
		t.Fatalf("params = %#v", store.listParams)
	}
	if store.listParams.Query != (pgtype.Text{String: "goroutine", Valid: true}) {
		t.Fatalf("query = %#v", store.listParams.Query)
	}
	var body struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
		Pagination struct {
			TotalItems int64 `json:"total_items"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Slug != "goroutines" || body.Pagination.TotalItems != 1 {
		t.Fatalf("body = %#v", body)
	}
}
