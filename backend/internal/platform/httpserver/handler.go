package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

const requestIDHeader = "X-Request-ID"

type ReadyChecker interface {
	Ping(context.Context) error
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func NewHandler(checker ReadyChecker, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()
	router.Use(requestIDMiddleware)
	router.Use(requestLogMiddleware(logger))
	router.Get("/api/v1/health/live", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/api/v1/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()

		if err := checker.Ping(ctx); err != nil {
			writeError(writer, request, http.StatusServiceUnavailable, "service_unavailable", "Сервис временно недоступен")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	return router
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func requestLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			wrapped := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(wrapped, request)

			requestID, _ := request.Context().Value(requestIDContextKey{}).(string)
			logger.InfoContext(request.Context(), "http request",
				"request_id", requestID,
				"method", request.Method,
				"path", chi.RouteContext(request.Context()).RoutePattern(),
				"status", wrapped.status,
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		writer.Header().Set(requestIDHeader, requestID)
		ctx := context.WithValue(request.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

type requestIDContextKey struct{}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic("generate request ID: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

func writeError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	requestID, _ := request.Context().Value(requestIDContextKey{}).(string)
	writeJSON(writer, status, errorResponse{Error: errorBody{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
