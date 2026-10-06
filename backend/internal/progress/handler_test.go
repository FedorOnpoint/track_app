package progress

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeStore struct {
	topicResult     TopicProgress
	checklistResult ChecklistProgress
	err             error
}

func (store *fakeStore) SetTopicProgress(context.Context, uuid.UUID, string, string, bool, time.Time) (TopicProgress, error) {
	return store.topicResult, store.err
}
func (store *fakeStore) SetChecklistProgress(context.Context, uuid.UUID, string, uuid.UUID, bool, time.Time) (ChecklistProgress, error) {
	return store.checklistResult, store.err
}

func TestSetTopicProgressContract(t *testing.T) {
	topicID := uuid.New()
	userID := uuid.New()
	tests := []struct {
		name       string
		body       string
		authorized bool
		store      *fakeStore
		wantStatus int
	}{
		{name: "success", body: `{"status":"learning","review_required":true}`, authorized: true, store: &fakeStore{topicResult: TopicProgress{TopicID: topicID, Status: "learning", ReviewRequired: true, UpdatedAt: time.Now()}}, wantStatus: http.StatusOK},
		{name: "invalid status", body: `{"status":"unknown","review_required":false}`, authorized: true, store: &fakeStore{}, wantStatus: http.StatusBadRequest},
		{name: "unknown field", body: `{"status":"learning","review_required":false,"extra":1}`, authorized: true, store: &fakeStore{}, wantStatus: http.StatusBadRequest},
		{name: "missing auth", body: `{"status":"learning","review_required":false}`, store: &fakeStore{}, wantStatus: http.StatusUnauthorized},
		{name: "unknown topic", body: `{"status":"learning","review_required":false}`, authorized: true, store: &fakeStore{err: pgx.ErrNoRows}, wantStatus: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := chi.NewRouter()
			MountRoutes(router, test.store)
			request := httptest.NewRequest(http.MethodPut, "/topics/goroutines/progress", bytes.NewBufferString(test.body))
			if test.authorized {
				request = request.WithContext(auth.ContextWithUserID(request.Context(), userID))
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestSetChecklistProgressContract(t *testing.T) {
	itemID := uuid.New()
	store := &fakeStore{checklistResult: ChecklistProgress{ChecklistItemID: itemID, Completed: true, UpdatedAt: time.Now()}}
	router := chi.NewRouter()
	MountRoutes(router, store)
	request := httptest.NewRequest(http.MethodPut, "/topics/goroutines/checklist/"+itemID.String(), bytes.NewBufferString(`{"completed":true}`))
	request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"completed":true`)) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSetChecklistProgressRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct{ name, itemID, body string }{
		{name: "invalid uuid", itemID: "not-a-uuid", body: `{"completed":true}`},
		{name: "missing completed", itemID: uuid.NewString(), body: `{}`},
		{name: "unknown field", itemID: uuid.NewString(), body: `{"completed":true,"extra":1}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := chi.NewRouter()
			MountRoutes(router, &fakeStore{err: errors.New("must not reach store")})
			request := httptest.NewRequest(http.MethodPut, "/topics/goroutines/checklist/"+test.itemID, bytes.NewBufferString(test.body))
			request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d, want 400; body=%s", response.Code, response.Body.String())
			}
		})
	}
}
