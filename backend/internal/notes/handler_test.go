package notes

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeStore struct {
	note  Note
	items []Note
	err   error
}

func (s *fakeStore) List(context.Context, uuid.UUID) ([]Note, error) { return s.items, s.err }
func (s *fakeStore) Create(context.Context, uuid.UUID, string, string, time.Time) (Note, error) {
	return s.note, s.err
}
func (s *fakeStore) Update(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (Note, error) {
	return s.note, s.err
}
func (s *fakeStore) Delete(context.Context, uuid.UUID, uuid.UUID) error { return s.err }

func TestCreateNoteValidatesBody(t *testing.T) {
	for _, test := range []struct {
		name, body string
		want       int
	}{
		{name: "valid", body: `{"body_markdown":"Конспект"}`, want: http.StatusCreated},
		{name: "blank", body: `{"body_markdown":"  "}`, want: http.StatusBadRequest},
		{name: "unknown field", body: `{"body_markdown":"ok","extra":true}`, want: http.StatusBadRequest},
		{name: "too long", body: `{"body_markdown":"` + strings.Repeat("я", maxNoteRunes+1) + `"}`, want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{note: Note{ID: uuid.New(), TopicID: uuid.New(), BodyMarkdown: "Конспект", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
			router := chi.NewRouter()
			MountRoutes(router, store)
			request := httptest.NewRequest(http.MethodPost, "/topics/goroutines/notes", bytes.NewBufferString(test.body))
			request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestForeignOrMissingNoteUsesSameNotFound(t *testing.T) {
	store := &fakeStore{err: pgx.ErrNoRows}
	router := chi.NewRouter()
	MountRoutes(router, store)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		var body *bytes.Reader
		if method == http.MethodPut {
			body = bytes.NewReader([]byte(`{"body_markdown":"updated"}`))
		} else {
			body = bytes.NewReader(nil)
		}
		request := httptest.NewRequest(method, "/notes/"+uuid.NewString(), body)
		request = request.WithContext(auth.ContextWithUserID(request.Context(), uuid.New()))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"note_not_found"`)) {
			t.Fatalf("%s status=%d body=%s", method, response.Code, response.Body.String())
		}
	}
}

func TestNotesRequireAuthentication(t *testing.T) {
	router := chi.NewRouter()
	MountRoutes(router, &fakeStore{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/notes", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.Code)
	}
}
