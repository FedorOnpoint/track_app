package bookmarks

import (
	"context"
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
	items                 []Bookmark
	err                   error
	setCalls, deleteCalls int
}

func (s *fakeStore) List(context.Context, uuid.UUID) ([]Bookmark, error) { return s.items, s.err }
func (s *fakeStore) Set(context.Context, uuid.UUID, string, time.Time) error {
	s.setCalls++
	return s.err
}
func (s *fakeStore) Delete(context.Context, uuid.UUID, string) error { s.deleteCalls++; return s.err }

func TestBookmarkRoutesRequireUser(t *testing.T) {
	router := chi.NewRouter()
	MountRoutes(router, &fakeStore{})
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/bookmarks", nil),
		httptest.NewRequest(http.MethodPut, "/topics/goroutines/bookmark", nil),
		httptest.NewRequest(http.MethodDelete, "/topics/goroutines/bookmark", nil),
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s status=%d", request.Method, response.Code)
		}
	}
}

func TestBookmarkMutationsAreNoContent(t *testing.T) {
	store := &fakeStore{}
	router := chi.NewRouter()
	MountRoutes(router, store)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		request := httptest.NewRequest(method, "/topics/goroutines/bookmark", nil)
		request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s status=%d body=%s", method, response.Code, response.Body.String())
		}
	}
	if store.setCalls != 1 || store.deleteCalls != 1 {
		t.Fatalf("calls set=%d delete=%d", store.setCalls, store.deleteCalls)
	}
}

func TestBookmarkUnknownTopicIsNotFound(t *testing.T) {
	router := chi.NewRouter()
	MountRoutes(router, &fakeStore{err: pgx.ErrNoRows})
	request := httptest.NewRequest(http.MethodPut, "/topics/missing/bookmark", nil)
	request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestListBookmarksReturnsStableShape(t *testing.T) {
	item := Bookmark{TopicID: uuid.New(), Slug: "goroutines", Title: "Goroutines", CreatedAt: time.Now()}
	router := chi.NewRouter()
	MountRoutes(router, &fakeStore{items: []Bookmark{item}})
	request := httptest.NewRequest(http.MethodGet, "/bookmarks", nil)
	request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
