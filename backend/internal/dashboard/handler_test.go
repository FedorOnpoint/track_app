package dashboard

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type fakeStore struct {
	value Dashboard
	err   error
}

func (store *fakeStore) Get(context.Context, uuid.UUID) (Dashboard, error) {
	return store.value, store.err
}

func TestDashboardPercentagesHandleEmptyAndFull(t *testing.T) {
	store := &fakeStore{value: Dashboard{Overall: ProgressSummary{Total: 2, Mastered: 1}, Categories: []CategoryProgress{{Slug: "empty", Title: "Empty"}, {Slug: "full", Title: "Full", Total: 1, Mastered: 1}}}}
	router := chi.NewRouter()
	MountRoutes(router, store)
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for _, fragment := range []string{`"percent":50`, `"slug":"empty"`, `"percent":0`, `"slug":"full"`, `"percent":100`} {
		if !bytes.Contains(response.Body.Bytes(), []byte(fragment)) {
			t.Errorf("body lacks %s: %s", fragment, response.Body.String())
		}
	}
}

func TestDashboardRequiresAuthentication(t *testing.T) {
	router := chi.NewRouter()
	MountRoutes(router, &fakeStore{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.Code)
	}
}
