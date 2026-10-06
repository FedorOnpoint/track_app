package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
)

type readyCheckerFunc func(context.Context) error

func (fn readyCheckerFunc) Ping(ctx context.Context) error {
	return fn(ctx)
}

func TestLivenessDoesNotCallDatabase(t *testing.T) {
	t.Parallel()

	checker := readyCheckerFunc(func(context.Context) error {
		t.Fatal("liveness must not call the database")
		return nil
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)

	httpserver.NewHandler(checker, discardLogger(), httpserver.SecurityConfig{Environment: "development", Domain: "localhost"}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("X-Request-ID is empty")
	}
}

func TestReadinessReturnsServiceUnavailableWithSafeEnvelope(t *testing.T) {
	t.Parallel()

	checker := readyCheckerFunc(func(context.Context) error {
		return errors.New("postgres password leaked in internal error")
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil)

	httpserver.NewHandler(checker, discardLogger(), httpserver.SecurityConfig{Environment: "development", Domain: "localhost"}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "service_unavailable" {
		t.Errorf("error code = %q, want service_unavailable", body.Error.Code)
	}
	if body.Error.Message != "Сервис временно недоступен" {
		t.Errorf("message = %q, want safe public message", body.Error.Message)
	}
	if body.Error.RequestID == "" {
		t.Error("request_id is empty")
	}
	if body.Error.RequestID != recorder.Header().Get("X-Request-ID") {
		t.Errorf("body request_id = %q, header = %q", body.Error.RequestID, recorder.Header().Get("X-Request-ID"))
	}
}

func TestReadinessReturnsOKWhenDatabaseResponds(t *testing.T) {
	t.Parallel()

	checker := readyCheckerFunc(func(context.Context) error { return nil })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil)

	httpserver.NewHandler(checker, discardLogger(), httpserver.SecurityConfig{Environment: "development", Domain: "localhost"}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRequestIDDoesNotTrustClientValue(t *testing.T) {
	t.Parallel()

	checker := readyCheckerFunc(func(context.Context) error { return nil })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	request.Header.Set("X-Request-ID", "attacker-controlled\nvalue")

	httpserver.NewHandler(checker, discardLogger(), httpserver.SecurityConfig{Environment: "development", Domain: "localhost"}).ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got == "" || got == request.Header.Get("X-Request-ID") {
		t.Fatalf("X-Request-ID = %q, want a server-generated value", got)
	}
}

func TestRequestLogUsesRoutePatternAndOmitsQuery(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	checker := readyCheckerFunc(func(context.Context) error { return nil })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/live?note=private", nil)

	httpserver.NewHandler(checker, logger, httpserver.SecurityConfig{Environment: "development", Domain: "localhost"}).ServeHTTP(recorder, request)

	var entry map[string]any
	if err := json.NewDecoder(&output).Decode(&entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	for _, field := range []string{"request_id", "method", "path", "status", "duration_ms"} {
		if _, ok := entry[field]; !ok {
			t.Errorf("log has no %q field: %v", field, entry)
		}
	}
	if got := entry["path"]; got != "/api/v1/health/live" {
		t.Errorf("path = %v, want route pattern", got)
	}
	if strings.Contains(output.String(), "private") {
		t.Errorf("log contains query value: %s", output.String())
	}
}

func TestUnsafeRequestOriginPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		security   httpserver.SecurityConfig
		origin     string
		referer    string
		wantStatus int
	}{
		{name: "production origin", security: httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"}, origin: "https://interview.example.com", wantStatus: http.StatusNoContent},
		{name: "production referer fallback", security: httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"}, referer: "https://interview.example.com/settings", wantStatus: http.StatusNoContent},
		{name: "foreign production origin", security: httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"}, origin: "https://evil.example", wantStatus: http.StatusForbidden},
		{name: "production subdomain confusion", security: httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"}, origin: "https://interview.example.com.evil.example", wantStatus: http.StatusForbidden},
		{name: "production userinfo confusion", security: httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"}, origin: "https://interview.example.com@evil.example", wantStatus: http.StatusForbidden},
		{name: "origin query is malformed", security: httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"}, origin: "https://interview.example.com?redirect=evil", wantStatus: http.StatusForbidden},
		{name: "origin takes precedence over referer", security: httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"}, origin: "https://evil.example", referer: "https://interview.example.com/settings", wantStatus: http.StatusForbidden},
		{name: "missing source", security: httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"}, wantStatus: http.StatusForbidden},
		{name: "development localhost", security: httpserver.SecurityConfig{Environment: "development", Domain: "localhost"}, origin: "http://localhost:5173", wantStatus: http.StatusNoContent},
		{name: "development loopback", security: httpserver.SecurityConfig{Environment: "development", Domain: "localhost"}, origin: "http://127.0.0.1:5173", wantStatus: http.StatusNoContent},
		{name: "development foreign host", security: httpserver.SecurityConfig{Environment: "development", Domain: "localhost"}, origin: "http://workstation.local:5173", wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := httpserver.NewHandler(readyCheckerFunc(func(context.Context) error { return nil }), discardLogger(), test.security, func(router chi.Router) {
				router.Post("/change", func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) })
			})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/change", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Referer", test.referer)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestSafeRequestsDoNotRequireOrigin(t *testing.T) {
	t.Parallel()
	handler := httpserver.NewHandler(readyCheckerFunc(func(context.Context) error { return nil }), discardLogger(), httpserver.SecurityConfig{Environment: "production", Domain: "interview.example.com"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", response.Code, http.StatusOK)
	}
}

func TestSecurityHeadersDependOnEnvironment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		env      string
		wantHSTS bool
	}{
		{name: "development", env: "development"},
		{name: "production", env: "production", wantHSTS: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := httpserver.NewHandler(readyCheckerFunc(func(context.Context) error { return nil }), discardLogger(), httpserver.SecurityConfig{Environment: test.env, Domain: "interview.example.com"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil))
			for name, want := range map[string]string{
				"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
				"Referrer-Policy":         "no-referrer",
			} {
				if got := response.Header().Get(name); got != want {
					t.Errorf("%s=%q, want %q", name, got, want)
				}
			}
			if got := response.Header().Get("Strict-Transport-Security"); (got != "") != test.wantHSTS {
				t.Errorf("Strict-Transport-Security=%q, want present=%v", got, test.wantHSTS)
			}
		})
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
