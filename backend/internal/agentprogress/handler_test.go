package agentprogress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type fakeStore struct {
	user  uuid.UUID
	calls int
}

func (s *fakeStore) Topics(_ context.Context, user uuid.UUID) ([]Topic, error) {
	s.user = user
	s.calls++
	return []Topic{}, nil
}
func (s *fakeStore) Apply(_ context.Context, user uuid.UUID, slug, key string, input Update) (Result, error) {
	s.user = user
	s.calls++
	return Result{Slug: slug, UpdateID: key, Status: "learning"}, nil
}

type ready struct{}

func (ready) Ping(context.Context) error { return nil }

func TestAgentAuthenticationAndBoundary(t *testing.T) {
	token := strings.Repeat("a", 64)
	hash := sha256.Sum256([]byte(token))
	owner := uuid.New()
	for _, tc := range []struct {
		name, path, token string
		expired           bool
		want              int
	}{
		{"valid", "/api/v1/agent/topics/go-maps/updates/session-1", token, false, 200},
		{"missing", "/api/v1/agent/topics/go-maps/updates/session-1", "", false, 401},
		{"wrong", "/api/v1/agent/topics/go-maps/updates/session-1", "wrong", false, 401},
		{"expired", "/api/v1/agent/topics/go-maps/updates/session-1", token, true, 401},
		{"cookie_route_still_requires_origin", "/api/v1/topics/go-maps/progress", token, false, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{}
			expires := time.Now().Add(time.Hour)
			if tc.expired {
				expires = time.Now().Add(-time.Hour)
			}
			credentials := Credentials{UserID: owner, TokenHash: hash, ExpiresAt: expires, Enabled: true}
			h := httpserver.NewHandler(ready{}, slog.New(slog.NewTextHandler(io.Discard, nil)), httpserver.SecurityConfig{Environment: "production", Domain: "example.com"}, func(r chi.Router) { Mount(r, store, credentials) })
			req := httptest.NewRequest(http.MethodPut, tc.path, strings.NewReader(`{"status":"learning"}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			req.AddCookie(&http.Cookie{Name: "track_session", Value: "not-an-agent-token"})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.want == 200 && (store.user != owner || store.calls != 1) {
				t.Fatal("not scoped to credential owner")
			}
			if tc.want != 200 && store.calls != 0 {
				t.Fatal("unauthorized store access")
			}
		})
	}
}

func TestCredentialsFailClosed(t *testing.T) {
	hash := sha256.Sum256([]byte("secret"))
	valid := map[string]string{"AGENT_API_TOKEN_SHA256": hex.EncodeToString(hash[:]), "AGENT_USER_ID": uuid.NewString(), "AGENT_TOKEN_EXPIRES_AT": "2099-01-01T00:00:00Z"}
	for _, key := range []string{"AGENT_API_TOKEN_SHA256", "AGENT_USER_ID", "AGENT_TOKEN_EXPIRES_AT"} {
		values := map[string]string{}
		for k, v := range valid {
			values[k] = v
		}
		values[key] = "invalid"
		if _, err := LoadCredentials(func(k string) string { return values[k] }); err == nil {
			t.Fatalf("accepted invalid %s", key)
		}
		delete(values, key)
		if _, err := LoadCredentials(func(k string) string { return values[k] }); err == nil {
			t.Fatalf("accepted missing %s", key)
		}
	}
	if c, err := LoadCredentials(func(string) string { return "" }); err != nil || c.Enabled {
		t.Fatal("disabled configuration must work")
	}
	if c, err := LoadCredentials(func(k string) string { return valid[k] }); err != nil || !c.Enabled {
		t.Fatalf("valid config: %v", err)
	}
}

func TestUpdateValidation(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"status":"unknown"}`, `{"user_id":"other","status":"learning"}`, `{"checklist":{"key":null}}`, `{"summary":" "}`, `{"status":"learning"} {}`, `{"checklist":{"":true}}`} {
		r := httptest.NewRequest("PUT", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if _, err := decodeUpdate(httptest.NewRecorder(), r); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
	r := httptest.NewRequest("PUT", "/", strings.NewReader(`{"status":"learning","checklist":{"nil-map":true},"summary":"Learned maps"}`))
	r.Header.Set("Content-Type", "application/json")
	if _, err := decodeUpdate(httptest.NewRecorder(), r); err != nil {
		t.Fatal(err)
	}
}
