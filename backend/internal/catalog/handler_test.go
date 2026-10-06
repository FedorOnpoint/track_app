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

type fakePersonalizedStore struct {
	params UserTopicParams
	rows   []UserTopic
	total  int64
}

func (store *fakePersonalizedStore) CountUserTopics(_ context.Context, params UserTopicParams) (int64, error) {
	store.params = params
	return store.total, nil
}
func (store *fakePersonalizedStore) ListUserTopics(_ context.Context, params UserTopicParams) ([]UserTopic, error) {
	store.params = params
	return store.rows, nil
}

type recordingVisit struct {
	userID uuid.UUID
	slug   string
}

type fakeTopicStateStore struct {
	state UserTopicState
}

func (store *fakeTopicStateStore) GetTopicState(context.Context, uuid.UUID, string) (UserTopicState, error) {
	return store.state, nil
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

func TestTopicDetailContainsAuthenticatedUserState(t *testing.T) {
	itemID := uuid.New()
	router := chi.NewRouter()
	MountRoutesWithUserData(router, &fakeStore{found: true}, nil, &fakePersonalizedStore{}, &fakeTopicStateStore{state: UserTopicState{
		Status: "learning", ReviewRequired: true, Bookmarked: true, Checklist: map[uuid.UUID]bool{itemID: true},
	}})
	request := httptest.NewRequest(http.MethodGet, "/topics/goroutines", nil)
	request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for _, fragment := range []string{`"status":"learning"`, `"review_required":true`, `"bookmarked":true`, `"` + itemID.String() + `":true`} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Errorf("response missing %s: %s", fragment, response.Body.String())
		}
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

func TestPersonalizedTopicsValidateAndForwardFilters(t *testing.T) {
	userID := uuid.New()
	personalized := &fakePersonalizedStore{total: 1, rows: []UserTopic{{ID: uuid.New(), Slug: "goroutines", Status: "review", ReviewRequired: true, Bookmarked: true}}}
	router := chi.NewRouter()
	MountRoutesWithUserData(router, &fakeStore{}, nil, personalized, nil)
	request := httptest.NewRequest(http.MethodGet, "/topics?status=review&bookmarked=true&review_required=true&page=2&page_size=10&sort=title", nil)
	request = request.WithContext(auth.ContextWithUserID(request.Context(), userID))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if personalized.params.UserID != userID || personalized.params.Status == nil || *personalized.params.Status != "review" || personalized.params.Bookmarked == nil || !*personalized.params.Bookmarked || personalized.params.PageOffset != 10 {
		t.Fatalf("params=%+v", personalized.params)
	}
	for _, fragment := range []string{`"status":"review"`, `"review_required":true`, `"bookmarked":true`} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Errorf("missing %s: %s", fragment, response.Body.String())
		}
	}
}

func TestReviewForcesReviewFilter(t *testing.T) {
	personalized := &fakePersonalizedStore{}
	router := chi.NewRouter()
	MountRoutesWithUserData(router, &fakeStore{}, nil, personalized, nil)
	request := httptest.NewRequest(http.MethodGet, "/review", nil)
	request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || personalized.params.ReviewRequired == nil || !*personalized.params.ReviewRequired {
		t.Fatalf("status=%d params=%+v", response.Code, personalized.params)
	}
}

func TestPersonalizedFiltersRejectInvalidValues(t *testing.T) {
	for _, path := range []string{"/topics?status=unknown", "/topics?bookmarked=maybe", "/topics?review_required=1", "/topics?query=" + strings.Repeat("я", 201)} {
		router := chi.NewRouter()
		MountRoutesWithUserData(router, &fakeStore{}, nil, &fakePersonalizedStore{}, nil)
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d", path, response.Code)
		}
	}
}
