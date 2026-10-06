package auth

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/requestcontext"
	"github.com/go-chi/chi/v5"
)

const sessionCookieName = "track_session"

type Handler struct {
	service      *Service
	secureCookie bool
	limiter      *RateLimiter
}

func NewHandler(service *Service, secureCookie bool) *Handler {
	return &Handler{service: service, secureCookie: secureCookie, limiter: NewRateLimiter(10, 15*time.Minute, 10_000)}
}

func (handler *Handler) Mount(router chi.Router) {
	router.Post("/auth/login", handler.login)
	router.Post("/auth/logout", handler.logout)
	router.With(handler.Require).Post("/auth/logout-all", handler.logoutAll)
	router.With(handler.Require).Put("/auth/password", handler.changePassword)
	router.With(handler.Require).Get("/auth/me", handler.me)
}

func (handler *Handler) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		userSession, ok := handler.authenticateRequest(writer, request)
		if !ok {
			return
		}
		ctx := withUserSession(request.Context(), userSession)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (handler *Handler) login(writer http.ResponseWriter, request *http.Request) {
	if !handler.limiter.Allow(clientIP(request), time.Now()) {
		writer.Header().Set("Retry-After", "900")
		writeAuthError(writer, request, http.StatusTooManyRequests, "rate_limited", "Слишком много попыток. Попробуйте позже")
		return
	}
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decodeAuthJSON(writer, request, &input) || !loginPattern.MatchString(input.Login) || len(input.Password) < 12 || len(input.Password) > 1024 {
		writeAuthError(writer, request, http.StatusBadRequest, "validation_error", "Проверьте введённые данные")
		return
	}
	result, err := handler.service.Login(request.Context(), input.Login, input.Password, time.Now())
	if err != nil {
		if err == ErrInvalidCredentials {
			writeAuthError(writer, request, http.StatusUnauthorized, "invalid_credentials", "Неверный логин или пароль")
			return
		}
		writeAuthError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось выполнить вход")
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: result.Token, Path: "/", HttpOnly: true, Secure: handler.secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: int(handler.service.ttl.Seconds())})
	writeAuthJSON(writer, http.StatusOK, map[string]any{"data": map[string]any{"id": result.User.ID.String(), "login": result.User.Login}})
}

func (handler *Handler) logout(writer http.ResponseWriter, request *http.Request) {
	if cookie, err := request.Cookie(sessionCookieName); err == nil {
		_ = handler.service.Logout(request.Context(), cookie.Value, time.Now())
	}
	handler.clearSessionCookie(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) logoutAll(writer http.ResponseWriter, request *http.Request) {
	cookie, _ := request.Cookie(sessionCookieName)
	if err := handler.service.LogoutAll(request.Context(), cookie.Value, time.Now()); err != nil {
		writeAuthError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось завершить сеансы")
		return
	}
	handler.clearSessionCookie(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) changePassword(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeAuthJSON(writer, request, &input) || len(input.CurrentPassword) < 12 || len(input.CurrentPassword) > 1024 || len(input.NewPassword) < 12 || len(input.NewPassword) > 1024 {
		writeAuthError(writer, request, http.StatusBadRequest, "validation_error", "Проверьте введённые данные")
		return
	}
	cookie, _ := request.Cookie(sessionCookieName)
	result, err := handler.service.ChangePassword(request.Context(), cookie.Value, input.CurrentPassword, input.NewPassword, time.Now())
	if err != nil {
		if err == ErrInvalidCredentials {
			writeAuthError(writer, request, http.StatusUnauthorized, "invalid_credentials", "Неверный текущий пароль")
			return
		}
		writeAuthError(writer, request, http.StatusInternalServerError, "internal_error", "Не удалось изменить пароль")
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: result.Token, Path: "/", HttpOnly: true, Secure: handler.secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: int(handler.service.ttl.Seconds())})
	writer.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) clearSessionCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: handler.secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func (handler *Handler) me(writer http.ResponseWriter, request *http.Request) {
	userSession, _ := userSessionFromContext(request.Context())
	writeAuthJSON(writer, http.StatusOK, map[string]any{"data": map[string]any{"id": userSession.User.ID.String(), "login": userSession.User.Login}})
}

func (handler *Handler) authenticateRequest(writer http.ResponseWriter, request *http.Request) (UserSession, bool) {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		writeAuthError(writer, request, http.StatusUnauthorized, "unauthorized", "Требуется вход")
		return UserSession{}, false
	}
	userSession, err := handler.service.Authenticate(request.Context(), cookie.Value, time.Now())
	if err != nil {
		writeAuthError(writer, request, http.StatusUnauthorized, "unauthorized", "Требуется вход")
		return UserSession{}, false
	}
	return userSession, true
}

func decodeAuthJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 4096)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(request.RemoteAddr)
}

func writeAuthError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	writeAuthJSON(writer, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": requestcontext.RequestID(request.Context())}})
}
func writeAuthJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
