package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestLoginMeAndLogoutFlow(t *testing.T) {
	store := &memorySessions{user: User{ID: uuid.New(), Login: "fedor", PasswordHash: "encoded"}}
	handler := NewHandler(NewService(store, &recordingVerifier{match: true}, time.Hour), false)
	router := chi.NewRouter()
	handler.Mount(router)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"login":"fedor","password":"secret-password"}`)))
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("login cookies=%#v", cookies)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meRequest.AddCookie(cookies[0])
	me := httptest.NewRecorder()
	router.ServeHTTP(me, meRequest)
	if me.Code != http.StatusOK || !bytes.Contains(me.Body.Bytes(), []byte(`"login":"fedor"`)) {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutRequest.AddCookie(cookies[0])
	logout := httptest.NewRecorder()
	router.ServeHTTP(logout, logoutRequest)
	if logout.Code != http.StatusNoContent || store.revokedID == uuid.Nil {
		t.Fatalf("logout status=%d revoked=%s", logout.Code, store.revokedID)
	}
}

func TestLoginReturnsSameErrorForUnknownUserAndWrongPassword(t *testing.T) {
	requests := []struct {
		store *memorySessions
		body  string
	}{
		{store: &memorySessions{}, body: `{"login":"unknown","password":"secret-password"}`},
		{store: &memorySessions{user: User{ID: uuid.New(), Login: "fedor", PasswordHash: "encoded"}}, body: `{"login":"fedor","password":"wrong-password"}`},
	}
	var messages []string
	for _, item := range requests {
		handler := NewHandler(NewService(item.store, &recordingVerifier{}, time.Hour), false)
		router := chi.NewRouter()
		handler.Mount(router)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(item.body)))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		var envelope struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, envelope.Error.Code+":"+envelope.Error.Message)
	}
	if messages[0] != messages[1] {
		t.Fatalf("credential errors differ: %q vs %q", messages[0], messages[1])
	}
}

func TestLoginRateLimitIsBounded(t *testing.T) {
	handler := NewHandler(NewService(&memorySessions{}, &recordingVerifier{}, time.Hour), false)
	router := chi.NewRouter()
	handler.Mount(router)
	for attempt := 1; attempt <= 11; attempt++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"login":"unknown","password":"secret-password"}`)))
		if attempt == 11 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt 11 status=%d", response.Code)
		}
	}
	if handler.limiter.Size() > 1 {
		t.Fatalf("limiter size=%d, want <=1", handler.limiter.Size())
	}
}

func TestPasswordChangeRotatesCookie(t *testing.T) {
	store := &memorySessions{user: User{ID: uuid.New(), Login: "fedor", PasswordHash: "encoded"}}
	handler := NewHandler(NewService(store, &recordingVerifier{match: true}, time.Hour), false)
	router := chi.NewRouter()
	handler.Mount(router)
	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"login":"fedor","password":"current-password"}`)))
	oldCookie := login.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodPut, "/auth/password", bytes.NewBufferString(`{"current_password":"current-password","new_password":"new-password-long"}`))
	request.AddCookie(oldCookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	newCookies := response.Result().Cookies()
	if len(newCookies) != 1 || newCookies[0].Value == oldCookie.Value {
		t.Fatalf("cookie was not rotated: %#v", newCookies)
	}
}

func TestLogoutAllRevokesEverySession(t *testing.T) {
	store := &memorySessions{user: User{ID: uuid.New(), Login: "fedor", PasswordHash: "encoded"}}
	handler := NewHandler(NewService(store, &recordingVerifier{match: true}, time.Hour), false)
	router := chi.NewRouter()
	handler.Mount(router)
	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"login":"fedor","password":"current-password"}`)))
	request := httptest.NewRequest(http.MethodPost, "/auth/logout-all", nil)
	request.AddCookie(login.Result().Cookies()[0])
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !store.revokedAll {
		t.Fatalf("status=%d revokedAll=%v", response.Code, store.revokedAll)
	}
}
