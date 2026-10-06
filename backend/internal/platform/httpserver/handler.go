package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/requestcontext"
	"github.com/go-chi/chi/v5"
)

const requestIDHeader = "X-Request-ID"

const maxRequestBodyBytes = 1 << 20

type SecurityConfig struct {
	Environment string
	Domain      string
}

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

type RouteMounter func(chi.Router)

func NewHandler(checker ReadyChecker, logger *slog.Logger, security SecurityConfig, mounters ...RouteMounter) http.Handler {
	router := chi.NewRouter()
	router.Use(requestIDMiddleware)
	router.Use(securityHeadersMiddleware(security))
	router.Use(requestLogMiddleware(logger))
	router.Use(requestBoundaryMiddleware(security))
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
	for _, mount := range mounters {
		router.Route("/api/v1", mount)
	}
	return router
}

func securityHeadersMiddleware(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
			writer.Header().Set("X-Content-Type-Options", "nosniff")
			writer.Header().Set("X-Frame-Options", "DENY")
			writer.Header().Set("Referrer-Policy", "no-referrer")
			if config.Environment == "production" {
				writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func requestBoundaryMiddleware(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
			if isSafeMethod(request.Method) {
				next.ServeHTTP(writer, request)
				return
			}
			source := request.Header.Get("Origin")
			isReferer := false
			if source == "" {
				source = request.Header.Get("Referer")
				isReferer = true
			}
			if !allowedRequestSource(source, isReferer, config) {
				writeError(writer, request, http.StatusForbidden, "forbidden_origin", "Источник запроса не разрешён")
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func allowedRequestSource(source string, referer bool, config SecurityConfig) bool {
	parsed, err := url.Parse(source)
	if err != nil || parsed.User != nil || parsed.Opaque != "" || parsed.Scheme == "" || parsed.Host == "" || parsed.Fragment != "" {
		return false
	}
	if !referer && ((parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "") {
		return false
	}
	if config.Environment == "production" {
		return parsed.Scheme == "https" && strings.EqualFold(parsed.Host, config.Domain)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	hostname := parsed.Hostname()
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
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

			requestID := requestcontext.RequestID(request.Context())
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
		ctx := requestcontext.WithRequestID(request.Context(), requestID)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic("generate request ID: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

func writeError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	requestID := requestcontext.RequestID(request.Context())
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
