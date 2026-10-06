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

	httpserver.NewHandler(checker, discardLogger()).ServeHTTP(recorder, request)

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

	httpserver.NewHandler(checker, discardLogger()).ServeHTTP(recorder, request)

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

	httpserver.NewHandler(checker, discardLogger()).ServeHTTP(recorder, request)

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

	httpserver.NewHandler(checker, discardLogger()).ServeHTTP(recorder, request)

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

	httpserver.NewHandler(checker, logger).ServeHTTP(recorder, request)

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

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
